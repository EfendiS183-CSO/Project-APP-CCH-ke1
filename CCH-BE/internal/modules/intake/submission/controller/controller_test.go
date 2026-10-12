package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/intake/session/controller"
	"cch.local/backend/internal/modules/intake/submission"
)

type submitterStub struct {
	result  submission.Result
	err     error
	calls   int
	token   string
	request submission.Request
}

func (submitter *submitterStub) Submit(_ context.Context, token string, request submission.Request) (submission.Result, error) {
	submitter.calls++
	submitter.token = token
	submitter.request = request
	return submitter.result, submitter.err
}

func TestSubmissionControllerCreatesTicketWithoutCachingResponse(t *testing.T) {
	service := &submitterStub{result: submission.Result{Status: "created", TicketID: "TICKET-001"}}
	controller := newTestController(t, service, 2)
	app := fiber.New()
	controller.Register(app)

	response := sendSubmission(t, app, "https://chat.example.test", "signed-intake-token", validJSONBody())
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated || service.calls != 1 {
		t.Fatalf("status=%d service calls=%d", response.StatusCode, service.calls)
	}
	if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Access-Control-Allow-Origin") != "https://chat.example.test" {
		t.Fatalf("unexpected response security headers: %#v", response.Header)
	}
	if service.token != "signed-intake-token" || !service.request.Consent || service.request.Payload.CustomerName != "CUSTOMER" {
		t.Fatalf("service received unexpected submission: token=%q request=%#v", service.token, service.request)
	}
	var body submitResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "created" || body.TicketID != "TICKET-001" {
		t.Fatalf("unexpected response body: %#v", body)
	}
}

func TestSubmissionControllerSendsPendingReviewAsAccepted(t *testing.T) {
	service := &submitterStub{result: submission.Result{Status: "pending_cs", Reason: "profile_unmatched"}}
	controller := newTestController(t, service, 2)
	app := fiber.New()
	controller.Register(app)

	response := sendSubmission(t, app, "https://chat.example.test", "signed-intake-token", validJSONBody())
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusAccepted)
	}
	var body submitResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "pending_cs" || body.Reason != "profile_unmatched" || body.TicketID != "" {
		t.Fatalf("unexpected review response: %#v", body)
	}
}

func TestSubmissionControllerRejectsOriginAndMissingTokenBeforeService(t *testing.T) {
	service := &submitterStub{}
	controller := newTestController(t, service, 2)
	app := fiber.New()
	controller.Register(app)
	for _, test := range []struct {
		origin string
		token  string
		want   int
	}{
		{origin: "https://untrusted.example.test", token: "signed-intake-token", want: http.StatusForbidden},
		{origin: "https://chat.example.test", token: "", want: http.StatusUnauthorized},
	} {
		response := sendSubmission(t, app, test.origin, test.token, validJSONBody())
		response.Body.Close()
		if response.StatusCode != test.want {
			t.Errorf("origin=%q token=%q status=%d, want %d", test.origin, test.token, response.StatusCode, test.want)
		}
	}
	if service.calls != 0 {
		t.Fatalf("service called %d times for rejected requests", service.calls)
	}
}

func TestSubmissionControllerRejectsUnknownFieldsInvalidContentAndLargeBody(t *testing.T) {
	service := &submitterStub{}
	controller := newTestController(t, service, 5)
	app := fiber.New()
	controller.Register(app)
	for _, test := range []struct {
		name        string
		contentType string
		body        string
		want        int
	}{
		{name: "unknown field", contentType: "application/json", body: `{"consent":true,"extra":"value"}`, want: http.StatusBadRequest},
		{name: "wrong media type", contentType: "text/plain", body: validJSONBody(), want: http.StatusUnsupportedMediaType},
		{name: "large body", contentType: "application/json", body: strings.Repeat("x", maxRequestBodyBytes+1), want: http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/intake/submissions", strings.NewReader(test.body))
			request.Header.Set("Origin", "https://chat.example.test")
			request.Header.Set("X-Intake-Session", "signed-intake-token")
			request.Header.Set("Content-Type", test.contentType)
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != test.want {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.want)
			}
		})
	}
	if service.calls != 0 {
		t.Fatalf("service called %d times for invalid requests", service.calls)
	}
}

func TestSubmissionControllerMapsServiceErrorsWithoutLeakingDetails(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid session", err: submission.ErrInvalidSession, want: http.StatusUnauthorized},
		{name: "invalid input", err: submission.ErrInvalidSubmission, want: http.StatusBadRequest},
		{name: "processing", err: submission.ErrSubmissionProcessing, want: http.StatusServiceUnavailable},
		{name: "internal", err: errors.New("private database detail"), want: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &submitterStub{err: test.err}
			controller := newTestController(t, service, 1)
			app := fiber.New()
			controller.Register(app)
			response := sendSubmission(t, app, "https://chat.example.test", "signed-intake-token", validJSONBody())
			defer response.Body.Close()
			if response.StatusCode != test.want {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.want)
			}
			body := make([]byte, 2048)
			n, _ := response.Body.Read(body)
			if strings.Contains(string(body[:n]), "private database detail") {
				t.Fatal("internal error details leaked")
			}
		})
	}
}

func TestSubmissionControllerPreflightRequiresAllowedOrigin(t *testing.T) {
	service := &submitterStub{}
	controller := newTestController(t, service, 1)
	app := fiber.New()
	controller.Register(app)
	request := httptest.NewRequest(http.MethodOptions, "/intake/submissions", nil)
	request.Header.Set("Origin", "https://chat.example.test")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || response.Header.Get("Access-Control-Allow-Headers") == "" {
		t.Fatalf("preflight status=%d headers=%#v", response.StatusCode, response.Header)
	}
}

func newTestController(t *testing.T, service Submitter, limit int) *Controller {
	t.Helper()
	limiter, err := controller.NewRateLimiter(limit, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	controller, err := New(service, []string{"https://chat.example.test"}, limiter)
	if err != nil {
		t.Fatal(err)
	}
	return controller
}

func sendSubmission(t *testing.T, app *fiber.App, origin string, token string, body string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/intake/submissions", strings.NewReader(body))
	request.Header.Set("Origin", origin)
	request.Header.Set("X-Intake-Session", token)
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("submit intake: %v", err)
	}
	return response
}

func validJSONBody() string {
	return `{"consent":true,"summary":"Summary","payload":{"Nama_Pelanggan":"CUSTOMER","Nomor_HP_Pelanggan":"0812345678","Alamat_Pelanggan":"ADDRESS","Jenis_Pengaduan":1,"Jenis_Kiriman":"REG","Nomor_Kiriman":"WAYBILL-1","Kantor_Asal_Kiriman":"40005CC","Kantor_Tujuan_Kiriman":"40006CC","Uraian_Pengaduan":"Complaint"}}`
}

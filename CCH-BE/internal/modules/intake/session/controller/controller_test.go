package controller

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/intake/session"
)

type issuerStub struct {
	issued session.IssuedConversation
	err    error
	calls  int
}

func (issuer *issuerStub) Issue() (session.IssuedConversation, error) {
	issuer.calls++
	return issuer.issued, issuer.err
}

func TestSessionControllerIssuesConversationTokenOnlyForAllowedOrigin(t *testing.T) {
	expiresAt := time.Now().UTC().Add(20 * time.Minute).Truncate(time.Second)
	issuer := &issuerStub{issued: session.IssuedConversation{
		ConversationID: "0123456789abcdef0123456789abcdef",
		Token:          "signed-opaque-token",
		ExpiresAt:      expiresAt,
	}}
	controller := newTestController(t, issuer, 2)
	app := fiber.New()
	controller.Register(app)

	request := httptest.NewRequest(http.MethodPost, "/intake/sessions", nil)
	request.Header.Set("Origin", "https://chat.example.test")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("issue intake session: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated || issuer.calls != 1 {
		t.Fatalf("status=%d issuer calls=%d", response.StatusCode, issuer.calls)
	}
	if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Pragma") != "no-cache" || response.Header.Get("Access-Control-Allow-Origin") != "https://chat.example.test" {
		t.Fatalf("session response cache headers = %q/%q", response.Header.Get("Cache-Control"), response.Header.Get("Pragma"))
	}
	var body issuedResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode issue response: %v", err)
	}
	if body.ConversationID != issuer.issued.ConversationID || body.Token != issuer.issued.Token || !body.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected issue response: %#v", body)
	}
}

func TestSessionControllerRejectsMissingOrUnlistedOrigin(t *testing.T) {
	issuer := &issuerStub{}
	controller := newTestController(t, issuer, 2)
	app := fiber.New()
	controller.Register(app)
	for _, origin := range []string{"", "https://evil.example.test"} {
		request := httptest.NewRequest(http.MethodPost, "/intake/sessions", nil)
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response, err := app.Test(request)
		if err != nil {
			t.Fatalf("request with origin %q: %v", origin, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			t.Errorf("origin %q status = %d, want %d", origin, response.StatusCode, http.StatusForbidden)
		}
	}
	if issuer.calls != 0 {
		t.Fatalf("issuer called %d times for disallowed origins", issuer.calls)
	}
}

func TestSessionControllerAllowsPreflightOnlyForAllowedOrigin(t *testing.T) {
	controller := newTestController(t, &issuerStub{}, 1)
	app := fiber.New()
	controller.Register(app)
	request := httptest.NewRequest(http.MethodOptions, "/intake/sessions", nil)
	request.Header.Set("Origin", "https://chat.example.test")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || response.Header.Get("Access-Control-Allow-Origin") != "https://chat.example.test" {
		t.Fatalf("preflight status=%d headers=%#v", response.StatusCode, response.Header)
	}
}

func TestSessionControllerRateLimitsTokenIssuanceByClientIP(t *testing.T) {
	issuer := &issuerStub{issued: session.IssuedConversation{ConversationID: "conversation", Token: "token"}}
	controller := newTestController(t, issuer, 1)
	app := fiber.New()
	controller.Register(app)
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/intake/sessions", nil)
		request.Header.Set("Origin", "https://chat.example.test")
		response, err := app.Test(request)
		if err != nil {
			t.Fatalf("issue attempt %d: %v", attempt, err)
		}
		response.Body.Close()
		wantStatus := http.StatusCreated
		if attempt == 1 {
			wantStatus = http.StatusTooManyRequests
		}
		if response.StatusCode != wantStatus {
			t.Errorf("attempt %d status = %d, want %d", attempt, response.StatusCode, wantStatus)
		}
	}
	if issuer.calls != 1 {
		t.Fatalf("issuer called %d times, want 1", issuer.calls)
	}
}

func TestSessionControllerMapsIssuerFailureToGenericUnavailable(t *testing.T) {
	issuer := &issuerStub{err: errors.New("secret signing failure")}
	controller := newTestController(t, issuer, 2)
	app := fiber.New()
	controller.Register(app)
	request := httptest.NewRequest(http.MethodPost, "/intake/sessions", nil)
	request.Header.Set("Origin", "https://chat.example.test")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusServiceUnavailable)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read error response: %v", err)
	}
	if strings.Contains(string(body), "secret signing failure") {
		t.Fatal("issuer error details leaked to client")
	}
}

func TestNewRejectsInvalidOriginPolicyAndMissingDependencies(t *testing.T) {
	issuer := &issuerStub{}
	limiter, err := NewRateLimiter(1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, origins := range [][]string{nil, {"http://public.example.test"}, {"https://trusted.example.test/path"}} {
		if _, err := New(issuer, origins, limiter); !errors.Is(err, ErrInvalidOrigin) {
			t.Errorf("New(..., %v, ...) error = %v, want invalid origin", origins, err)
		}
	}
	if _, err := New(nil, []string{"https://trusted.example.test"}, limiter); !errors.Is(err, ErrIssuerNotConfigured) {
		t.Fatalf("New(nil, ...) error = %v, want issuer not configured", err)
	}
	if _, err := New(issuer, []string{"https://trusted.example.test"}, nil); !errors.Is(err, ErrInvalidRateLimit) {
		t.Fatalf("New(..., nil limiter) error = %v, want invalid limiter", err)
	}
}

func newTestController(t *testing.T, issuer Issuer, limit int) *Controller {
	t.Helper()
	limiter, err := NewRateLimiter(limit, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	controller, err := New(issuer, []string{"https://chat.example.test"}, limiter)
	if err != nil {
		t.Fatal(err)
	}
	return controller
}

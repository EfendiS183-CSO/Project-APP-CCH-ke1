package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cch.local/backend/internal/modules/complaints"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
	"github.com/gofiber/fiber/v3"
)

type stubStatusTransitionApplier struct {
	identity authentication.VerifiedIdentity
	request  complaints.StatusTransitionRequest
	result   complaints.StatusTransitionResult
	err      error
	calls    int
}

func (service *stubStatusTransitionApplier) ApplyForCallCenter(_ context.Context, identity authentication.VerifiedIdentity, request complaints.StatusTransitionRequest) (complaints.StatusTransitionResult, error) {
	service.calls++
	service.identity = identity
	service.request = request
	return service.result, service.err
}

func TestStatusTransitionControllerCreatesAndReplaysIdempotentRequest(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test"}
	for _, test := range []struct {
		name       string
		result     complaints.StatusTransitionResult
		wantStatus int
	}{
		{
			name: "first application",
			result: complaints.StatusTransitionResult{
				ComplaintID: "TKT-001", HistoryID: "2026100110203045ABCD2345", StatusCode: "103", UpdatedAt: "2026-10-01T10:20:30Z",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "exact replay",
			result: complaints.StatusTransitionResult{
				ComplaintID: "TKT-001", HistoryID: "2026100110203045ABCD2345", StatusCode: "103", UpdatedAt: "2026-10-01T10:20:30Z", Replayed: true,
			},
			wantStatus: http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &stubStatusTransitionApplier{result: test.result}
			app := statusTransitionTestApp(identity, service)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/status", strings.NewReader(`{"expected_status_code":"102","status_code":"103","description":"Sedang dikonfirmasi"}`))
			request.Header.Set("X-Authorization", "Bearer valid-token")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "request-key-0001")
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("request status transition: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Pragma") != "no-cache" {
				t.Fatalf("status/header = %d/%q/%q", response.StatusCode, response.Header.Get("Cache-Control"), response.Header.Get("Pragma"))
			}
			if service.calls != 1 || service.identity != identity {
				t.Fatalf("service call identity/count = %#v/%d", service.identity, service.calls)
			}
			if service.request.ComplaintID != "TKT-001" || service.request.ExpectedStatusCode != "102" || service.request.StatusCode != "103" || service.request.Description != "Sedang dikonfirmasi" || service.request.IdempotencyKey != "request-key-0001" {
				t.Fatalf("service request = %#v", service.request)
			}
			var body statusTransitionResponse
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.HistoryID != test.result.HistoryID || body.Replayed != test.result.Replayed {
				t.Fatalf("response body = %#v", body)
			}
		})
	}
}

func TestStatusTransitionControllerRejectsUnknownFieldsBeforeService(t *testing.T) {
	service := &stubStatusTransitionApplier{}
	app := statusTransitionTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/status", strings.NewReader(`{"expected_status_code":"102","status_code":"103","description":"update","office_code":"ATTACKER"}`))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request-key-0001")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.calls != 0 {
		t.Fatalf("status/calls = %d/%d, want bad request without service call", response.StatusCode, service.calls)
	}
}

func TestStatusTransitionControllerMapsDomainErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{name: "stale status", serviceErr: complaints.ErrStatusTransitionConflict, wantStatus: http.StatusConflict},
		{name: "reused key", serviceErr: complaints.ErrIdempotencyConflict, wantStatus: http.StatusConflict},
		{name: "out of scope", serviceErr: complaints.ErrStatusTransitionNotAvailable, wantStatus: http.StatusNotFound},
		{name: "invalid command", serviceErr: complaints.ErrInvalidStatusTransition, wantStatus: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &stubStatusTransitionApplier{err: test.serviceErr}
			app := statusTransitionTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/status", strings.NewReader(`{"expected_status_code":"102","status_code":"103","description":"update"}`))
			request.Header.Set("X-Authorization", "Bearer valid-token")
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", "request-key-0001")
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}

func statusTransitionTestApp(identity authentication.VerifiedIdentity, service *stubStatusTransitionApplier) *fiber.App {
	registrar := httpapi.AuthenticatedRouteModule(stubTokenVerifier{identity: identity}, NewStatusTransitionController(service))
	return httpapi.NewServer(nil, registrar)
}

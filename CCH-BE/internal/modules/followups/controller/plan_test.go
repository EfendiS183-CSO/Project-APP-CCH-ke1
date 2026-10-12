package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/followups"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type stubCallCenterPlanCreator struct {
	identity authentication.VerifiedIdentity
	request  followups.CreatePlanRequest
	entry    followups.Entry
	err      error
	calls    int
}

func (service *stubCallCenterPlanCreator) Create(_ context.Context, identity authentication.VerifiedIdentity, request followups.CreatePlanRequest) (followups.Entry, error) {
	service.calls++
	service.identity = identity
	service.request = request
	return service.entry, service.err
}

func TestPlanControllerCreatesPlanForVerifiedIdentity(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "UNTRUSTED"}
	plannedAt := time.Date(2026, time.October, 2, 10, 30, 0, 0, time.UTC)
	service := &stubCallCenterPlanCreator{entry: followups.Entry{
		ComplaintID: "TKT-001", OfficeCode: "40005CC", StatusCode: "FU1", PlannedAt: &plannedAt, PlanNotes: "Hubungi pelanggan",
	}}
	app := newPlanTestApp(identity, service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/follow-ups", strings.NewReader(`{"status_code":"FU1","planned_at":"2026-10-02T10:30:00Z","plan_notes":"Hubungi pelanggan"}`))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Pragma") != "no-cache" {
		t.Fatalf("status/cache headers = %d/%q/%q", response.StatusCode, response.Header.Get("Cache-Control"), response.Header.Get("Pragma"))
	}
	if service.calls != 1 || service.identity != identity || service.request.ComplaintID != "TKT-001" || service.request.StatusCode != "FU1" || service.request.PlanNotes != "Hubungi pelanggan" || !service.request.PlannedAt.Equal(plannedAt) {
		t.Fatalf("service call = %#v", service)
	}
	var body followUpItem
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.ComplaintID != "TKT-001" || body.OfficeCode != "40005CC" || body.StatusCode != "FU1" {
		t.Fatalf("response = %#v", body)
	}
}

func TestPlanControllerRejectsUnknownFieldsBeforeService(t *testing.T) {
	service := &stubCallCenterPlanCreator{}
	app := newPlanTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/follow-ups", strings.NewReader(`{"status_code":"FU1","planned_at":"2026-10-02T10:30:00Z","office_code":"ATTACKER"}`))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.calls != 0 {
		t.Fatalf("status/calls = %d/%d", response.StatusCode, service.calls)
	}
}

func TestPlanControllerMapsDuplicateAndScopeErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{name: "duplicate plan", serviceErr: followups.ErrPlanAlreadyExists, wantStatus: http.StatusConflict},
		{name: "parent not visible", serviceErr: followups.ErrPlanNotAvailable, wantStatus: http.StatusNotFound},
		{name: "invalid plan", serviceErr: followups.ErrInvalidPlanRequest, wantStatus: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &stubCallCenterPlanCreator{err: test.serviceErr}
			app := newPlanTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/follow-ups", strings.NewReader(`{"status_code":"FU1","planned_at":"2026-10-02T10:30:00Z"}`))
			request.Header.Set("X-Authorization", "Bearer valid-token")
			request.Header.Set("Content-Type", "application/json")
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

func newPlanTestApp(identity authentication.VerifiedIdentity, service *stubCallCenterPlanCreator) *fiber.App {
	registrar := httpapi.AuthenticatedRouteModule(stubVerifier{identity: identity}, NewPlanController(service))
	return httpapi.NewServer(nil, registrar)
}

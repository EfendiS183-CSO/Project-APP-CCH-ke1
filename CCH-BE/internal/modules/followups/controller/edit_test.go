package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cch.local/backend/internal/modules/followups"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type editServiceStub struct {
	request followups.EditPlanRequest
	entry   followups.Entry
	err     error
}

func (service *editServiceStub) Edit(_ context.Context, _ authentication.VerifiedIdentity, request followups.EditPlanRequest) (followups.Entry, error) {
	service.request = request
	return service.entry, service.err
}

func TestEditControllerAcceptsStrictPlanPayload(t *testing.T) {
	plannedAt := time.Date(2026, time.October, 3, 12, 0, 0, 0, time.UTC)
	service := &editServiceStub{entry: followups.Entry{ComplaintID: "TKT-001", StatusCode: "FU1", PlannedAt: &plannedAt, PlanNotes: "Updated"}}
	app := httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(stubVerifier{identity: authentication.VerifiedIdentity{Username: "agent.test"}}, NewEditController(service)))
	request := httptest.NewRequest(http.MethodPut, "/api/v1/complaints/call-center/TKT-001/follow-ups/FU1", strings.NewReader(`{"planned_at":"2026-10-03T12:00:00Z","plan_notes":"Updated"}`))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || service.request.ComplaintID != "TKT-001" || service.request.StatusCode != "FU1" || service.request.PlanNotes != "Updated" {
		t.Fatalf("status/request = %d/%#v", response.StatusCode, service.request)
	}
}

func TestEditControllerRejectsUnknownFields(t *testing.T) {
	service := &editServiceStub{}
	app := httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(stubVerifier{identity: authentication.VerifiedIdentity{Username: "agent.test"}}, NewEditController(service)))
	request := httptest.NewRequest(http.MethodPut, "/api/v1/complaints/call-center/TKT-001/follow-ups/FU1", strings.NewReader(`{"planned_at":"2026-10-03T12:00:00Z","office_code":"ATTACKER"}`))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want bad request", response.StatusCode)
	}
}

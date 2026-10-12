package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/followups"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type stubCallCenterRealizationService struct {
	identity authentication.VerifiedIdentity
	request  followups.RealizeRequest
	result   followups.RealizeResult
	err      error
	calls    int
}

func (service *stubCallCenterRealizationService) Realize(_ context.Context, identity authentication.VerifiedIdentity, request followups.RealizeRequest) (followups.RealizeResult, error) {
	service.calls++
	service.identity = identity
	service.request = request
	return service.result, service.err
}

func TestRealizationControllerMapsPathAndReturnsCreatedOrReplay(t *testing.T) {
	for _, test := range []struct {
		name       string
		result     followups.RealizeResult
		wantStatus int
	}{
		{
			name: "new realization",
			result: followups.RealizeResult{Entry: followups.Entry{
				ComplaintID: "TKT-001", OfficeCode: "40005CC", StatusCode: "FU1", Outcome: "Pelanggan dihubungi",
			}},
			wantStatus: http.StatusCreated,
		},
		{
			name: "exact replay",
			result: followups.RealizeResult{Entry: followups.Entry{
				ComplaintID: "TKT-001", OfficeCode: "40005CC", StatusCode: "FU1", Outcome: "Pelanggan dihubungi",
			}, Replayed: true},
			wantStatus: http.StatusOK,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}
			service := &stubCallCenterRealizationService{result: test.result}
			app := realizationTestApp(identity, service)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/follow-ups/FU1/realize", strings.NewReader(`{"outcome":"Pelanggan dihubungi"}`))
			request.Header.Set("X-Authorization", "Bearer valid-token")
			request.Header.Set("Content-Type", "application/json")
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus || response.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("status/cache = %d/%q", response.StatusCode, response.Header.Get("Cache-Control"))
			}
			if service.calls != 1 || service.identity != identity || service.request.ComplaintID != "TKT-001" || service.request.StatusCode != "FU1" || service.request.Outcome != "Pelanggan dihubungi" {
				t.Fatalf("service call = %#v", service)
			}
		})
	}
}

func TestRealizationControllerRejectsUnknownFieldsBeforeService(t *testing.T) {
	service := &stubCallCenterRealizationService{}
	app := realizationTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/follow-ups/FU1/realize", strings.NewReader(`{"outcome":"done","office_code":"ATTACKER"}`))
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

func TestRealizationControllerMapsConflictAndNotFound(t *testing.T) {
	for _, test := range []struct {
		serviceErr error
		wantStatus int
	}{
		{serviceErr: followups.ErrRealizationConflict, wantStatus: http.StatusConflict},
		{serviceErr: followups.ErrRealizationNotFound, wantStatus: http.StatusNotFound},
		{serviceErr: followups.ErrInvalidRealization, wantStatus: http.StatusBadRequest},
	} {
		service := &stubCallCenterRealizationService{err: test.serviceErr}
		app := realizationTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
		request := httptest.NewRequest(http.MethodPost, "/api/v1/complaints/call-center/TKT-001/follow-ups/FU1/realize", strings.NewReader(`{"outcome":"done"}`))
		request.Header.Set("X-Authorization", "Bearer valid-token")
		request.Header.Set("Content-Type", "application/json")
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.wantStatus {
			t.Errorf("error %v status = %d, want %d", test.serviceErr, response.StatusCode, test.wantStatus)
		}
	}
}

func realizationTestApp(identity authentication.VerifiedIdentity, service *stubCallCenterRealizationService) *fiber.App {
	registrar := httpapi.AuthenticatedRouteModule(stubVerifier{identity: identity}, NewRealizationController(service))
	return httpapi.NewServer(nil, registrar)
}

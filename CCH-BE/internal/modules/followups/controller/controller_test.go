package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/followups"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type stubFollowUpLister struct {
	identity         authentication.VerifiedIdentity
	query            followups.Query
	officeQuery      followups.OfficeQuery
	dueTodayQuery    followups.OfficeQuery
	entries          []followups.Entry
	err              error
	called           bool
	officeListCalled bool
	dueTodayCalled   bool
}

func (service *stubFollowUpLister) ListForCallCenterTicket(_ context.Context, identity authentication.VerifiedIdentity, query followups.Query) ([]followups.Entry, error) {
	service.called = true
	service.identity = identity
	service.query = query
	return service.entries, service.err
}

func (service *stubFollowUpLister) ListForCallCenterOffice(_ context.Context, identity authentication.VerifiedIdentity, query followups.OfficeQuery) ([]followups.Entry, error) {
	service.officeListCalled = true
	service.identity = identity
	service.officeQuery = query
	return service.entries, service.err
}

func (service *stubFollowUpLister) ListDueTodayForCallCenterOffice(_ context.Context, identity authentication.VerifiedIdentity, query followups.OfficeQuery) ([]followups.Entry, error) {
	service.dueTodayCalled = true
	service.identity = identity
	service.dueTodayQuery = query
	return service.entries, service.err
}

type stubVerifier struct {
	identity authentication.VerifiedIdentity
}

func (verifier stubVerifier) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return verifier.identity, nil
}

func TestControllerListsFollowUpsForTicketUsingVerifiedIdentity(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC"}
	plannedAt := time.Date(2026, time.September, 30, 13, 0, 0, 0, time.UTC)
	service := &stubFollowUpLister{entries: []followups.Entry{{
		ComplaintID: "TKT-001",
		OfficeCode:  "40006CC",
		StatusCode:  "102",
		PlannedAt:   &plannedAt,
		PlanNotes:   "Jadwalkan pemeriksaan",
	}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/follow-ups?limit=10&offset=5&office_code=ATTACKER", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request follow-ups: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if !service.called || service.identity != identity || service.query.ComplaintID != "TKT-001" || service.query.OfficeCode != "" {
		t.Fatalf("controller passed unexpected identity/query: %#v", service)
	}
	var body listResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode follow-up response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].OfficeCode != "40006CC" || body.Data[0].PlanNotes != "Jadwalkan pemeriksaan" || body.Pagination.Limit != 10 || body.Pagination.Offset != 5 {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestControllerListsFollowUpsForAuthenticatedOfficeAndIgnoresRequestedOffice(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC"}
	service := &stubFollowUpLister{entries: []followups.Entry{{
		ComplaintID: "TKT-001",
		OfficeCode:  "40005CC",
		StatusCode:  "102",
		PlanNotes:   "Hubungi pelanggan",
	}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/follow-ups/call-center/office?limit=10&offset=5&office_code=ATTACKER", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request office follow-ups: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if !service.officeListCalled || service.identity != identity || service.officeQuery.OfficeCode != "" || service.officeQuery.Limit != 10 || service.officeQuery.Offset != 5 {
		t.Fatalf("controller passed unexpected office query: %#v", service)
	}
	var body listResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode follow-up response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].OfficeCode != "40005CC" || body.Pagination.Limit != 10 || body.Pagination.Offset != 5 {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestControllerListsDueTodayFollowUpsForVerifiedIdentity(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "40005CC"}
	plannedAt := time.Date(2026, time.September, 30, 9, 0, 0, 0, time.UTC)
	service := &stubFollowUpLister{entries: []followups.Entry{{
		ComplaintID: "TKT-002",
		OfficeCode:  "40005CC",
		StatusCode:  "102",
		PlannedAt:   &plannedAt,
	}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/follow-ups/call-center/office/due-today?limit=10&offset=5&office_code=ATTACKER", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request due-today follow-ups: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !service.dueTodayCalled || service.identity != identity || service.dueTodayQuery.OfficeCode != "" || service.dueTodayQuery.Limit != 10 || service.dueTodayQuery.Offset != 5 {
		t.Fatalf("status=%d service=%#v", response.StatusCode, service)
	}
	var body listResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode due-today response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ComplaintID != "TKT-002" || body.Pagination.Limit != 10 || body.Pagination.Offset != 5 {
		t.Fatalf("unexpected due-today response: %#v", body)
	}
}

func TestControllerRequiresJWTAndValidTicketID(t *testing.T) {
	service := &stubFollowUpLister{}
	app := newTestApp(authentication.VerifiedIdentity{}, service)
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/follow-ups", nil))
	if err != nil {
		t.Fatalf("request without JWT: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.called {
		t.Fatalf("status = %d, called = %v; want 401 without service call", response.StatusCode, service.called)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/12345678901234567890123456/follow-ups", nil)
	request.Header.Set("X-Authorization", "valid-token")
	response, err = app.Test(request)
	if err != nil {
		t.Fatalf("request with invalid ticket ID: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.called {
		t.Fatalf("status = %d, called = %v; want 400 without service call", response.StatusCode, service.called)
	}
}

func TestControllerMapsHiddenParentAndBackendError(t *testing.T) {
	for _, test := range []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{name: "parent not visible", serviceErr: followups.ErrNotFoundOrNotVisible, wantStatus: http.StatusNotFound},
		{name: "database unavailable", serviceErr: errors.New("database password detail"), wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, &stubFollowUpLister{err: test.serviceErr})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/TKT-001/follow-ups", nil)
			request.Header.Set("X-Authorization", "valid-token")
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("request follow-ups: %v", err)
			}
			response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}

func newTestApp(identity authentication.VerifiedIdentity, service *stubFollowUpLister) *fiber.App {
	app := httpapi.NewServer(nil)
	app.Use(httpapi.JWTAuthentication(stubVerifier{identity: identity}))
	New(service).Register(app.Group("/api/v1"))
	return app
}

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/complaints"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type lookupGetterStub struct {
	identity authentication.VerifiedIdentity
	lookups  complaints.CallCenterComplaintLookups
	err      error
	called   bool
}

func (service *lookupGetterStub) Get(_ context.Context, identity authentication.VerifiedIdentity) (complaints.CallCenterComplaintLookups, error) {
	service.called = true
	service.identity = identity
	return service.lookups, service.err
}

type lookupVerifierStub struct {
	identity authentication.VerifiedIdentity
}

func (verifier lookupVerifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return verifier.identity, nil
}

func TestLookupControllerReturnsActiveComplaintReferencesForVerifiedIdentity(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test"}
	service := &lookupGetterStub{lookups: complaints.CallCenterComplaintLookups{
		ComplaintTypes:     []complaints.ComplaintTypeOption{{Code: 1, Description: "Paket"}},
		Sources:            []complaints.ComplaintSourceOption{{ID: 2, Name: "Live Chat"}},
		OriginOfficeCode:   "40005CC",
		DestinationOffices: []complaints.OfficeOption{{Code: "40006CC", Name: "Tujuan"}},
		CopyOffices:        []complaints.OfficeOption{{Code: "40007CC", Name: "Tembusan"}},
		ShippingServices:   []complaints.ShippingServiceOption{{Code: "REG", Description: "Reguler"}},
		ShippingOffices:    []complaints.OfficeOption{{Code: "40008CC", Name: "Kantor Kiriman"}},
	}}
	app := newLookupTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/lookups", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request complaint lookups: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !service.called || service.identity != identity {
		t.Fatalf("status=%d called=%v identity=%#v", response.StatusCode, service.called, service.identity)
	}
	var body struct {
		Data complaints.CallCenterComplaintLookups `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode complaint lookups: %v", err)
	}
	if len(body.Data.ComplaintTypes) != 1 || len(body.Data.Sources) != 1 || body.Data.OriginOfficeCode != "40005CC" || len(body.Data.DestinationOffices) != 1 || len(body.Data.CopyOffices) != 1 || len(body.Data.ShippingServices) != 1 || len(body.Data.ShippingOffices) != 1 {
		t.Fatalf("unexpected lookup response: %#v", body)
	}
}

func TestLookupControllerRequiresJWTAndMapsAccessDenial(t *testing.T) {
	service := &lookupGetterStub{err: authorization.ErrPermissionDenied}
	app := newLookupTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/lookups", nil))
	if err != nil {
		t.Fatalf("request without token: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.called {
		t.Fatalf("status=%d called=%v; want unauthorized without service call", response.StatusCode, service.called)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/lookups", nil)
	request.Header.Set("X-Authorization", "valid-token")
	response, err = app.Test(request)
	if err != nil {
		t.Fatalf("request denied lookups: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d, want forbidden", response.StatusCode)
	}
}

func TestLookupControllerHidesRepositoryErrors(t *testing.T) {
	service := &lookupGetterStub{err: errors.New("database details")}
	app := newLookupTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/lookups", nil)
	request.Header.Set("X-Authorization", "valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request lookup backend error: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want service unavailable", response.StatusCode)
	}
}

func newLookupTestApp(identity authentication.VerifiedIdentity, service *lookupGetterStub) *fiber.App {
	app := httpapi.NewServer(nil)
	app.Use(httpapi.JWTAuthentication(lookupVerifierStub{identity: identity}))
	NewLookupController(service).Register(app.Group("/api/v1"))
	return app
}

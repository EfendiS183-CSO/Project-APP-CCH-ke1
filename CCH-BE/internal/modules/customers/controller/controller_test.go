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
	"cch.local/backend/internal/modules/customers"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type searcherStub struct {
	identity authentication.VerifiedIdentity
	query    customers.SearchQuery
	profiles []customers.Profile
	profile  customers.Profile
	err      error
	called   bool
}

func (service *searcherStub) Search(_ context.Context, identity authentication.VerifiedIdentity, query customers.SearchQuery) ([]customers.Profile, error) {
	service.called = true
	service.identity = identity
	service.query = query
	return service.profiles, service.err
}

func (service *searcherStub) GetProfileForTicket(_ context.Context, _ authentication.VerifiedIdentity, _ int64) (customers.Profile, error) {
	return service.profile, service.err
}

type verifierStub struct {
	identity authentication.VerifiedIdentity
}

func (verifier verifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return verifier.identity, nil
}

func TestCustomerControllerReturnsBoundedProfilesForVerifiedIdentity(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "agent.test"}
	service := &searcherStub{profiles: []customers.Profile{{ID: 10, FullName: "CUSTOMER TEST", Phone: "0812345678"}}}
	app := newTestApp(identity, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/customers/search?q=customer&limit=5", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request customer search: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !service.called || service.identity != identity || service.query.Term != "customer" || service.query.Limit != 5 {
		t.Fatalf("status=%d service=%#v", response.StatusCode, service)
	}
	var body searchResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode customer search response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ID != 10 || body.Limit != 5 {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestCustomerControllerRequiresJWTAndMinimumQuery(t *testing.T) {
	service := &searcherStub{}
	app := newTestApp(authentication.VerifiedIdentity{}, service)
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/customers/search?q=customer", nil))
	if err != nil {
		t.Fatalf("request without JWT: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.called {
		t.Fatalf("status=%d called=%v; want unauthorized without service call", response.StatusCode, service.called)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/customers/search?q=ab", nil)
	request.Header.Set("X-Authorization", "valid-token")
	response, err = app.Test(request)
	if err != nil {
		t.Fatalf("request with short query: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest || service.called {
		t.Fatalf("status=%d called=%v; want bad request without service call", response.StatusCode, service.called)
	}
}

func TestCustomerControllerReturnsCanonicalProfileByID(t *testing.T) {
	profile := customers.Profile{ID: 42, FullName: "CUSTOMER TEST", Email: "customer@example.test"}
	service := &searcherStub{profile: profile}
	app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/customers/42", nil)
	request.Header.Set("X-Authorization", "valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("request customer profile: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("status=%d cache-control=%q", response.StatusCode, response.Header.Get("Cache-Control"))
	}
	var body profileResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode customer profile response: %v", err)
	}
	if body.Data != profile {
		t.Fatalf("profile = %#v, want %#v", body.Data, profile)
	}
}

func TestCustomerControllerHidesUnknownProfileAndRejectsInvalidID(t *testing.T) {
	for _, test := range []struct {
		name       string
		path       string
		serviceErr error
		wantStatus int
	}{
		{name: "unknown profile", path: "/api/v1/complaints/call-center/customers/42", serviceErr: customers.ErrProfileNotFound, wantStatus: http.StatusNotFound},
		{name: "invalid id", path: "/api/v1/complaints/call-center/customers/not-an-id", wantStatus: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &searcherStub{err: test.serviceErr}
			app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			request.Header.Set("X-Authorization", "valid-token")
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("request customer profile: %v", err)
			}
			response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status=%d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}

func TestCustomerControllerMapsPermissionDenialAndHidesRepositoryErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{name: "permission denied", serviceErr: authorization.ErrPermissionDenied, wantStatus: http.StatusForbidden},
		{name: "repository failure", serviceErr: errors.New("database secret detail"), wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &searcherStub{err: test.serviceErr}
			app := newTestApp(authentication.VerifiedIdentity{Username: "agent.test"}, service)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/complaints/call-center/customers/search?q=customer", nil)
			request.Header.Set("X-Authorization", "valid-token")
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("request customer search: %v", err)
			}
			response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status=%d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}

func newTestApp(identity authentication.VerifiedIdentity, service *searcherStub) *fiber.App {
	app := httpapi.NewServer(nil)
	app.Use(httpapi.JWTAuthentication(verifierStub{identity: identity}))
	New(service).Register(app.Group("/api/v1"))
	return app
}

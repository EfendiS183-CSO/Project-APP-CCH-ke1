package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/customers"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type profileManagerStub struct{ called string }

func (service *profileManagerStub) List(_ context.Context, _ authentication.VerifiedIdentity, query customers.ProfileListQuery) (customers.ProfilePage, error) {
	service.called = "list"
	return customers.ProfilePage{Data: []customers.Profile{}, Limit: query.Limit, Offset: query.Offset}, nil
}
func (service *profileManagerStub) Get(context.Context, authentication.VerifiedIdentity, int64) (customers.Profile, error) {
	service.called = "get"
	return customers.Profile{}, nil
}
func (service *profileManagerStub) Create(context.Context, authentication.VerifiedIdentity, customers.CreateProfileCommand) (customers.Profile, error) {
	service.called = "create"
	return customers.Profile{ID: 13}, nil
}
func (service *profileManagerStub) Update(context.Context, authentication.VerifiedIdentity, customers.UpdateProfileCommand) (customers.Profile, error) {
	service.called = "update"
	return customers.Profile{ID: 13}, nil
}
func (service *profileManagerStub) Delete(context.Context, authentication.VerifiedIdentity, int64) error {
	service.called = "delete"
	return nil
}

type profileTokenVerifierStub struct{}

func (profileTokenVerifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "customer.manager"}, nil
}
func newProfileTestApp(service ProfileManager) *fiber.App {
	return httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(profileTokenVerifierStub{}, NewProfileController(service)))
}
func authorizedProfileRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestProfileControllerRegistersCRUD(t *testing.T) {
	service := &profileManagerStub{}
	app := newProfileTestApp(service)
	for _, test := range []struct {
		request *http.Request
		status  int
		action  string
	}{
		{authorizedProfileRequest(http.MethodGet, "/api/v1/customers?limit=10&offset=5", ""), http.StatusOK, "list"},
		{authorizedProfileRequest(http.MethodGet, "/api/v1/customers/13", ""), http.StatusOK, "get"},
		{authorizedProfileRequest(http.MethodPost, "/api/v1/customers", `{"full_name":"Ada","email":"ada@example.test","phone":"+62123","address":"Street"}`), http.StatusCreated, "create"},
		{authorizedProfileRequest(http.MethodPut, "/api/v1/customers/13", `{"full_name":"Ada","email":"ada@example.test","phone":"+62123","address":"Street"}`), http.StatusOK, "update"},
		{authorizedProfileRequest(http.MethodDelete, "/api/v1/customers/13", ""), http.StatusNoContent, "delete"},
		{authorizedProfileRequest(http.MethodPost, "/api/v1/customers", `{"full_name":"Ada","extra":true}`), http.StatusBadRequest, "delete"},
	} {
		response, err := app.Test(test.request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.status {
			t.Errorf("%s %s status = %d, want %d", test.request.Method, test.request.URL.Path, response.StatusCode, test.status)
		}
		if test.status != http.StatusBadRequest && service.called != test.action {
			t.Errorf("action = %q, want %q", service.called, test.action)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Errorf("%s %s missing no-store", test.request.Method, test.request.URL.Path)
		}
	}
}

func TestProfileControllerRequiresAuthentication(t *testing.T) {
	service := &profileManagerStub{}
	response, err := newProfileTestApp(service).Test(httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.called != "" {
		t.Fatalf("unauthenticated status/call = %d/%q", response.StatusCode, service.called)
	}
}

package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/masters/offices"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct{ called string }

func (service *serviceStub) List(context.Context, authentication.VerifiedIdentity) ([]offices.Office, error) {
	service.called = "list"
	return nil, nil
}
func (service *serviceStub) Get(context.Context, authentication.VerifiedIdentity, string) (offices.Office, error) {
	service.called = "get"
	return offices.Office{}, nil
}
func (service *serviceStub) LookupOptions(context.Context, authentication.VerifiedIdentity) (offices.LookupOptions, error) {
	service.called = "lookup"
	return offices.LookupOptions{}, nil
}
func (service *serviceStub) Create(context.Context, authentication.VerifiedIdentity, offices.CreateCommand) (offices.Office, error) {
	service.called = "create"
	return offices.Office{}, nil
}
func (service *serviceStub) Update(context.Context, authentication.VerifiedIdentity, offices.UpdateCommand) (offices.Office, error) {
	service.called = "update"
	return offices.Office{}, nil
}
func (service *serviceStub) Deactivate(context.Context, authentication.VerifiedIdentity, string) error {
	service.called = "deactivate"
	return nil
}

type tokenVerifierStub struct{}

func (tokenVerifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "office.manager"}, nil
}
func newTestApp(service Service) *fiber.App {
	return httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(tokenVerifierStub{}, New(service)))
}
func authorizedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerRegistersOfficeCRUDAndLookupOptions(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, test := range []struct {
		request    *http.Request
		wantStatus int
		wantCall   string
	}{
		{authorizedRequest(http.MethodGet, "/api/v1/masters/offices", ""), http.StatusOK, "list"},
		{authorizedRequest(http.MethodGet, "/api/v1/masters/offices/40005CC", ""), http.StatusOK, "get"},
		{authorizedRequest(http.MethodGet, "/api/v1/masters/offices/options", ""), http.StatusOK, "lookup"},
		{authorizedRequest(http.MethodPost, "/api/v1/masters/offices", `{"code":"40006CC","name":"New Office","abbreviation":"NO","type":"UPT","regional_code":"40005"}`), http.StatusCreated, "create"},
		{authorizedRequest(http.MethodPut, "/api/v1/masters/offices/40005CC", `{"name":"Office","abbreviation":"OF","type":"UPT","regional_code":"40005","active":true,"as_destination":true,"as_copy":false}`), http.StatusOK, "update"},
		{authorizedRequest(http.MethodDelete, "/api/v1/masters/offices/40005CC", ""), http.StatusNoContent, "deactivate"},
	} {
		response, err := app.Test(test.request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.wantStatus {
			t.Errorf("%s %s status = %d, want %d", test.request.Method, test.request.URL.Path, response.StatusCode, test.wantStatus)
		}
		if service.called != test.wantCall {
			t.Errorf("%s %s dispatched %q, want %q", test.request.Method, test.request.URL.Path, service.called, test.wantCall)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Errorf("%s %s missing no-store", test.request.Method, test.request.URL.Path)
		}
	}
}

func TestControllerRejectsUnexpectedFieldsAndMissingUpdateFlags(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, request := range []*http.Request{
		authorizedRequest(http.MethodPost, "/api/v1/masters/offices", `{"code":"40006CC","unexpected":true}`),
		authorizedRequest(http.MethodPut, "/api/v1/masters/offices/40005CC", `{"name":"Office"}`),
	} {
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Errorf("%s %s status = %d, want 400", request.Method, request.URL.Path, response.StatusCode)
		}
	}
}

func TestControllerRequiresAuthentication(t *testing.T) {
	service := &serviceStub{}
	response, err := newTestApp(service).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/offices", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.called != "" {
		t.Fatalf("unauthenticated status/call = %d/%q", response.StatusCode, service.called)
	}
}

package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/masters/followupstatuses"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct{ called string }

func (service *serviceStub) List(context.Context, authentication.VerifiedIdentity) ([]followupstatuses.Status, error) {
	service.called = "list"
	return nil, nil
}
func (service *serviceStub) LookupOptions(context.Context, authentication.VerifiedIdentity) ([]followupstatuses.Status, error) {
	service.called = "lookup"
	return nil, nil
}
func (service *serviceStub) Get(context.Context, authentication.VerifiedIdentity, string) (followupstatuses.Status, error) {
	service.called = "get"
	return followupstatuses.Status{}, nil
}
func (service *serviceStub) Create(context.Context, authentication.VerifiedIdentity, followupstatuses.CreateCommand) (followupstatuses.Status, error) {
	service.called = "create"
	return followupstatuses.Status{}, nil
}
func (service *serviceStub) Update(context.Context, authentication.VerifiedIdentity, followupstatuses.UpdateCommand) (followupstatuses.Status, error) {
	service.called = "update"
	return followupstatuses.Status{}, nil
}
func (service *serviceStub) Delete(context.Context, authentication.VerifiedIdentity, string) error {
	service.called = "delete"
	return nil
}

type tokenVerifierStub struct{}

func (tokenVerifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "manager"}, nil
}
func newTestApp(service Service) *fiber.App {
	return httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(tokenVerifierStub{}, New(service)))
}
func authorizedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerRegistersFollowUpStatusCRUDAndLookup(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, test := range []struct {
		request *http.Request
		status  int
		action  string
	}{
		{authorizedRequest(http.MethodGet, "/api/v1/masters/follow-up-statuses", ""), http.StatusOK, "list"},
		{authorizedRequest(http.MethodGet, "/api/v1/masters/follow-up-statuses/options", ""), http.StatusOK, "lookup"},
		{authorizedRequest(http.MethodGet, "/api/v1/masters/follow-up-statuses/001", ""), http.StatusOK, "get"},
		{authorizedRequest(http.MethodPost, "/api/v1/masters/follow-up-statuses", `{"code":"003","description":null}`), http.StatusCreated, "create"},
		{authorizedRequest(http.MethodPut, "/api/v1/masters/follow-up-statuses/001", `{"description":null}`), http.StatusOK, "update"},
		{authorizedRequest(http.MethodDelete, "/api/v1/masters/follow-up-statuses/001", ""), http.StatusNoContent, "delete"},
		{authorizedRequest(http.MethodPut, "/api/v1/masters/follow-up-statuses/001", `{"other":true}`), http.StatusBadRequest, "delete"},
	} {
		response, err := app.Test(test.request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.status {
			t.Errorf("%s %s status = %d, want %d", test.request.Method, test.request.URL.Path, response.StatusCode, test.status)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Errorf("%s %s missing no-store", test.request.Method, test.request.URL.Path)
		}
		if test.status != http.StatusBadRequest && service.called != test.action {
			t.Errorf("action = %q, want %q", service.called, test.action)
		}
	}
}

func TestControllerRequiresAuthentication(t *testing.T) {
	service := &serviceStub{}
	response, err := newTestApp(service).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/follow-up-statuses/options", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.called != "" {
		t.Fatalf("unauthenticated status/call = %d/%q", response.StatusCode, service.called)
	}
}

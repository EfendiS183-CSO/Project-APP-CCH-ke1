package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/masters/handlingstatuses"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct{ called string }

func (service *serviceStub) List(context.Context, authentication.VerifiedIdentity) ([]handlingstatuses.Status, error) {
	service.called = "list"
	return nil, nil
}
func (service *serviceStub) Get(context.Context, authentication.VerifiedIdentity, string) (handlingstatuses.Status, error) {
	service.called = "view"
	return handlingstatuses.Status{}, nil
}
func (service *serviceStub) Create(context.Context, authentication.VerifiedIdentity, handlingstatuses.CreateCommand) (handlingstatuses.Status, error) {
	service.called = "add"
	return handlingstatuses.Status{}, nil
}
func (service *serviceStub) Update(context.Context, authentication.VerifiedIdentity, handlingstatuses.UpdateCommand) (handlingstatuses.Status, error) {
	service.called = "edit"
	return handlingstatuses.Status{}, nil
}
func (service *serviceStub) Deactivate(context.Context, authentication.VerifiedIdentity, string) error {
	service.called = "delete"
	return nil
}

type tokenVerifierStub struct{}

func (tokenVerifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "master.manager"}, nil
}
func newTestApp(service Service) *fiber.App {
	return httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(tokenVerifierStub{}, New(service)))
}
func authorizedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerOffersStrictHandlingStatusCRUD(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, test := range []struct {
		request *http.Request
		status  int
	}{
		{authorizedRequest(http.MethodGet, "/api/v1/masters/handling-statuses", ""), http.StatusOK},
		{authorizedRequest(http.MethodGet, "/api/v1/masters/handling-statuses/105", ""), http.StatusOK},
		{authorizedRequest(http.MethodPost, "/api/v1/masters/handling-statuses", `{"status_code":"107","description":"Tinjau"}`), http.StatusCreated},
		{authorizedRequest(http.MethodPut, "/api/v1/masters/handling-statuses/105", `{"description":"Review","active":true}`), http.StatusOK},
		{authorizedRequest(http.MethodDelete, "/api/v1/masters/handling-statuses/105", ""), http.StatusNoContent},
		{authorizedRequest(http.MethodPost, "/api/v1/masters/handling-statuses", `{"status_code":"107","description":"Status","other":1}`), http.StatusBadRequest},
		{authorizedRequest(http.MethodPut, "/api/v1/masters/handling-statuses/105", `{"description":"Review"}`), http.StatusBadRequest},
		{authorizedRequest(http.MethodDelete, "/api/v1/masters/handling-statuses/10", ""), http.StatusBadRequest},
	} {
		response, err := app.Test(test.request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.status {
			t.Errorf("%s %s status = %d, want %d", test.request.Method, test.request.URL.Path, response.StatusCode, test.status)
		}
		if test.status != http.StatusBadRequest && response.Header.Get("Cache-Control") != "private, no-store" {
			t.Errorf("%s %s missing no-store", test.request.Method, test.request.URL.Path)
		}
	}
	if service.called != "delete" {
		t.Fatalf("last route did not dispatch delete: %q", service.called)
	}
}

func TestControllerRequiresStaffAuthentication(t *testing.T) {
	service := &serviceStub{}
	response, err := newTestApp(service).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/handling-statuses", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.called != "" {
		t.Fatalf("unauthenticated status/call = %d/%q", response.StatusCode, service.called)
	}
}

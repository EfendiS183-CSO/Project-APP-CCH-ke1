package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/masters/timezones"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	value       timezones.Timezone
	values      []timezones.Timezone
	create      timezones.CreateCommand
	update      timezones.UpdateCommand
	getID       int64
	deletedID   int64
	listCalls   int
	addCalls    int
	editCalls   int
	deleteCalls int
	err         error
}

func (service *serviceStub) List(context.Context, authentication.VerifiedIdentity) ([]timezones.Timezone, error) {
	service.listCalls++
	return service.values, service.err
}
func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, id int64) (timezones.Timezone, error) {
	service.getID = id
	return service.value, service.err
}
func (service *serviceStub) Create(_ context.Context, _ authentication.VerifiedIdentity, command timezones.CreateCommand) (timezones.Timezone, error) {
	service.addCalls++
	service.create = command
	return service.value, service.err
}
func (service *serviceStub) Update(_ context.Context, _ authentication.VerifiedIdentity, command timezones.UpdateCommand) (timezones.Timezone, error) {
	service.editCalls++
	service.update = command
	return service.value, service.err
}
func (service *serviceStub) Delete(_ context.Context, _ authentication.VerifiedIdentity, id int64) error {
	service.deleteCalls++
	service.deletedID = id
	return service.err
}

type tokenVerifierStub struct{}

func (tokenVerifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "master.admin"}, nil
}

func newTestApp(service Service) *fiber.App {
	return httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(tokenVerifierStub{}, New(service)))
}

func authorizedRequest(method string, target string, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerProvidesTimezoneCRUDRoutes(t *testing.T) {
	service := &serviceStub{value: timezones.Timezone{ID: 1, Timezone: "Asia/Jakarta", IsDefault: true}, values: []timezones.Timezone{{ID: 1, Timezone: "Asia/Jakarta", IsDefault: true}}}
	app := newTestApp(service)
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/masters/timezones", ""), wantStatus: http.StatusOK},
		{name: "get", request: authorizedRequest(http.MethodGet, "/api/v1/masters/timezones/1", ""), wantStatus: http.StatusOK},
		{name: "create", request: authorizedRequest(http.MethodPost, "/api/v1/masters/timezones", `{"timezone":"Asia/Jayapura","is_default":false}`), wantStatus: http.StatusCreated},
		{name: "update", request: authorizedRequest(http.MethodPut, "/api/v1/masters/timezones/1", `{"timezone":"Asia/Makassar","is_default":true}`), wantStatus: http.StatusOK},
		{name: "delete", request: authorizedRequest(http.MethodDelete, "/api/v1/masters/timezones/1", ""), wantStatus: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, err := app.Test(test.request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if response.Header.Get("Cache-Control") != "private, no-store" {
				t.Fatalf("Cache-Control = %q, want private, no-store", response.Header.Get("Cache-Control"))
			}
		})
	}
	if service.listCalls != 1 || service.getID != 1 || service.addCalls != 1 || service.create.Timezone != "Asia/Jayapura" || service.editCalls != 1 || service.update.ID != 1 || service.update.Timezone != "Asia/Makassar" || !service.update.IsDefault || service.deleteCalls != 1 || service.deletedID != 1 {
		t.Fatalf("timezone CRUD calls did not match requests: %#v", service)
	}
}

func TestControllerRejectsInvalidTimezoneRequestsAndRequiresAuthentication(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, request := range []*http.Request{
		authorizedRequest(http.MethodPost, "/api/v1/masters/timezones", `{"timezone":"Asia/Jakarta","extra":1}`),
		authorizedRequest(http.MethodPost, "/api/v1/masters/timezones", ``),
		authorizedRequest(http.MethodPost, "/api/v1/masters/timezones", `{"timezone":"Asia/Jakarta"}{"timezone":"x"}`),
		authorizedRequest(http.MethodPut, "/api/v1/masters/timezones/abc", `{"timezone":"Asia/Jakarta"}`),
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
	if service.addCalls != 0 || service.editCalls != 0 {
		t.Fatalf("invalid body reached service: add=%d edit=%d", service.addCalls, service.editCalls)
	}
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/timezones", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.listCalls != 0 {
		t.Fatalf("unauthenticated status/calls = %d/%d, want 401/0", response.StatusCode, service.listCalls)
	}
}

func TestControllerMapsServiceFailuresToStatusCodes(t *testing.T) {
	for _, test := range []struct {
		err        error
		wantStatus int
	}{
		{err: timezones.ErrInvalidInput, wantStatus: http.StatusBadRequest},
		{err: timezones.ErrNotFound, wantStatus: http.StatusNotFound},
		{err: timezones.ErrDuplicate, wantStatus: http.StatusConflict},
		{err: timezones.ErrOnlyOneDefault, wantStatus: http.StatusConflict},
		{err: timezones.ErrCannotDeleteDefault, wantStatus: http.StatusBadRequest},
	} {
		service := &serviceStub{err: test.err}
		response, err := newTestApp(service).Test(authorizedRequest(http.MethodGet, "/api/v1/masters/timezones/1", ""))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.wantStatus {
			t.Errorf("error %v status = %d, want %d", test.err, response.StatusCode, test.wantStatus)
		}
	}
}

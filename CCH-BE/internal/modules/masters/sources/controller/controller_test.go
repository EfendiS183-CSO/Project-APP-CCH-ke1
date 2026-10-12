package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/masters/sources"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	value        sources.Source
	values       []sources.Source
	create       sources.CreateCommand
	update       sources.UpdateCommand
	getID        int64
	deactivateID int64
	listCalls    int
	getCalls     int
	addCalls     int
	editCalls    int
	deleteCalls  int
	err          error
}

func (service *serviceStub) List(context.Context, authentication.VerifiedIdentity) ([]sources.Source, error) {
	service.listCalls++
	return service.values, service.err
}

func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, id int64) (sources.Source, error) {
	service.getCalls++
	service.getID = id
	return service.value, service.err
}

func (service *serviceStub) Create(_ context.Context, _ authentication.VerifiedIdentity, command sources.CreateCommand) (sources.Source, error) {
	service.addCalls++
	service.create = command
	return service.value, service.err
}

func (service *serviceStub) Update(_ context.Context, _ authentication.VerifiedIdentity, command sources.UpdateCommand) (sources.Source, error) {
	service.editCalls++
	service.update = command
	return service.value, service.err
}

func (service *serviceStub) Deactivate(_ context.Context, _ authentication.VerifiedIdentity, id int64) error {
	service.deleteCalls++
	service.deactivateID = id
	return service.err
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

func authorizedRequest(method string, target string, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerProvidesComplaintSourceCRUDRoutes(t *testing.T) {
	service := &serviceStub{value: sources.Source{ID: 22, Name: "Portal", Active: true}, values: []sources.Source{{ID: 22, Name: "Portal", Active: true}}}
	app := newTestApp(service)
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/masters/complaint-sources", ""), wantStatus: http.StatusOK},
		{name: "get", request: authorizedRequest(http.MethodGet, "/api/v1/masters/complaint-sources/22", ""), wantStatus: http.StatusOK},
		{name: "create", request: authorizedRequest(http.MethodPost, "/api/v1/masters/complaint-sources", `{"name":"Portal"}`), wantStatus: http.StatusCreated},
		{name: "update", request: authorizedRequest(http.MethodPut, "/api/v1/masters/complaint-sources/22", `{"name":"Portal Baru","active":true}`), wantStatus: http.StatusOK},
		{name: "deactivate", request: authorizedRequest(http.MethodDelete, "/api/v1/masters/complaint-sources/22", ""), wantStatus: http.StatusNoContent},
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
	if service.listCalls != 1 || service.getCalls != 1 || service.getID != 22 || service.addCalls != 1 || service.create.Name != "Portal" || !service.create.Active || service.editCalls != 1 || service.update.ID != 22 || service.update.Name != "Portal Baru" || service.deleteCalls != 1 || service.deactivateID != 22 {
		t.Fatalf("source CRUD calls did not match requests: %#v", service)
	}
}

func TestControllerRejectsInvalidSourceRequestsAndRequiresAuthentication(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, request := range []*http.Request{
		authorizedRequest(http.MethodPost, "/api/v1/masters/complaint-sources", `{"name":"Portal","other":1}`),
		authorizedRequest(http.MethodPut, "/api/v1/masters/complaint-sources/0", `{"name":"Portal","active":true}`),
		authorizedRequest(http.MethodPut, "/api/v1/masters/complaint-sources/2", `{"name":"Portal"}`),
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
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/complaint-sources", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.listCalls != 0 {
		t.Fatalf("unauthenticated status/calls = %d/%d, want 401/0", response.StatusCode, service.listCalls)
	}
}

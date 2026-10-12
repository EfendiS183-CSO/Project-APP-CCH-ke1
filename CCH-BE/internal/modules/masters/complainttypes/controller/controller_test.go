package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/masters/complainttypes"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	value      complainttypes.ComplaintType
	list       []complainttypes.ComplaintType
	create     complainttypes.CreateCommand
	update     complainttypes.UpdateCommand
	getCode    int64
	deleteCode int64
	listCalls  int
	getCalls   int
	addCalls   int
	editCalls  int
	delCalls   int
	err        error
}

func (service *serviceStub) List(context.Context, authentication.VerifiedIdentity) ([]complainttypes.ComplaintType, error) {
	service.listCalls++
	return service.list, service.err
}

func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, code int64) (complainttypes.ComplaintType, error) {
	service.getCalls++
	service.getCode = code
	return service.value, service.err
}

func (service *serviceStub) Create(_ context.Context, _ authentication.VerifiedIdentity, command complainttypes.CreateCommand) (complainttypes.ComplaintType, error) {
	service.addCalls++
	service.create = command
	return service.value, service.err
}

func (service *serviceStub) Update(_ context.Context, _ authentication.VerifiedIdentity, command complainttypes.UpdateCommand) (complainttypes.ComplaintType, error) {
	service.editCalls++
	service.update = command
	return service.value, service.err
}

func (service *serviceStub) Deactivate(_ context.Context, _ authentication.VerifiedIdentity, code int64) error {
	service.delCalls++
	service.deleteCode = code
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

func TestControllerProvidesComplaintTypeCRUDRoutes(t *testing.T) {
	description := "Jenis keluhan"
	service := &serviceStub{
		value: complainttypes.ComplaintType{Code: 7, Description: &description, Active: true},
		list:  []complainttypes.ComplaintType{{Code: 7, Description: &description, Active: true}},
	}
	app := newTestApp(service)
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/masters/complaint-types", ""), wantStatus: http.StatusOK},
		{name: "get", request: authorizedRequest(http.MethodGet, "/api/v1/masters/complaint-types/7", ""), wantStatus: http.StatusOK},
		{name: "create", request: authorizedRequest(http.MethodPost, "/api/v1/masters/complaint-types", `{"code":7,"description":"Jenis keluhan"}`), wantStatus: http.StatusCreated},
		{name: "update", request: authorizedRequest(http.MethodPut, "/api/v1/masters/complaint-types/7", `{"description":"Jenis diperbarui","active":true}`), wantStatus: http.StatusOK},
		{name: "deactivate", request: authorizedRequest(http.MethodDelete, "/api/v1/masters/complaint-types/7", ""), wantStatus: http.StatusNoContent},
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
	if service.listCalls != 1 || service.getCalls != 1 || service.getCode != 7 || service.addCalls != 1 || service.create.Code != 7 || service.create.Active != true || service.editCalls != 1 || service.update.Code != 7 || service.update.Description == nil || *service.update.Description != "Jenis diperbarui" || service.delCalls != 1 || service.deleteCode != 7 {
		t.Fatalf("CRUD service calls did not match HTTP requests: %#v", service)
	}
}

func TestControllerRejectsInvalidAndUnknownCRUDFields(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, request := range []*http.Request{
		authorizedRequest(http.MethodPost, "/api/v1/masters/complaint-types", `{"code":1,"unexpected":true}`),
		authorizedRequest(http.MethodPost, "/api/v1/masters/complaint-types", `{"description":"missing code"}`),
		authorizedRequest(http.MethodPut, "/api/v1/masters/complaint-types/invalid", `{"active":true}`),
		authorizedRequest(http.MethodPut, "/api/v1/masters/complaint-types/1", `{"unexpected":true}`),
	} {
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Errorf("request %s %s status = %d, want 400", request.Method, request.URL.Path, response.StatusCode)
		}
	}
	if service.addCalls != 0 || service.editCalls != 0 {
		t.Fatal("invalid request reached master service")
	}
}

func TestControllerRequiresAuthenticationAndMapsConflict(t *testing.T) {
	service := &serviceStub{err: complainttypes.ErrAlreadyExists}
	app := newTestApp(service)
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/complaint-types", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", response.StatusCode)
	}
	response, err = app.Test(authorizedRequest(http.MethodPost, "/api/v1/masters/complaint-types", `{"code":7}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate code status = %d, want 409", response.StatusCode)
	}
}

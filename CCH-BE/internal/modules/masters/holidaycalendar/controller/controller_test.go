package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/masters/holidaycalendar"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	value       holidaycalendar.Holiday
	values      []holidaycalendar.Holiday
	create      holidaycalendar.CreateCommand
	update      holidaycalendar.UpdateCommand
	getDate     string
	deletedDate string
	listCalls   int
	addCalls    int
	editCalls   int
	deleteCalls int
	err         error
}

func (service *serviceStub) List(context.Context, authentication.VerifiedIdentity) ([]holidaycalendar.Holiday, error) {
	service.listCalls++
	return service.values, service.err
}

func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, date string) (holidaycalendar.Holiday, error) {
	service.getDate = date
	return service.value, service.err
}

func (service *serviceStub) Create(_ context.Context, _ authentication.VerifiedIdentity, command holidaycalendar.CreateCommand) (holidaycalendar.Holiday, error) {
	service.addCalls++
	service.create = command
	return service.value, service.err
}

func (service *serviceStub) Update(_ context.Context, _ authentication.VerifiedIdentity, command holidaycalendar.UpdateCommand) (holidaycalendar.Holiday, error) {
	service.editCalls++
	service.update = command
	return service.value, service.err
}

func (service *serviceStub) Delete(_ context.Context, _ authentication.VerifiedIdentity, date string) error {
	service.deleteCalls++
	service.deletedDate = date
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

func TestControllerProvidesHolidayCalendarCRUDRoutes(t *testing.T) {
	service := &serviceStub{value: holidaycalendar.Holiday{Date: "2026-08-17", Description: "Kemerdekaan"}, values: []holidaycalendar.Holiday{{Date: "2026-08-17", Description: "Kemerdekaan"}}}
	app := newTestApp(service)
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/masters/holiday-calendar", ""), wantStatus: http.StatusOK},
		{name: "get", request: authorizedRequest(http.MethodGet, "/api/v1/masters/holiday-calendar/2026-08-17", ""), wantStatus: http.StatusOK},
		{name: "create", request: authorizedRequest(http.MethodPost, "/api/v1/masters/holiday-calendar", `{"date":"2026-08-17","description":"Kemerdekaan"}`), wantStatus: http.StatusCreated},
		{name: "update", request: authorizedRequest(http.MethodPut, "/api/v1/masters/holiday-calendar/2026-08-17", `{"description":"HUT RI"}`), wantStatus: http.StatusOK},
		{name: "delete", request: authorizedRequest(http.MethodDelete, "/api/v1/masters/holiday-calendar/2026-08-17", ""), wantStatus: http.StatusNoContent},
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
	if service.listCalls != 1 || service.getDate != "2026-08-17" || service.addCalls != 1 || service.create.Date != "2026-08-17" || service.editCalls != 1 || service.update.Date != "2026-08-17" || service.update.Description != "HUT RI" || service.deleteCalls != 1 || service.deletedDate != "2026-08-17" {
		t.Fatalf("holiday CRUD calls did not match requests: %#v", service)
	}
}

func TestControllerRejectsInvalidHolidayRequestsAndRequiresAuthentication(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	// The transport rejects bodies that are empty, oversized, non-JSON, contain
	// unknown fields, or carry trailing data. Required-field checks live in the
	// service, so a well-formed but incomplete body is forwarded, not rejected.
	for _, request := range []*http.Request{
		authorizedRequest(http.MethodPost, "/api/v1/masters/holiday-calendar", `{"date":"2026-08-17","description":"Kemerdekaan","extra":1}`),
		authorizedRequest(http.MethodPost, "/api/v1/masters/holiday-calendar", ``),
		authorizedRequest(http.MethodPost, "/api/v1/masters/holiday-calendar", `{"date":"2026-08-17"}{"date":"2026-01-01"}`),
		authorizedRequest(http.MethodPut, "/api/v1/masters/holiday-calendar/2026-08-17", `{"description":"HUT RI","extra":true}`),
		authorizedRequest(http.MethodPut, "/api/v1/masters/holiday-calendar/2026-08-17", `not-json`),
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
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/holiday-calendar", nil))
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
		{err: holidaycalendar.ErrInvalidInput, wantStatus: http.StatusBadRequest},
		{err: holidaycalendar.ErrNotFound, wantStatus: http.StatusNotFound},
		{err: holidaycalendar.ErrAlreadyExists, wantStatus: http.StatusConflict},
	} {
		service := &serviceStub{err: test.err}
		response, err := newTestApp(service).Test(authorizedRequest(http.MethodGet, "/api/v1/masters/holiday-calendar/2026-08-17", ""))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.wantStatus {
			t.Errorf("error %v status = %d, want %d", test.err, response.StatusCode, test.wantStatus)
		}
	}
}

package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/masters/languages"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	value       languages.Language
	values      []languages.Language
	create      languages.CreateCommand
	update      languages.UpdateCommand
	getCode     string
	deletedCode string
	listCalls   int
	addCalls    int
	editCalls   int
	deleteCalls int
	err         error
}

func (service *serviceStub) List(context.Context, authentication.VerifiedIdentity) ([]languages.Language, error) {
	service.listCalls++
	return service.values, service.err
}
func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, code string) (languages.Language, error) {
	service.getCode = code
	return service.value, service.err
}
func (service *serviceStub) Create(_ context.Context, _ authentication.VerifiedIdentity, command languages.CreateCommand) (languages.Language, error) {
	service.addCalls++
	service.create = command
	return service.value, service.err
}
func (service *serviceStub) Update(_ context.Context, _ authentication.VerifiedIdentity, command languages.UpdateCommand) (languages.Language, error) {
	service.editCalls++
	service.update = command
	return service.value, service.err
}
func (service *serviceStub) Delete(_ context.Context, _ authentication.VerifiedIdentity, code string) error {
	service.deleteCalls++
	service.deletedCode = code
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

func authorizedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerProvidesLanguageCRUDRoutes(t *testing.T) {
	service := &serviceStub{value: languages.Language{Code: "ID", Name: "Indonesia"}, values: []languages.Language{{Code: "ID", Name: "Indonesia"}}}
	app := newTestApp(service)
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/masters/languages", ""), wantStatus: http.StatusOK},
		{name: "get", request: authorizedRequest(http.MethodGet, "/api/v1/masters/languages/ID", ""), wantStatus: http.StatusOK},
		{name: "create", request: authorizedRequest(http.MethodPost, "/api/v1/masters/languages", `{"language_code":"EN","language_name":"English","site_logo":"en.png","site_title":"CCH","terms_and_condition_text":"Terms","announcement_text":"News","about_text":"About"}`), wantStatus: http.StatusCreated},
		{name: "update", request: authorizedRequest(http.MethodPut, "/api/v1/masters/languages/ID", `{"language_name":"Bahasa","site_logo":"logo.png","site_title":"CCH","terms_and_condition_text":"Syarat","announcement_text":"Pengumuman","about_text":"Tentang"}`), wantStatus: http.StatusOK},
		{name: "delete", request: authorizedRequest(http.MethodDelete, "/api/v1/masters/languages/EN", ""), wantStatus: http.StatusNoContent},
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
	if service.listCalls != 1 || service.getCode != "ID" || service.addCalls != 1 || service.create.Code != "EN" || service.editCalls != 1 || service.update.Code != "ID" || service.deleteCalls != 1 || service.deletedCode != "EN" {
		t.Fatalf("language CRUD calls did not match requests: %#v", service)
	}
}

func TestControllerRejectsInvalidLanguageRequestsAndRequiresAuthentication(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, request := range []*http.Request{
		authorizedRequest(http.MethodPost, "/api/v1/masters/languages", `{"language_code":"EN","language_name":"English","extra":1}`),
		authorizedRequest(http.MethodPost, "/api/v1/masters/languages", ``),
		authorizedRequest(http.MethodPost, "/api/v1/masters/languages", `{"language_code":"EN"}{"language_code":"FR"}`),
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
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/languages", nil))
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
		{err: languages.ErrInvalidInput, wantStatus: http.StatusBadRequest},
		{err: languages.ErrNotFound, wantStatus: http.StatusNotFound},
		{err: languages.ErrDuplicate, wantStatus: http.StatusConflict},
		{err: languages.ErrOnlyOneDefault, wantStatus: http.StatusConflict},
		{err: languages.ErrCannotDeleteDefault, wantStatus: http.StatusBadRequest},
	} {
		service := &serviceStub{err: test.err}
		response, err := newTestApp(service).Test(authorizedRequest(http.MethodGet, "/api/v1/masters/languages/ID", ""))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.wantStatus {
			t.Errorf("error %v status = %d, want %d", test.err, response.StatusCode, test.wantStatus)
		}
	}
}

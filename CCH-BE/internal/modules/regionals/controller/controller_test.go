package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/regionals"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	page       regionals.Page
	office     regionals.Office
	options    []regionals.OfficeOption
	query      regionals.ListQuery
	getCode    string
	update     regionals.UpdateCommand
	listCalls  int
	getCalls   int
	optionCall int
	editCalls  int
	err        error
}

func (service *serviceStub) List(_ context.Context, _ authentication.VerifiedIdentity, query regionals.ListQuery) (regionals.Page, error) {
	service.listCalls++
	service.query = query
	return service.page, service.err
}
func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, code string) (regionals.Office, error) {
	service.getCalls++
	service.getCode = code
	return service.office, service.err
}
func (service *serviceStub) LookupOptions(context.Context, authentication.VerifiedIdentity) ([]regionals.OfficeOption, error) {
	service.optionCall++
	return service.options, service.err
}
func (service *serviceStub) Update(_ context.Context, _ authentication.VerifiedIdentity, command regionals.UpdateCommand) (regionals.Office, error) {
	service.editCalls++
	service.update = command
	return service.office, service.err
}

type tokenVerifierStub struct{}

func (tokenVerifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "regional.admin"}, nil
}

func newTestApp(service Service) *fiber.App {
	return httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(tokenVerifierStub{}, New(service)))
}

func authorizedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerProvidesRegionalReadAndEditRoutes(t *testing.T) {
	service := &serviceStub{
		page:    regionals.Page{Data: []regionals.Office{{Code: "40005", Name: "Kantor Pusat", Type: "KP"}}, Total: 1, Limit: 25},
		office:  regionals.Office{Code: "40005", Name: "Kantor Pusat", Type: "KP"},
		options: []regionals.OfficeOption{{Code: "40005", Name: "Kantor Pusat", Type: "KP"}},
	}
	app := newTestApp(service)
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/admin/regional-offices?limit=10&offset=20", ""), wantStatus: http.StatusOK},
		{name: "lookup options", request: authorizedRequest(http.MethodGet, "/api/v1/admin/regional-offices/options", ""), wantStatus: http.StatusOK},
		{name: "view", request: authorizedRequest(http.MethodGet, "/api/v1/admin/regional-offices/40005", ""), wantStatus: http.StatusOK},
		{name: "edit", request: authorizedRequest(http.MethodPut, "/api/v1/admin/regional-offices/40005", `{"office_name":"Kantor Pusat Baru","abbreviation":"KP","type":"KP","regional_code":"40005","active":true}`), wantStatus: http.StatusOK},
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
	if service.listCalls != 1 || service.query.Limit != 10 || service.query.Offset != 20 || service.optionCall != 1 || service.getCalls != 1 || service.getCode != "40005" || service.editCalls != 1 || service.update.Code != "40005" || service.update.Name != "Kantor Pusat Baru" {
		t.Fatalf("regional routes did not forward expected requests: %#v", service)
	}
}

func TestControllerRejectsInvalidRegionalRequestsAndRequiresAuthentication(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, test := range []struct {
		name    string
		request *http.Request
	}{
		{name: "bad limit", request: authorizedRequest(http.MethodGet, "/api/v1/admin/regional-offices?limit=bad", "")},
		{name: "unknown field", request: authorizedRequest(http.MethodPut, "/api/v1/admin/regional-offices/40005", `{"office_name":"Kantor","unexpected":true,"active":true}`)},
		{name: "missing active", request: authorizedRequest(http.MethodPut, "/api/v1/admin/regional-offices/40005", `{"office_name":"Kantor"}`)},
		{name: "trailing json", request: authorizedRequest(http.MethodPut, "/api/v1/admin/regional-offices/40005", `{"office_name":"Kantor"}{"office_name":"Lain"}`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, err := app.Test(test.request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("%s %s status = %d, want 400", test.request.Method, test.request.URL.String(), response.StatusCode)
			}
		})
	}
	if service.editCalls != 0 {
		t.Fatalf("invalid payload reached service: %d", service.editCalls)
	}
	service.listCalls = 0
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/admin/regional-offices", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.listCalls != 0 {
		t.Fatalf("unauthenticated status/calls = %d/%d, want 401/0", response.StatusCode, service.listCalls)
	}
}

func TestControllerMapsRegionalErrors(t *testing.T) {
	for _, test := range []struct {
		err        error
		wantStatus int
	}{
		{err: regionals.ErrInvalidInput, wantStatus: http.StatusBadRequest},
		{err: regionals.ErrNotFound, wantStatus: http.StatusNotFound},
		{err: regionals.ErrInvalidRegional, wantStatus: http.StatusConflict},
	} {
		service := &serviceStub{err: test.err}
		response, err := newTestApp(service).Test(authorizedRequest(http.MethodGet, "/api/v1/admin/regional-offices/40005", ""))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.wantStatus {
			t.Errorf("error %v status = %d, want %d", test.err, response.StatusCode, test.wantStatus)
		}
	}
}

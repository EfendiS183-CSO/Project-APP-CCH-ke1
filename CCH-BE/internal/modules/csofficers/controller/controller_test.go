package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/csofficers"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	page      csofficers.Page
	officer   csofficers.Officer
	query     csofficers.ListQuery
	username  string
	listCalls int
	getCalls  int
	err       error
}

func (service *serviceStub) List(_ context.Context, _ authentication.VerifiedIdentity, query csofficers.ListQuery) (csofficers.Page, error) {
	service.listCalls++
	service.query = query
	return service.page, service.err
}
func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, username string) (csofficers.Officer, error) {
	service.getCalls++
	service.username = username
	return service.officer, service.err
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

func authorizedRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerProvidesCSOfficerListAndDetail(t *testing.T) {
	service := &serviceStub{
		page:    csofficers.Page{Data: []csofficers.Officer{{Username: "cs.user", Status: stringPointer("Login")}}, Total: 1, Limit: 10, Offset: 20},
		officer: csofficers.Officer{Username: "cs.user", Status: stringPointer("Login")},
	}
	app := newTestApp(service)
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/admin/cs-officers?regional=40005&office=40005CC&status=Login&q=cs&limit=10&offset=20"), wantStatus: http.StatusOK},
		{name: "detail", request: authorizedRequest(http.MethodGet, "/api/v1/admin/cs-officers/cs.user"), wantStatus: http.StatusOK},
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
			if response.Header.Get("Cache-Control") != "private, no-store" || response.Header.Get("Pragma") != "no-cache" {
				t.Fatalf("sensitive CS roster response not no-store: headers=%v", response.Header)
			}
		})
	}
	if service.listCalls != 1 || service.query.Regional != "40005" || service.query.Office != "40005CC" || service.query.Status != "Login" || service.query.Search != "cs" || service.query.Limit != 10 || service.query.Offset != 20 || service.getCalls != 1 || service.username != "cs.user" {
		t.Fatalf("CS officer requests not forwarded: %#v", service)
	}
}

func TestControllerRejectsMalformedQueriesAndUnauthenticatedRequests(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, test := range []struct {
		name    string
		request *http.Request
	}{
		{name: "bad limit", request: authorizedRequest(http.MethodGet, "/api/v1/admin/cs-officers?limit=abc")},
		{name: "bad offset", request: authorizedRequest(http.MethodGet, "/api/v1/admin/cs-officers?offset=abc")},
		{name: "empty username", request: authorizedRequest(http.MethodGet, "/api/v1/admin/cs-officers/%20")},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, err := app.Test(test.request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.StatusCode)
			}
		})
	}
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/admin/cs-officers", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.listCalls != 0 {
		t.Fatalf("unauthenticated status/calls = %d/%d, want 401/0", response.StatusCode, service.listCalls)
	}
}

func TestControllerMapsCSOfficerErrors(t *testing.T) {
	for _, test := range []struct {
		err        error
		wantStatus int
	}{
		{err: csofficers.ErrInvalidInput, wantStatus: http.StatusBadRequest},
		{err: csofficers.ErrNotFound, wantStatus: http.StatusNotFound},
	} {
		service := &serviceStub{err: test.err}
		response, err := newTestApp(service).Test(authorizedRequest(http.MethodGet, "/api/v1/admin/cs-officers/cs.user"))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.wantStatus {
			t.Errorf("error %v status = %d, want %d", test.err, response.StatusCode, test.wantStatus)
		}
	}
}

func stringPointer(value string) *string { return &value }

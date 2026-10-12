package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/userlevels"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	page      userlevels.Page
	level     userlevels.UserLevel
	query     userlevels.ListQuery
	id        int64
	listCalls int
	getCalls  int
	err       error
}

func (service *serviceStub) List(_ context.Context, _ authentication.VerifiedIdentity, query userlevels.ListQuery) (userlevels.Page, error) {
	service.listCalls++
	service.query = query
	return service.page, service.err
}
func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, id int64) (userlevels.UserLevel, error) {
	service.getCalls++
	service.id = id
	return service.level, service.err
}

type tokenVerifierStub struct{}

func (tokenVerifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "security.admin"}, nil
}

func newTestApp(service Service) *fiber.App {
	return httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(tokenVerifierStub{}, New(service)))
}

func authorizedRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerListsAndGetsRoleCatalog(t *testing.T) {
	service := &serviceStub{
		page:  userlevels.Page{Data: []userlevels.UserLevel{{ID: 3, Name: "Call Center"}}, Total: 1, Limit: 10, Offset: 20},
		level: userlevels.UserLevel{ID: -2, Name: "Anonymous"},
	}
	app := newTestApp(service)
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/admin/user-levels?limit=10&offset=20"), wantStatus: http.StatusOK},
		{name: "detail negative legacy ID", request: authorizedRequest(http.MethodGet, "/api/v1/admin/user-levels/-2"), wantStatus: http.StatusOK},
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
				t.Fatalf("role catalog response lacks no-store headers: %v", response.Header)
			}
		})
	}
	if service.listCalls != 1 || service.query.Limit != 10 || service.query.Offset != 20 || service.getCalls != 1 || service.id != -2 {
		t.Fatalf("role catalog calls = %#v", service)
	}
}

func TestControllerRejectsMalformedPaginationAndIDsAndRequiresAuth(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, request := range []*http.Request{
		authorizedRequest(http.MethodGet, "/api/v1/admin/user-levels?limit=abc"),
		authorizedRequest(http.MethodGet, "/api/v1/admin/user-levels?offset=abc"),
		authorizedRequest(http.MethodGet, "/api/v1/admin/user-levels/not-an-id"),
	} {
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", request.URL.String(), response.StatusCode)
		}
	}
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/admin/user-levels", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.listCalls != 0 {
		t.Fatalf("unauthenticated status/calls = %d/%d, want 401/0", response.StatusCode, service.listCalls)
	}
}

func TestControllerMapsUserLevelErrors(t *testing.T) {
	service := &serviceStub{err: userlevels.ErrNotFound}
	response, err := newTestApp(service).Test(authorizedRequest(http.MethodGet, "/api/v1/admin/user-levels/3"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("not-found status = %d, want 404", response.StatusCode)
	}
}

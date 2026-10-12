package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/breadcrumbs"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	page      breadcrumbs.Page
	node      breadcrumbs.Breadcrumb
	ancestors []breadcrumbs.Ancestor
	created   breadcrumbs.Breadcrumb
	parent    string
	title     string
	url       string
	moveTo    string
	deleted   string
	listCalls int
	err       error
}

func (service *serviceStub) List(_ context.Context, _ authentication.VerifiedIdentity, _ breadcrumbs.ListQuery) (breadcrumbs.Page, error) {
	service.listCalls++
	return service.page, service.err
}
func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, title string) (breadcrumbs.Breadcrumb, error) {
	service.title = title
	return service.node, service.err
}
func (service *serviceStub) Check(_ context.Context, _ authentication.VerifiedIdentity, title string) ([]breadcrumbs.Ancestor, error) {
	service.title = title
	return service.ancestors, service.err
}
func (service *serviceStub) Add(_ context.Context, _ authentication.VerifiedIdentity, parent, title, url string) (breadcrumbs.Breadcrumb, error) {
	service.parent, service.title, service.url = parent, title, url
	return service.created, service.err
}
func (service *serviceStub) Move(_ context.Context, _ authentication.VerifiedIdentity, title, newRoot string) error {
	service.title, service.moveTo = title, newRoot
	return service.err
}
func (service *serviceStub) Delete(_ context.Context, _ authentication.VerifiedIdentity, title string) error {
	service.deleted = title
	return service.err
}

type verifierStub struct{}

func (verifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "security.admin"}, nil
}

func newTestApp(service Service) *fiber.App {
	return httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(verifierStub{}, New(service)))
}

func authorizedRequest(method, target, body string) *http.Request {
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	request := httptest.NewRequest(method, target, reader)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func TestControllerExposesBreadcrumbWorkflows(t *testing.T) {
	service := &serviceStub{
		page:      breadcrumbs.Page{Data: []breadcrumbs.Breadcrumb{{PageTitle: "Home", Left: 1, Right: 2}}, Total: 1},
		node:      breadcrumbs.Breadcrumb{PageTitle: "Home", PageURL: "/", Left: 1, Right: 2},
		ancestors: []breadcrumbs.Ancestor{{PageTitle: "Home", PageURL: "/"}},
		created:   breadcrumbs.Breadcrumb{PageTitle: "Contact", PageURL: "/contact", Left: 3, Right: 4},
	}
	app := newTestApp(service)
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/admin/breadcrumbs?limit=25", ""), wantStatus: http.StatusOK},
		{name: "get", request: authorizedRequest(http.MethodGet, "/api/v1/admin/breadcrumbs/Home", ""), wantStatus: http.StatusOK},
		{name: "ancestors", request: authorizedRequest(http.MethodGet, "/api/v1/admin/breadcrumbs/Home/ancestors", ""), wantStatus: http.StatusOK},
		{name: "add", request: authorizedRequest(http.MethodPost, "/api/v1/admin/breadcrumbs", `{"parent_title":"Home","page_title":"Contact","page_url":"/contact"}`), wantStatus: http.StatusCreated},
		{name: "move", request: authorizedRequest(http.MethodPut, "/api/v1/admin/breadcrumbs/Home/move", `{"new_root":"About"}`), wantStatus: http.StatusNoContent},
		{name: "delete", request: authorizedRequest(http.MethodDelete, "/api/v1/admin/breadcrumbs/Home", ""), wantStatus: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, err := app.Test(test.request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("%s status = %d, want %d", test.name, response.StatusCode, test.wantStatus)
			}
			if response.Header.Get("Cache-Control") != "private, no-store" || response.Header.Get("Pragma") != "no-cache" {
				t.Fatalf("%s missing no-store headers: %v", test.name, response.Header)
			}
		})
	}
	if service.listCalls != 1 || service.title != "Home" || service.parent != "Home" || service.url != "/contact" || service.moveTo != "About" || service.deleted != "Home" {
		t.Fatalf("unexpected stub calls: %#v", service)
	}
}

func TestControllerRejectsBadInputAndUnauthenticated(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, request := range []*http.Request{
		authorizedRequest(http.MethodGet, "/api/v1/admin/breadcrumbs?limit=abc", ""),
		authorizedRequest(http.MethodPost, "/api/v1/admin/breadcrumbs", `{"parent_title":1}`),
		httptest.NewRequest(http.MethodGet, "/api/v1/admin/breadcrumbs", nil),
	} {
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		want := http.StatusBadRequest
		if request.Header.Get("X-Authorization") == "" {
			want = http.StatusUnauthorized
		}
		if response.StatusCode != want {
			t.Errorf("%s status = %d, want %d", request.URL.String(), response.StatusCode, want)
		}
	}
}

func TestControllerMapsBreadcrumbErrors(t *testing.T) {
	for _, test := range []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "not found", err: breadcrumbs.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "duplicate", err: breadcrumbs.ErrDuplicateTitle, wantStatus: http.StatusConflict},
		{name: "invalid parent", err: breadcrumbs.ErrInvalidParent, wantStatus: http.StatusConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &serviceStub{err: test.err}
			response, err := newTestApp(service).Test(authorizedRequest(http.MethodGet, "/api/v1/admin/breadcrumbs/Home", ""))
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("%s status = %d, want %d", test.name, response.StatusCode, test.wantStatus)
			}
		})
	}
}

package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/announcements"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct{ called string }

func (service *serviceStub) List(_ context.Context, _ authentication.VerifiedIdentity, query announcements.ListQuery) (announcements.Page, error) {
	service.called = "list"
	return announcements.Page{Data: []announcements.Announcement{}, Limit: query.Limit, Offset: query.Offset}, nil
}
func (service *serviceStub) Options(context.Context, authentication.VerifiedIdentity) (announcements.Options, error) {
	service.called = "options"
	return announcements.Options{Languages: []announcements.LanguageOption{{Code: "id", Name: "Indonesia"}}}, nil
}
func (service *serviceStub) Get(context.Context, authentication.VerifiedIdentity, int64) (announcements.Announcement, error) {
	service.called = "get"
	return announcements.Announcement{ID: 12}, nil
}
func (service *serviceStub) Create(context.Context, authentication.VerifiedIdentity, announcements.CreateCommand) (announcements.Announcement, error) {
	service.called = "create"
	return announcements.Announcement{ID: 13}, nil
}
func (service *serviceStub) Update(context.Context, authentication.VerifiedIdentity, announcements.UpdateCommand) (announcements.Announcement, error) {
	service.called = "update"
	return announcements.Announcement{ID: 12}, nil
}
func (service *serviceStub) Delete(context.Context, authentication.VerifiedIdentity, int64) error {
	service.called = "delete"
	return nil
}

type tokenVerifierStub struct{}

func (tokenVerifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "announcement.manager"}, nil
}

func newTestApp(service Service) *fiber.App {
	return httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(tokenVerifierStub{}, New(service)))
}

func authorizedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerRegistersAnnouncementCRUDAndOptions(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, test := range []struct {
		request *http.Request
		status  int
		action  string
	}{
		{authorizedRequest(http.MethodGet, "/api/v1/announcements?limit=10", ""), http.StatusOK, "list"},
		{authorizedRequest(http.MethodGet, "/api/v1/announcements/options", ""), http.StatusOK, "options"},
		{authorizedRequest(http.MethodGet, "/api/v1/announcements/12", ""), http.StatusOK, "get"},
		{authorizedRequest(http.MethodPost, "/api/v1/announcements", `{"is_active":true,"topic":"Hello","message":"World","language":"en","auto_publish":false}`), http.StatusCreated, "create"},
		{authorizedRequest(http.MethodPut, "/api/v1/announcements/12", `{"is_active":true,"topic":"Hello","message":"World","language":"en","auto_publish":false}`), http.StatusOK, "update"},
		{authorizedRequest(http.MethodDelete, "/api/v1/announcements/12", ""), http.StatusNoContent, "delete"},
		{authorizedRequest(http.MethodPost, "/api/v1/announcements", `{"topic":"Hello","unknown":true}`), http.StatusBadRequest, "delete"},
	} {
		response, err := app.Test(test.request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.status {
			t.Errorf("%s %s status = %d, want %d", test.request.Method, test.request.URL.Path, response.StatusCode, test.status)
		}
		if test.status != http.StatusBadRequest && service.called != test.action {
			t.Errorf("action = %q, want %q", service.called, test.action)
		}
		if response.Header.Get("Cache-Control") != "private, no-store" {
			t.Errorf("%s %s missing no-store", test.request.Method, test.request.URL.Path)
		}
	}
}

func TestControllerRequiresAuthentication(t *testing.T) {
	service := &serviceStub{}
	response, err := newTestApp(service).Test(httptest.NewRequest(http.MethodGet, "/api/v1/announcements", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.called != "" {
		t.Fatalf("unauthenticated status/call = %d/%q", response.StatusCode, service.called)
	}
}

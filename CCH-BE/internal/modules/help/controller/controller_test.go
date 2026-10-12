package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/help"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	page        help.Page
	article     help.Article
	options     help.LookupOptions
	listQuery   help.ListQuery
	create      help.CreateCommand
	update      help.UpdateCommand
	getID       int64
	deleteID    int64
	listCalls   int
	getCalls    int
	optionCalls int
	addCalls    int
	editCalls   int
	deleteCalls int
}

func (service *serviceStub) List(_ context.Context, _ authentication.VerifiedIdentity, query help.ListQuery) (help.Page, error) {
	service.listCalls++
	service.listQuery = query
	return service.page, nil
}
func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, id int64) (help.Article, error) {
	service.getCalls++
	service.getID = id
	return service.article, nil
}
func (service *serviceStub) LookupOptions(context.Context, authentication.VerifiedIdentity) (help.LookupOptions, error) {
	service.optionCalls++
	return service.options, nil
}
func (service *serviceStub) Create(_ context.Context, _ authentication.VerifiedIdentity, command help.CreateCommand) (help.Article, error) {
	service.addCalls++
	service.create = command
	return service.article, nil
}
func (service *serviceStub) Update(_ context.Context, _ authentication.VerifiedIdentity, command help.UpdateCommand) (help.Article, error) {
	service.editCalls++
	service.update = command
	return service.article, nil
}
func (service *serviceStub) Delete(_ context.Context, _ authentication.VerifiedIdentity, id int64) error {
	service.deleteCalls++
	service.deleteID = id
	return nil
}

type tokenVerifierStub struct{}

func (tokenVerifierStub) Verify(token string) (authentication.VerifiedIdentity, error) {
	if token != "valid-token" {
		return authentication.VerifiedIdentity{}, authentication.ErrInvalidToken
	}
	return authentication.VerifiedIdentity{Username: "help.manager"}, nil
}

func newTestApp(service Service) *fiber.App {
	return httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(tokenVerifierStub{}, New(service)))
}

func authorizedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerExposesHelpArticleCRUDAndLookupRoutes(t *testing.T) {
	article := help.Article{
		ArticleSummary: help.ArticleSummary{ID: 7, Language: "id", Topic: "Login", CategoryID: 3, Order: 1, DisplayInPage: "login"},
		Description:    "<p>Help text</p>",
	}
	service := &serviceStub{
		article: article,
		page:    help.Page{Data: []help.ArticleSummary{article.ArticleSummary}, Total: 1, Limit: 25, Offset: 0},
		options: help.LookupOptions{Languages: []help.LanguageOption{{Code: "id", Name: "Indonesia"}}},
	}
	app := newTestApp(service)
	body := `{"language":"id","topic":"Login","description":"Help text","category_id":3,"order":1,"display_in_page":"login"}`
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/help/articles?language=id&category_id=3&limit=10", ""), wantStatus: http.StatusOK},
		{name: "options", request: authorizedRequest(http.MethodGet, "/api/v1/help/articles/options", ""), wantStatus: http.StatusOK},
		{name: "get", request: authorizedRequest(http.MethodGet, "/api/v1/help/articles/7", ""), wantStatus: http.StatusOK},
		{name: "create", request: authorizedRequest(http.MethodPost, "/api/v1/help/articles", body), wantStatus: http.StatusCreated},
		{name: "update", request: authorizedRequest(http.MethodPut, "/api/v1/help/articles/7", body), wantStatus: http.StatusOK},
		{name: "delete", request: authorizedRequest(http.MethodDelete, "/api/v1/help/articles/7", ""), wantStatus: http.StatusNoContent},
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
	if service.listCalls != 1 || service.listQuery.Language != "id" || service.listQuery.CategoryID != 3 || service.listQuery.Limit != 10 ||
		service.optionCalls != 1 || service.getCalls != 1 || service.getID != 7 || service.addCalls != 1 || service.create.Topic != "Login" ||
		service.editCalls != 1 || service.update.ID != 7 || service.deleteCalls != 1 || service.deleteID != 7 {
		t.Fatalf("help article route calls did not match requests: %#v", service)
	}
}

func TestControllerRejectsUnknownFieldsAndRequiresAuthentication(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, request := range []*http.Request{
		authorizedRequest(http.MethodPost, "/api/v1/help/articles", `{"language":"id","topic":"Login","description":"Body","category_id":3,"order":1,"display_in_page":"login","updated_by":"spoofed"}`),
		authorizedRequest(http.MethodPut, "/api/v1/help/articles/0", `{"language":"id","topic":"Login","description":"Body","category_id":3,"order":1,"display_in_page":"login"}`),
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
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/help/articles", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.listCalls != 0 {
		t.Fatalf("unauthenticated status/calls = %d/%d, want 401/0", response.StatusCode, service.listCalls)
	}
}

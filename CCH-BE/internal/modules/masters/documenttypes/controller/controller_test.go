package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/masters/documenttypes"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	value      documenttypes.DocumentType
	values     []documenttypes.DocumentType
	create     documenttypes.CreateCommand
	update     documenttypes.UpdateCommand
	getCode    string
	deleteCode string
	called     string
}

func (service *serviceStub) List(context.Context, authentication.VerifiedIdentity) ([]documenttypes.DocumentType, error) {
	service.called = "list"
	return service.values, nil
}
func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, code string) (documenttypes.DocumentType, error) {
	service.called, service.getCode = "get", code
	return service.value, nil
}
func (service *serviceStub) Create(_ context.Context, _ authentication.VerifiedIdentity, command documenttypes.CreateCommand) (documenttypes.DocumentType, error) {
	service.called, service.create = "add", command
	return service.value, nil
}
func (service *serviceStub) Update(_ context.Context, _ authentication.VerifiedIdentity, command documenttypes.UpdateCommand) (documenttypes.DocumentType, error) {
	service.called, service.update = "edit", command
	return service.value, nil
}
func (service *serviceStub) Deactivate(_ context.Context, _ authentication.VerifiedIdentity, code string) error {
	service.called, service.deleteCode = "delete", code
	return nil
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
func authorizedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("X-Authorization", "Bearer valid-token")
	return request
}

func TestControllerOffersDocumentTypeCRUDAndStrictRequests(t *testing.T) {
	service := &serviceStub{value: documenttypes.DocumentType{Code: "PDF", Description: "Dokumen", Active: true}, values: []documenttypes.DocumentType{{Code: "PDF", Description: "Dokumen", Active: true}}}
	app := newTestApp(service)
	for _, test := range []struct {
		request *http.Request
		status  int
	}{
		{authorizedRequest(http.MethodGet, "/api/v1/masters/document-types", ""), http.StatusOK},
		{authorizedRequest(http.MethodGet, "/api/v1/masters/document-types/PDF", ""), http.StatusOK},
		{authorizedRequest(http.MethodPost, "/api/v1/masters/document-types", `{"code":"PDF","description":"Dokumen"}`), http.StatusCreated},
		{authorizedRequest(http.MethodPut, "/api/v1/masters/document-types/PDF", `{"description":"Dokumen baru","active":true}`), http.StatusOK},
		{authorizedRequest(http.MethodDelete, "/api/v1/masters/document-types/PDF", ""), http.StatusNoContent},
		{authorizedRequest(http.MethodPost, "/api/v1/masters/document-types", `{"code":"PDF","description":"Dokumen","unexpected":true}`), http.StatusBadRequest},
		{authorizedRequest(http.MethodPut, "/api/v1/masters/document-types/PDF", `{"description":"Dokumen"}`), http.StatusBadRequest},
		{authorizedRequest(http.MethodDelete, "/api/v1/masters/document-types/PD", ""), http.StatusBadRequest},
	} {
		response, err := app.Test(test.request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != test.status {
			t.Errorf("%s %s status = %d, want %d", test.request.Method, test.request.URL.Path, response.StatusCode, test.status)
		}
		if test.status != http.StatusBadRequest && response.Header.Get("Cache-Control") != "private, no-store" {
			t.Errorf("%s %s missing no-store", test.request.Method, test.request.URL.Path)
		}
	}
	if service.called != "delete" || service.deleteCode != "PDF" {
		t.Fatalf("delete dispatch = %#v", service)
	}
}

func TestControllerRequiresAuthentication(t *testing.T) {
	service := &serviceStub{}
	response, err := newTestApp(service).Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/document-types", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.called != "" {
		t.Fatalf("unauthenticated status/call = %d/%q, want 401/no call", response.StatusCode, service.called)
	}
}

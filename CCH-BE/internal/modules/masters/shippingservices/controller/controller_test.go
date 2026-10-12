package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/masters/shippingservices"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type serviceStub struct {
	value      shippingservices.ShippingService
	values     []shippingservices.ShippingService
	create     shippingservices.CreateCommand
	update     shippingservices.UpdateCommand
	getCode    string
	deleteCode string
	listCalls  int
	getCalls   int
	addCalls   int
	editCalls  int
	delCalls   int
	err        error
}

func (service *serviceStub) List(context.Context, authentication.VerifiedIdentity) ([]shippingservices.ShippingService, error) {
	service.listCalls++
	return service.values, service.err
}

func (service *serviceStub) Get(_ context.Context, _ authentication.VerifiedIdentity, code string) (shippingservices.ShippingService, error) {
	service.getCalls++
	service.getCode = code
	return service.value, service.err
}

func (service *serviceStub) Create(_ context.Context, _ authentication.VerifiedIdentity, command shippingservices.CreateCommand) (shippingservices.ShippingService, error) {
	service.addCalls++
	service.create = command
	return service.value, service.err
}

func (service *serviceStub) Update(_ context.Context, _ authentication.VerifiedIdentity, command shippingservices.UpdateCommand) (shippingservices.ShippingService, error) {
	service.editCalls++
	service.update = command
	return service.value, service.err
}

func (service *serviceStub) Deactivate(_ context.Context, _ authentication.VerifiedIdentity, code string) error {
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

func TestControllerProvidesShippingServiceCRUDRoutes(t *testing.T) {
	service := &serviceStub{value: shippingservices.ShippingService{Code: "REG", Description: "Reguler", TrackingURL: "https://track.example/{item}", Active: true}}
	app := newTestApp(service)
	for _, test := range []struct {
		name       string
		request    *http.Request
		wantStatus int
	}{
		{name: "list", request: authorizedRequest(http.MethodGet, "/api/v1/masters/shipping-services", ""), wantStatus: http.StatusOK},
		{name: "get", request: authorizedRequest(http.MethodGet, "/api/v1/masters/shipping-services/REG", ""), wantStatus: http.StatusOK},
		{name: "create", request: authorizedRequest(http.MethodPost, "/api/v1/masters/shipping-services", `{"code":"REG","description":"Reguler","tracking_url":"https://track.example/{item}"}`), wantStatus: http.StatusCreated},
		{name: "update", request: authorizedRequest(http.MethodPut, "/api/v1/masters/shipping-services/REG", `{"description":"Reguler Baru","tracking_url":"https://track.example/{item}","active":true}`), wantStatus: http.StatusOK},
		{name: "deactivate", request: authorizedRequest(http.MethodDelete, "/api/v1/masters/shipping-services/REG", ""), wantStatus: http.StatusNoContent},
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
				t.Fatalf("Cache-Control = %q", response.Header.Get("Cache-Control"))
			}
		})
	}
	if service.listCalls != 1 || service.getCalls != 1 || service.getCode != "REG" || service.addCalls != 1 || service.create.Code != "REG" || service.editCalls != 1 || service.update.Code != "REG" || service.deleteCode != "REG" || service.delCalls != 1 {
		t.Fatalf("shipping service CRUD calls unexpected: %#v", service)
	}
}

func TestControllerRejectsInvalidShippingServicePayloadAndUnauthenticatedRequest(t *testing.T) {
	service := &serviceStub{}
	app := newTestApp(service)
	for _, request := range []*http.Request{
		authorizedRequest(http.MethodPost, "/api/v1/masters/shipping-services", `{"code":"REG","tracking_url":"https://x","unknown":1}`),
		authorizedRequest(http.MethodPut, "/api/v1/masters/shipping-services/REG", `{"description":"Missing active","tracking_url":"https://track.example/{item}"}`),
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
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/masters/shipping-services", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || service.listCalls != 0 {
		t.Fatalf("unauthenticated status/calls = %d/%d, want 401/0", response.StatusCode, service.listCalls)
	}
}

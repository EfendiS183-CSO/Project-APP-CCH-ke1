package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/followups"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type cancelServiceStub struct {
	request followups.CancelPlanRequest
	err     error
}

func (service *cancelServiceStub) Cancel(_ context.Context, _ authentication.VerifiedIdentity, request followups.CancelPlanRequest) error {
	service.request = request
	return service.err
}

func TestCancelControllerReturnsNoContent(t *testing.T) {
	service := &cancelServiceStub{}
	app := httpapi.NewServer(nil, httpapi.AuthenticatedRouteModule(stubVerifier{identity: authentication.VerifiedIdentity{Username: "agent.test"}}, NewCancelController(service)))
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/complaints/call-center/TKT-001/follow-ups/FU1", nil)
	request.Header.Set("X-Authorization", "Bearer valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || service.request.ComplaintID != "TKT-001" || service.request.StatusCode != "FU1" {
		t.Fatalf("status/request = %d/%#v", response.StatusCode, service.request)
	}
}

func TestCancelControllerMapsRealizedConflict(t *testing.T) {
	service := &cancelServiceStub{err: followups.ErrPlanCancellationConflict}
	app := fiber.New(fiber.Config{ErrorHandler: func(ctx fiber.Ctx, err error) error { return ctx.SendStatus(fiberErrStatus(err)) }})
	app.Use(httpapi.JWTAuthentication(stubVerifier{identity: authentication.VerifiedIdentity{Username: "agent.test"}}))
	NewCancelController(service).Register(app)
	request := httptest.NewRequest(http.MethodDelete, "/complaints/call-center/TKT-001/follow-ups/FU1", nil)
	request.Header.Set("X-Authorization", "valid-token")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want conflict", response.StatusCode)
	}
}

func fiberErrStatus(err error) int {
	if fiberError, ok := err.(*fiber.Error); ok {
		return fiberError.Code
	}
	return http.StatusInternalServerError
}

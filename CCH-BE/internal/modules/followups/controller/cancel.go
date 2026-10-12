package controller

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/followups"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type CallCenterPlanCanceller interface {
	Cancel(context.Context, authentication.VerifiedIdentity, followups.CancelPlanRequest) error
}

type CancelController struct {
	service CallCenterPlanCanceller
}

func NewCancelController(service CallCenterPlanCanceller) *CancelController {
	return &CancelController{service: service}
}

func (controller *CancelController) Register(router fiber.Router) {
	router.Delete("/complaints/call-center/:id/follow-ups/:status", controller.Cancel)
}

func (controller *CancelController) Cancel(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "no-store")
	ctx.Set("Pragma", "no-cache")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	if err := controller.service.Cancel(ctx.Context(), identity, followups.CancelPlanRequest{ComplaintID: ctx.Params("id"), StatusCode: ctx.Params("status")}); err != nil {
		switch {
		case errors.Is(err, followups.ErrInvalidPlanRequest):
			return fiber.ErrBadRequest
		case errors.Is(err, followups.ErrPlanCancellationNotFound):
			return fiber.ErrNotFound
		case errors.Is(err, followups.ErrPlanCancellationConflict):
			return fiber.ErrConflict
		case errors.Is(err, followups.ErrCallCenterRoleRequired), errors.Is(err, authorization.ErrPermissionDenied),
			errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
			return fiber.ErrForbidden
		default:
			return httpapi.AuthorizationHTTPError(err)
		}
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

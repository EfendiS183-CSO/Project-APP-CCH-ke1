package controller

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/complaints"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type CallCenterComplaintLookupGetter interface {
	Get(context.Context, authentication.VerifiedIdentity) (complaints.CallCenterComplaintLookups, error)
}

type LookupController struct {
	service CallCenterComplaintLookupGetter
}

func NewLookupController(service CallCenterComplaintLookupGetter) *LookupController {
	return &LookupController{service: service}
}

func (controller *LookupController) Register(router fiber.Router) {
	router.Get("/complaints/call-center/lookups", controller.Get)
}

func (controller *LookupController) Get(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	lookups, err := controller.service.Get(ctx.Context(), identity)
	if err != nil {
		switch {
		case errors.Is(err, authorization.ErrPermissionDenied),
			errors.Is(err, authorization.ErrSystemAdministratorUnsupported),
			errors.Is(err, authorization.ErrInvalidUserLevels),
			errors.Is(err, complaints.ErrLookupCallCenterRoleRequired),
			errors.Is(err, complaints.ErrLookupOfficeRequired):
			return fiber.ErrForbidden
		default:
			return httpapi.AuthorizationHTTPError(err)
		}
	}
	return ctx.JSON(fiber.Map{"data": lookups})
}

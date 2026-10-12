package httpapi

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
)

const authorizedAccountAccessLocal = "authorizedAccountAccess"

func RequireTableAction(service *authorization.AccessService, tableVariable string, action authorization.Action) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		if service == nil {
			return fiber.ErrServiceUnavailable
		}
		identity, ok := VerifiedIdentityFromContext(ctx)
		if !ok {
			return fiber.ErrUnauthorized
		}

		access, err := service.Authorize(ctx.Context(), identity, tableVariable, action)
		if err != nil {
			return AuthorizationHTTPError(err)
		}
		ctx.Locals(authorizedAccountAccessLocal, access)
		return ctx.Next()
	}
}

func AuthorizedAccountAccessFromContext(ctx fiber.Ctx) (authorization.AccountAccess, bool) {
	access, ok := ctx.Locals(authorizedAccountAccessLocal).(authorization.AccountAccess)
	return access, ok
}

func AuthorizationHTTPError(err error) error {
	switch {
	case errors.Is(err, authorization.ErrAccountNotFound),
		errors.Is(err, authorization.ErrAccountInactive),
		errors.Is(err, authorization.ErrAccountLocked),
		errors.Is(err, authorization.ErrIdentityMismatch):
		return fiber.ErrUnauthorized
	case errors.Is(err, authorization.ErrPermissionDenied),
		errors.Is(err, authorization.ErrSystemAdministratorUnsupported),
		errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	case errors.Is(err, authorization.ErrInvalidAction),
		errors.Is(err, authorization.ErrInvalidTableVariable):
		return fiber.ErrInternalServerError
	case errors.Is(err, authorization.ErrAccessServiceNotConfigured):
		return fiber.ErrServiceUnavailable
	default:
		return fiber.ErrServiceUnavailable
	}
}

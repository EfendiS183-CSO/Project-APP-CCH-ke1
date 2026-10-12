package controller

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/userlevels"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type Service interface {
	List(context.Context, authentication.VerifiedIdentity, userlevels.ListQuery) (userlevels.Page, error)
	Get(context.Context, authentication.VerifiedIdentity, int64) (userlevels.UserLevel, error)
}

type Controller struct{ service Service }

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/admin/user-levels", controller.List)
	router.Get("/admin/user-levels/:id", controller.Get)
}

func (controller *Controller) List(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	query := userlevels.ListQuery{}
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		query.Limit, err = strconv.Atoi(raw)
		if err != nil {
			return fiber.ErrBadRequest
		}
	}
	if raw := strings.TrimSpace(ctx.Query("offset")); raw != "" {
		query.Offset, err = strconv.Atoi(raw)
		if err != nil {
			return fiber.ErrBadRequest
		}
	}
	page, err := controller.service.List(ctx.Context(), identity, query)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(page)
}

func (controller *Controller) Get(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(strings.TrimSpace(ctx.Params("id")), 10, 64)
	if err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Get(ctx.Context(), identity, id)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(value)
}

func (controller *Controller) identity(ctx fiber.Ctx) (authentication.VerifiedIdentity, error) {
	if controller == nil || controller.service == nil {
		return authentication.VerifiedIdentity{}, fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "private, no-store")
	ctx.Set("Pragma", "no-cache")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return authentication.VerifiedIdentity{}, fiber.ErrUnauthorized
	}
	return identity, nil
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, userlevels.ErrInvalidInput):
		return fiber.ErrBadRequest
	case errors.Is(err, userlevels.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

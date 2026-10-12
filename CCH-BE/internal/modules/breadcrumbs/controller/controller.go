package controller

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/breadcrumbs"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type Service interface {
	List(context.Context, authentication.VerifiedIdentity, breadcrumbs.ListQuery) (breadcrumbs.Page, error)
	Get(context.Context, authentication.VerifiedIdentity, string) (breadcrumbs.Breadcrumb, error)
	Check(context.Context, authentication.VerifiedIdentity, string) ([]breadcrumbs.Ancestor, error)
	Add(context.Context, authentication.VerifiedIdentity, string, string, string) (breadcrumbs.Breadcrumb, error)
	Move(context.Context, authentication.VerifiedIdentity, string, string) error
	Delete(context.Context, authentication.VerifiedIdentity, string) error
}

type Controller struct{ service Service }

func New(service Service) *Controller {
	return &Controller{service: service}
}

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/admin/breadcrumbs", controller.List)
	router.Get("/admin/breadcrumbs/:title", controller.Get)
	router.Get("/admin/breadcrumbs/:title/ancestors", controller.Check)
	router.Post("/admin/breadcrumbs", controller.Add)
	router.Put("/admin/breadcrumbs/:title/move", controller.Move)
	router.Delete("/admin/breadcrumbs/:title", controller.Delete)
}

func (controller *Controller) List(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	limit, offset, err := parsePagination(ctx)
	if err != nil {
		return fiber.ErrBadRequest
	}
	page, err := controller.service.List(ctx.Context(), identity, breadcrumbs.ListQuery{Limit: limit, Offset: offset})
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
	title := strings.TrimSpace(ctx.Params("title"))
	value, err := controller.service.Get(ctx.Context(), identity, title)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(value)
}

func (controller *Controller) Check(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	title := strings.TrimSpace(ctx.Params("title"))
	ancestors, err := controller.service.Check(ctx.Context(), identity, title)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(map[string][]breadcrumbs.Ancestor{"ancestors": ancestors})
}

type addRequest struct {
	ParentTitle string `json:"parent_title"`
	PageTitle   string `json:"page_title"`
	PageURL     string `json:"page_url"`
}

type moveRequest struct {
	NewRoot string `json:"new_root"`
}

func (controller *Controller) Add(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	var request addRequest
	if err := ctx.Bind().JSON(&request); err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Add(ctx.Context(), identity, strings.TrimSpace(request.ParentTitle), strings.TrimSpace(request.PageTitle), strings.TrimSpace(request.PageURL))
	if err != nil {
		return controllerError(err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(value)
}

func (controller *Controller) Move(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	title := strings.TrimSpace(ctx.Params("title"))
	var request moveRequest
	if err := ctx.Bind().JSON(&request); err != nil {
		return fiber.ErrBadRequest
	}
	if err := controller.service.Move(ctx.Context(), identity, title, strings.TrimSpace(request.NewRoot)); err != nil {
		return controllerError(err)
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

func (controller *Controller) Delete(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	title := strings.TrimSpace(ctx.Params("title"))
	if err := controller.service.Delete(ctx.Context(), identity, title); err != nil {
		return controllerError(err)
	}
	return ctx.SendStatus(fiber.StatusNoContent)
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
	case errors.Is(err, breadcrumbs.ErrInvalidInput), errors.Is(err, breadcrumbs.ErrDuplicateTitle), errors.Is(err, breadcrumbs.ErrInvalidParent), errors.Is(err, breadcrumbs.ErrInvalidRoot), errors.Is(err, breadcrumbs.ErrSameRoot), errors.Is(err, breadcrumbs.ErrNoData):
		return fiber.ErrConflict
	case errors.Is(err, breadcrumbs.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

func parsePagination(ctx fiber.Ctx) (int, int, error) {
	var limit, offset int
	var err error
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		limit, err = parsePositiveInt(raw, 25)
		if err != nil {
			return 0, 0, err
		}
	}
	if raw := strings.TrimSpace(ctx.Query("offset")); raw != "" {
		offset, err = parseNonNegativeInt(raw)
		if err != nil {
			return 0, 0, err
		}
	}
	return limit, offset, nil
}

func parsePositiveInt(raw string, fallback int) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fiber.ErrBadRequest
	}
	return value, nil
}

func parseNonNegativeInt(raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, fiber.ErrBadRequest
	}
	return value, nil
}

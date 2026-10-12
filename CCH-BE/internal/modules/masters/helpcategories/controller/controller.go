package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/masters/helpcategories"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 8 * 1024

type Service interface {
	List(context.Context, authentication.VerifiedIdentity) ([]helpcategories.Category, error)
	Get(context.Context, authentication.VerifiedIdentity, int64) (helpcategories.Category, error)
	Languages(context.Context, authentication.VerifiedIdentity) ([]helpcategories.Language, error)
	Create(context.Context, authentication.VerifiedIdentity, helpcategories.CreateCommand) (helpcategories.Category, error)
	Update(context.Context, authentication.VerifiedIdentity, helpcategories.UpdateCommand) (helpcategories.Category, error)
	Delete(context.Context, authentication.VerifiedIdentity, int64) error
}

type Controller struct{ service Service }

type categoryRequest struct {
	Language    string `json:"language"`
	Description string `json:"description"`
}

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/masters/help-categories", controller.List)
	router.Get("/masters/help-categories/languages", controller.Languages)
	router.Post("/masters/help-categories", controller.Create)
	router.Get("/masters/help-categories/:id", controller.Get)
	router.Put("/masters/help-categories/:id", controller.Update)
	router.Delete("/masters/help-categories/:id", controller.Delete)
}

func (controller *Controller) List(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	values, err := controller.service.List(ctx.Context(), identity)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(values)
}

func (controller *Controller) Languages(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	values, err := controller.service.Languages(ctx.Context(), identity)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(values)
}

func (controller *Controller) Get(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	id, err := parseID(ctx.Params("id"))
	if err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Get(ctx.Context(), identity, id)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(value)
}

func (controller *Controller) Create(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	var request categoryRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Create(ctx.Context(), identity, helpcategories.CreateCommand{Language: request.Language, Description: request.Description})
	if err != nil {
		return controllerError(err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(value)
}

func (controller *Controller) Update(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	id, err := parseID(ctx.Params("id"))
	if err != nil {
		return fiber.ErrBadRequest
	}
	var request categoryRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Update(ctx.Context(), identity, helpcategories.UpdateCommand{ID: id, Language: request.Language, Description: request.Description})
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(value)
}

func (controller *Controller) Delete(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	id, err := parseID(ctx.Params("id"))
	if err != nil {
		return fiber.ErrBadRequest
	}
	if err := controller.service.Delete(ctx.Context(), identity, id); err != nil {
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

func decodeStrict(ctx fiber.Ctx, target any) error {
	body := ctx.Body()
	if len(body) == 0 || len(body) > maxRequestBodyBytes {
		return helpcategories.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return helpcategories.ErrInvalidInput
	}
	var trailing any
	if !errors.Is(decoder.Decode(&trailing), io.EOF) {
		return helpcategories.ErrInvalidInput
	}
	return nil
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id < 1 {
		return 0, helpcategories.ErrInvalidInput
	}
	return id, nil
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, helpcategories.ErrInvalidInput), errors.Is(err, helpcategories.ErrInvalidLanguage):
		return fiber.ErrBadRequest
	case errors.Is(err, helpcategories.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, helpcategories.ErrInUse):
		return fiber.NewError(fiber.StatusConflict, "help category is referenced by help entries")
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

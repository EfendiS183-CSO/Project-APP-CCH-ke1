package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/masters/handlingstatuses"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 8 * 1024

type Service interface {
	List(context.Context, authentication.VerifiedIdentity) ([]handlingstatuses.Status, error)
	Get(context.Context, authentication.VerifiedIdentity, string) (handlingstatuses.Status, error)
	Create(context.Context, authentication.VerifiedIdentity, handlingstatuses.CreateCommand) (handlingstatuses.Status, error)
	Update(context.Context, authentication.VerifiedIdentity, handlingstatuses.UpdateCommand) (handlingstatuses.Status, error)
	Deactivate(context.Context, authentication.VerifiedIdentity, string) error
}

type Controller struct{ service Service }

type createRequest struct {
	Code        string `json:"status_code"`
	Description string `json:"description"`
	Active      *bool  `json:"active"`
}

type updateRequest struct {
	Description string `json:"description"`
	Active      *bool  `json:"active"`
}

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/masters/handling-statuses", controller.List)
	router.Post("/masters/handling-statuses", controller.Create)
	router.Get("/masters/handling-statuses/:code", controller.Get)
	router.Put("/masters/handling-statuses/:code", controller.Update)
	router.Delete("/masters/handling-statuses/:code", controller.Deactivate)
}

func (controller *Controller) List(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	setNoStore(ctx)
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	values, err := controller.service.List(ctx.Context(), identity)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(values)
}

func (controller *Controller) Get(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	setNoStore(ctx)
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	value, err := controller.service.Get(ctx.Context(), identity, strings.TrimSpace(ctx.Params("code")))
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(value)
}

func (controller *Controller) Create(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	setNoStore(ctx)
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	var request createRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	active := true
	if request.Active != nil {
		active = *request.Active
	}
	value, err := controller.service.Create(ctx.Context(), identity, handlingstatuses.CreateCommand{Code: request.Code, Description: request.Description, Active: active})
	if err != nil {
		return controllerError(err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(value)
}

func (controller *Controller) Update(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	setNoStore(ctx)
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	var request updateRequest
	if err := decodeStrict(ctx, &request); err != nil || request.Active == nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Update(ctx.Context(), identity, handlingstatuses.UpdateCommand{Code: strings.TrimSpace(ctx.Params("code")), Description: request.Description, Active: *request.Active})
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(value)
}

func (controller *Controller) Deactivate(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	setNoStore(ctx)
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	code := strings.TrimSpace(ctx.Params("code"))
	if !utf8.ValidString(code) || utf8.RuneCountInString(code) != 3 {
		return fiber.ErrBadRequest
	}
	if err := controller.service.Deactivate(ctx.Context(), identity, code); err != nil {
		return controllerError(err)
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

func decodeStrict(ctx fiber.Ctx, target any) error {
	body := ctx.Body()
	if len(body) == 0 || len(body) > maxRequestBodyBytes {
		return handlingstatuses.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return handlingstatuses.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return handlingstatuses.ErrInvalidInput
	}
	return nil
}

func setNoStore(ctx fiber.Ctx) {
	ctx.Set("Cache-Control", "private, no-store")
	ctx.Set("Pragma", "no-cache")
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, handlingstatuses.ErrInvalidInput):
		return fiber.ErrBadRequest
	case errors.Is(err, handlingstatuses.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, handlingstatuses.ErrAlreadyExists):
		return fiber.NewError(fiber.StatusConflict, "handling status code already exists")
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

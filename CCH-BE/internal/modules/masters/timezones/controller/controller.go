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
	"cch.local/backend/internal/modules/masters/timezones"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 8 * 1024

type Service interface {
	List(context.Context, authentication.VerifiedIdentity) ([]timezones.Timezone, error)
	Get(context.Context, authentication.VerifiedIdentity, int64) (timezones.Timezone, error)
	Create(context.Context, authentication.VerifiedIdentity, timezones.CreateCommand) (timezones.Timezone, error)
	Update(context.Context, authentication.VerifiedIdentity, timezones.UpdateCommand) (timezones.Timezone, error)
	Delete(context.Context, authentication.VerifiedIdentity, int64) error
}

type Controller struct{ service Service }

type createRequest struct {
	Timezone  string `json:"timezone"`
	IsDefault bool   `json:"is_default"`
}

type updateRequest struct {
	Timezone  string `json:"timezone"`
	IsDefault bool   `json:"is_default"`
}

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/masters/timezones", controller.List)
	router.Post("/masters/timezones", controller.Create)
	router.Get("/masters/timezones/:id", controller.Get)
	router.Put("/masters/timezones/:id", controller.Update)
	router.Delete("/masters/timezones/:id", controller.Delete)
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

func (controller *Controller) Create(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	var request createRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Create(ctx.Context(), identity, timezones.CreateCommand{Timezone: request.Timezone, IsDefault: request.IsDefault})
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
	id, err := strconv.ParseInt(strings.TrimSpace(ctx.Params("id")), 10, 64)
	if err != nil {
		return fiber.ErrBadRequest
	}
	var request updateRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Update(ctx.Context(), identity, timezones.UpdateCommand{ID: id, Timezone: request.Timezone, IsDefault: request.IsDefault})
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
	id, err := strconv.ParseInt(strings.TrimSpace(ctx.Params("id")), 10, 64)
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
		return timezones.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return timezones.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return timezones.ErrInvalidInput
	}
	return nil
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, timezones.ErrInvalidInput), errors.Is(err, timezones.ErrCannotDeleteDefault):
		return fiber.ErrBadRequest
	case errors.Is(err, timezones.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, timezones.ErrDuplicate), errors.Is(err, timezones.ErrOnlyOneDefault):
		return fiber.NewError(fiber.StatusConflict, "timezone conflict")
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

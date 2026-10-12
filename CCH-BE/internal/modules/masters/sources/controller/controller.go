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
	"cch.local/backend/internal/modules/masters/sources"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 8 * 1024

type Service interface {
	List(context.Context, authentication.VerifiedIdentity) ([]sources.Source, error)
	Get(context.Context, authentication.VerifiedIdentity, int64) (sources.Source, error)
	Create(context.Context, authentication.VerifiedIdentity, sources.CreateCommand) (sources.Source, error)
	Update(context.Context, authentication.VerifiedIdentity, sources.UpdateCommand) (sources.Source, error)
	Deactivate(context.Context, authentication.VerifiedIdentity, int64) error
}

type Controller struct {
	service Service
}

type createRequest struct {
	Name   string `json:"name"`
	Active *bool  `json:"active"`
}

type updateRequest struct {
	Name   string `json:"name"`
	Active *bool  `json:"active"`
}

func New(service Service) *Controller {
	return &Controller{service: service}
}

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/masters/complaint-sources", controller.List)
	router.Post("/masters/complaint-sources", controller.Create)
	router.Get("/masters/complaint-sources/:id", controller.Get)
	router.Put("/masters/complaint-sources/:id", controller.Update)
	router.Delete("/masters/complaint-sources/:id", controller.Deactivate)
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
	value, err := controller.service.Create(ctx.Context(), identity, sources.CreateCommand{Name: request.Name, Active: active})
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
	id, err := parseID(ctx.Params("id"))
	if err != nil {
		return fiber.ErrBadRequest
	}
	var request updateRequest
	if err := decodeStrict(ctx, &request); err != nil || request.Active == nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Update(ctx.Context(), identity, sources.UpdateCommand{ID: id, Name: request.Name, Active: *request.Active})
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
	id, err := parseID(ctx.Params("id"))
	if err != nil {
		return fiber.ErrBadRequest
	}
	if err := controller.service.Deactivate(ctx.Context(), identity, id); err != nil {
		return controllerError(err)
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

func decodeStrict(ctx fiber.Ctx, target any) error {
	body := ctx.Body()
	if len(body) == 0 || len(body) > maxRequestBodyBytes {
		return sources.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return sources.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return sources.ErrInvalidInput
	}
	return nil
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id < 1 {
		return 0, sources.ErrInvalidInput
	}
	return id, nil
}

func setNoStore(ctx fiber.Ctx) {
	ctx.Set("Cache-Control", "private, no-store")
	ctx.Set("Pragma", "no-cache")
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, sources.ErrInvalidInput):
		return fiber.ErrBadRequest
	case errors.Is(err, sources.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, authorization.ErrPermissionDenied),
		errors.Is(err, authorization.ErrSystemAdministratorUnsupported),
		errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

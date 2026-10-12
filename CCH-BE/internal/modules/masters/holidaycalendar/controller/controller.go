package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/masters/holidaycalendar"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 8 * 1024

type Service interface {
	List(context.Context, authentication.VerifiedIdentity) ([]holidaycalendar.Holiday, error)
	Get(context.Context, authentication.VerifiedIdentity, string) (holidaycalendar.Holiday, error)
	Create(context.Context, authentication.VerifiedIdentity, holidaycalendar.CreateCommand) (holidaycalendar.Holiday, error)
	Update(context.Context, authentication.VerifiedIdentity, holidaycalendar.UpdateCommand) (holidaycalendar.Holiday, error)
	Delete(context.Context, authentication.VerifiedIdentity, string) error
}

type Controller struct{ service Service }

type createRequest struct {
	Date        string `json:"date"`
	Description string `json:"description"`
}

type updateRequest struct {
	Description string `json:"description"`
}

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/masters/holiday-calendar", controller.List)
	router.Post("/masters/holiday-calendar", controller.Create)
	router.Get("/masters/holiday-calendar/:date", controller.Get)
	router.Put("/masters/holiday-calendar/:date", controller.Update)
	router.Delete("/masters/holiday-calendar/:date", controller.Delete)
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
	value, err := controller.service.Get(ctx.Context(), identity, strings.TrimSpace(ctx.Params("date")))
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
	value, err := controller.service.Create(ctx.Context(), identity, holidaycalendar.CreateCommand{Date: request.Date, Description: request.Description})
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
	var request updateRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Update(ctx.Context(), identity, holidaycalendar.UpdateCommand{Date: strings.TrimSpace(ctx.Params("date")), Description: request.Description})
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
	if err := controller.service.Delete(ctx.Context(), identity, strings.TrimSpace(ctx.Params("date"))); err != nil {
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
		return holidaycalendar.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return holidaycalendar.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return holidaycalendar.ErrInvalidInput
	}
	return nil
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, holidaycalendar.ErrInvalidInput):
		return fiber.ErrBadRequest
	case errors.Is(err, holidaycalendar.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, holidaycalendar.ErrAlreadyExists):
		return fiber.NewError(fiber.StatusConflict, "holiday calendar date already exists")
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

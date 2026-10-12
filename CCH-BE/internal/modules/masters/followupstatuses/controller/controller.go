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
	"cch.local/backend/internal/modules/masters/followupstatuses"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 8 * 1024

type Service interface {
	List(context.Context, authentication.VerifiedIdentity) ([]followupstatuses.Status, error)
	LookupOptions(context.Context, authentication.VerifiedIdentity) ([]followupstatuses.Status, error)
	Get(context.Context, authentication.VerifiedIdentity, string) (followupstatuses.Status, error)
	Create(context.Context, authentication.VerifiedIdentity, followupstatuses.CreateCommand) (followupstatuses.Status, error)
	Update(context.Context, authentication.VerifiedIdentity, followupstatuses.UpdateCommand) (followupstatuses.Status, error)
	Delete(context.Context, authentication.VerifiedIdentity, string) error
}

type Controller struct{ service Service }

type createRequest struct {
	Code        string  `json:"code"`
	Description *string `json:"description"`
}

type updateRequest struct {
	Description json.RawMessage `json:"description"`
}

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/masters/follow-up-statuses", controller.List)
	router.Get("/masters/follow-up-statuses/options", controller.LookupOptions)
	router.Post("/masters/follow-up-statuses", controller.Create)
	router.Get("/masters/follow-up-statuses/:code", controller.Get)
	router.Put("/masters/follow-up-statuses/:code", controller.Update)
	router.Delete("/masters/follow-up-statuses/:code", controller.Delete)
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

func (controller *Controller) LookupOptions(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	values, err := controller.service.LookupOptions(ctx.Context(), identity)
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
	value, err := controller.service.Get(ctx.Context(), identity, strings.TrimSpace(ctx.Params("code")))
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
	value, err := controller.service.Create(ctx.Context(), identity, followupstatuses.CreateCommand{Code: request.Code, Description: request.Description})
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
	if err := decodeStrict(ctx, &request); err != nil || len(request.Description) == 0 {
		return fiber.ErrBadRequest
	}
	var description *string
	if !bytes.Equal(bytes.TrimSpace(request.Description), []byte("null")) {
		var value string
		if err := json.Unmarshal(request.Description, &value); err != nil {
			return fiber.ErrBadRequest
		}
		description = &value
	}
	value, err := controller.service.Update(ctx.Context(), identity, followupstatuses.UpdateCommand{Code: strings.TrimSpace(ctx.Params("code")), Description: description})
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
	code := strings.TrimSpace(ctx.Params("code"))
	if !utf8.ValidString(code) || len(code) != 3 {
		return fiber.ErrBadRequest
	}
	if err := controller.service.Delete(ctx.Context(), identity, code); err != nil {
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
		return followupstatuses.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return followupstatuses.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return followupstatuses.ErrInvalidInput
	}
	return nil
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, followupstatuses.ErrInvalidInput):
		return fiber.ErrBadRequest
	case errors.Is(err, followupstatuses.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, followupstatuses.ErrAlreadyExists):
		return fiber.NewError(fiber.StatusConflict, "follow-up status code already exists")
	case errors.Is(err, followupstatuses.ErrInUse):
		return fiber.NewError(fiber.StatusConflict, "follow-up status is referenced by existing follow-up records")
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

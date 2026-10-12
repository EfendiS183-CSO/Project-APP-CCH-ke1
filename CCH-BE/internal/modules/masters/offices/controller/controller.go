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
	"cch.local/backend/internal/modules/masters/offices"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 8 * 1024

type Service interface {
	List(context.Context, authentication.VerifiedIdentity) ([]offices.Office, error)
	Get(context.Context, authentication.VerifiedIdentity, string) (offices.Office, error)
	LookupOptions(context.Context, authentication.VerifiedIdentity) (offices.LookupOptions, error)
	Create(context.Context, authentication.VerifiedIdentity, offices.CreateCommand) (offices.Office, error)
	Update(context.Context, authentication.VerifiedIdentity, offices.UpdateCommand) (offices.Office, error)
	Deactivate(context.Context, authentication.VerifiedIdentity, string) error
}

type Controller struct{ service Service }

type createRequest struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Abbreviation  string `json:"abbreviation"`
	Type          string `json:"type"`
	RegionalCode  string `json:"regional_code"`
	Active        *bool  `json:"active"`
	AsDestination *bool  `json:"as_destination"`
	AsCopy        *bool  `json:"as_copy"`
}

type updateRequest struct {
	Name          string `json:"name"`
	Abbreviation  string `json:"abbreviation"`
	Type          string `json:"type"`
	RegionalCode  string `json:"regional_code"`
	Active        *bool  `json:"active"`
	AsDestination *bool  `json:"as_destination"`
	AsCopy        *bool  `json:"as_copy"`
}

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/masters/offices", controller.List)
	router.Post("/masters/offices", controller.Create)
	router.Get("/masters/offices/options", controller.LookupOptions)
	router.Get("/masters/offices/:code", controller.Get)
	router.Put("/masters/offices/:code", controller.Update)
	router.Delete("/masters/offices/:code", controller.Deactivate)
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
	value, err := controller.service.Get(ctx.Context(), identity, strings.TrimSpace(ctx.Params("code")))
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(value)
}

func (controller *Controller) LookupOptions(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	options, err := controller.service.LookupOptions(ctx.Context(), identity)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(options)
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
	active := defaultTrue(request.Active)
	asDestination := defaultTrue(request.AsDestination)
	asCopy := defaultTrue(request.AsCopy)
	value, err := controller.service.Create(ctx.Context(), identity, offices.CreateCommand{
		Code: request.Code, Name: request.Name, Abbreviation: request.Abbreviation, Type: request.Type,
		RegionalCode: request.RegionalCode, Active: active, AsDestination: asDestination, AsCopy: asCopy,
	})
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
	if err := decodeStrict(ctx, &request); err != nil || request.Active == nil || request.AsDestination == nil || request.AsCopy == nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Update(ctx.Context(), identity, offices.UpdateCommand{
		Code: strings.TrimSpace(ctx.Params("code")), Name: request.Name, Abbreviation: request.Abbreviation,
		Type: request.Type, RegionalCode: request.RegionalCode, Active: *request.Active,
		AsDestination: *request.AsDestination, AsCopy: *request.AsCopy,
	})
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(value)
}

func (controller *Controller) Deactivate(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	code := strings.TrimSpace(ctx.Params("code"))
	if !utf8.ValidString(code) || len(code) == 0 || len(code) > 7 {
		return fiber.ErrBadRequest
	}
	if err := controller.service.Deactivate(ctx.Context(), identity, code); err != nil {
		return controllerError(err)
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

func (controller *Controller) identity(ctx fiber.Ctx) (authentication.VerifiedIdentity, error) {
	if controller == nil || controller.service == nil {
		return authentication.VerifiedIdentity{}, fiber.ErrServiceUnavailable
	}
	setNoStore(ctx)
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return authentication.VerifiedIdentity{}, fiber.ErrUnauthorized
	}
	return identity, nil
}

func decodeStrict(ctx fiber.Ctx, target any) error {
	body := ctx.Body()
	if len(body) == 0 || len(body) > maxRequestBodyBytes {
		return offices.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return offices.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return offices.ErrInvalidInput
	}
	return nil
}

func defaultTrue(value *bool) bool { return value == nil || *value }

func setNoStore(ctx fiber.Ctx) {
	ctx.Set("Cache-Control", "private, no-store")
	ctx.Set("Pragma", "no-cache")
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, offices.ErrInvalidInput), errors.Is(err, offices.ErrInvalidType), errors.Is(err, offices.ErrInvalidRegional):
		return fiber.ErrBadRequest
	case errors.Is(err, offices.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, offices.ErrAlreadyExists):
		return fiber.NewError(fiber.StatusConflict, "office code already exists")
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

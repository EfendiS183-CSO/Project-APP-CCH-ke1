package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/regionals"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 8 * 1024

type Service interface {
	List(context.Context, authentication.VerifiedIdentity, regionals.ListQuery) (regionals.Page, error)
	Get(context.Context, authentication.VerifiedIdentity, string) (regionals.Office, error)
	LookupOptions(context.Context, authentication.VerifiedIdentity) ([]regionals.OfficeOption, error)
	Update(context.Context, authentication.VerifiedIdentity, regionals.UpdateCommand) (regionals.Office, error)
}

type Controller struct{ service Service }

type updateRequest struct {
	Name         string `json:"office_name"`
	Abbreviation string `json:"abbreviation"`
	Type         string `json:"type"`
	RegionalCode string `json:"regional_code"`
	Active       *bool  `json:"active"`
}

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/admin/regional-offices", controller.List)
	router.Get("/admin/regional-offices/options", controller.LookupOptions)
	router.Get("/admin/regional-offices/:code", controller.Get)
	router.Put("/admin/regional-offices/:code", controller.Update)
}

func (controller *Controller) List(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	query := regionals.ListQuery{}
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
	code := strings.TrimSpace(ctx.Params("code"))
	if !utf8.ValidString(code) || len(code) == 0 || len(code) > 7 {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Get(ctx.Context(), identity, code)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(value)
}

func (controller *Controller) Update(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	code := strings.TrimSpace(ctx.Params("code"))
	if !utf8.ValidString(code) || len(code) == 0 || len(code) > 7 {
		return fiber.ErrBadRequest
	}
	var request updateRequest
	if err := decodeStrict(ctx, &request); err != nil || request.Active == nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Update(ctx.Context(), identity, regionals.UpdateCommand{
		Code: code, Name: request.Name, Abbreviation: request.Abbreviation,
		Type: request.Type, RegionalCode: request.RegionalCode, Active: *request.Active,
	})
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

func decodeStrict(ctx fiber.Ctx, target any) error {
	body := ctx.Body()
	if len(body) == 0 || len(body) > maxRequestBodyBytes {
		return regionals.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return regionals.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return regionals.ErrInvalidInput
	}
	return nil
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, regionals.ErrInvalidInput):
		return fiber.ErrBadRequest
	case errors.Is(err, regionals.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, regionals.ErrInvalidRegional):
		return fiber.NewError(fiber.StatusConflict, "regional office assignment is invalid")
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

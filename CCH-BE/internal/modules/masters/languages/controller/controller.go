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
	"cch.local/backend/internal/modules/masters/languages"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 64 * 1024

type Service interface {
	List(context.Context, authentication.VerifiedIdentity) ([]languages.Language, error)
	Get(context.Context, authentication.VerifiedIdentity, string) (languages.Language, error)
	Create(context.Context, authentication.VerifiedIdentity, languages.CreateCommand) (languages.Language, error)
	Update(context.Context, authentication.VerifiedIdentity, languages.UpdateCommand) (languages.Language, error)
	Delete(context.Context, authentication.VerifiedIdentity, string) error
}

type Controller struct{ service Service }

type createRequest struct {
	Code                      string  `json:"language_code"`
	Name                      string  `json:"language_name"`
	IsDefault                 bool    `json:"is_default"`
	SiteLogo                  string  `json:"site_logo"`
	SiteTitle                 string  `json:"site_title"`
	DefaultThousandsSeparator *string `json:"default_thousands_separator,omitempty"`
	DefaultDecimalPoint       *string `json:"default_decimal_point,omitempty"`
	DefaultCurrencySymbol     *string `json:"default_currency_symbol,omitempty"`
	DefaultMoneyThousandsSep  *string `json:"default_money_thousands_separator,omitempty"`
	DefaultMoneyDecimalPoint  *string `json:"default_money_decimal_point,omitempty"`
	TermsAndConditionText     string  `json:"terms_and_condition_text"`
	AnnouncementText          string  `json:"announcement_text"`
	AboutText                 string  `json:"about_text"`
}

type updateRequest struct {
	Name                      string  `json:"language_name"`
	IsDefault                 bool    `json:"is_default"`
	SiteLogo                  string  `json:"site_logo"`
	SiteTitle                 string  `json:"site_title"`
	DefaultThousandsSeparator *string `json:"default_thousands_separator,omitempty"`
	DefaultDecimalPoint       *string `json:"default_decimal_point,omitempty"`
	DefaultCurrencySymbol     *string `json:"default_currency_symbol,omitempty"`
	DefaultMoneyThousandsSep  *string `json:"default_money_thousands_separator,omitempty"`
	DefaultMoneyDecimalPoint  *string `json:"default_money_decimal_point,omitempty"`
	TermsAndConditionText     string  `json:"terms_and_condition_text"`
	AnnouncementText          string  `json:"announcement_text"`
	AboutText                 string  `json:"about_text"`
}

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/masters/languages", controller.List)
	router.Post("/masters/languages", controller.Create)
	router.Get("/masters/languages/:code", controller.Get)
	router.Put("/masters/languages/:code", controller.Update)
	router.Delete("/masters/languages/:code", controller.Delete)
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

func (controller *Controller) Create(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	var request createRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Create(ctx.Context(), identity, languages.CreateCommand{
		Code: request.Code, Name: request.Name, IsDefault: request.IsDefault,
		SiteLogo: request.SiteLogo, SiteTitle: request.SiteTitle,
		DefaultThousandsSeparator: request.DefaultThousandsSeparator, DefaultDecimalPoint: request.DefaultDecimalPoint,
		DefaultCurrencySymbol: request.DefaultCurrencySymbol, DefaultMoneyThousandsSep: request.DefaultMoneyThousandsSep,
		DefaultMoneyDecimalPoint: request.DefaultMoneyDecimalPoint,
		TermsAndConditionText:    request.TermsAndConditionText, AnnouncementText: request.AnnouncementText, AboutText: request.AboutText,
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
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Update(ctx.Context(), identity, languages.UpdateCommand{
		Code: strings.TrimSpace(ctx.Params("code")), Name: request.Name, IsDefault: request.IsDefault,
		SiteLogo: request.SiteLogo, SiteTitle: request.SiteTitle,
		DefaultThousandsSeparator: request.DefaultThousandsSeparator, DefaultDecimalPoint: request.DefaultDecimalPoint,
		DefaultCurrencySymbol: request.DefaultCurrencySymbol, DefaultMoneyThousandsSep: request.DefaultMoneyThousandsSep,
		DefaultMoneyDecimalPoint: request.DefaultMoneyDecimalPoint,
		TermsAndConditionText:    request.TermsAndConditionText, AnnouncementText: request.AnnouncementText, AboutText: request.AboutText,
	})
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
	if err := controller.service.Delete(ctx.Context(), identity, strings.TrimSpace(ctx.Params("code"))); err != nil {
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
		return languages.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return languages.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return languages.ErrInvalidInput
	}
	return nil
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, languages.ErrInvalidInput), errors.Is(err, languages.ErrCannotDeleteDefault):
		return fiber.ErrBadRequest
	case errors.Is(err, languages.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, languages.ErrDuplicate), errors.Is(err, languages.ErrOnlyOneDefault):
		return fiber.NewError(fiber.StatusConflict, "language conflict")
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

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
	"cch.local/backend/internal/modules/help"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 2 << 20

type Service interface {
	List(context.Context, authentication.VerifiedIdentity, help.ListQuery) (help.Page, error)
	Get(context.Context, authentication.VerifiedIdentity, int64) (help.Article, error)
	LookupOptions(context.Context, authentication.VerifiedIdentity) (help.LookupOptions, error)
	Create(context.Context, authentication.VerifiedIdentity, help.CreateCommand) (help.Article, error)
	Update(context.Context, authentication.VerifiedIdentity, help.UpdateCommand) (help.Article, error)
	Delete(context.Context, authentication.VerifiedIdentity, int64) error
}

type Controller struct{ service Service }

type articleRequest struct {
	Language      string `json:"language"`
	Topic         string `json:"topic"`
	Description   string `json:"description"`
	CategoryID    int64  `json:"category_id"`
	Order         int64  `json:"order"`
	DisplayInPage string `json:"display_in_page"`
}

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/help/articles", controller.List)
	router.Get("/help/articles/options", controller.LookupOptions)
	router.Post("/help/articles", controller.Create)
	router.Get("/help/articles/:id", controller.Get)
	router.Put("/help/articles/:id", controller.Update)
	router.Delete("/help/articles/:id", controller.Delete)
}

func (controller *Controller) List(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	limit, err := parseOptionalInt(ctx.Query("limit"))
	if err != nil {
		return fiber.ErrBadRequest
	}
	offset, err := parseOptionalInt(ctx.Query("offset"))
	if err != nil {
		return fiber.ErrBadRequest
	}
	categoryID := int64(0)
	if value := strings.TrimSpace(ctx.Query("category_id")); value != "" {
		categoryID, err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fiber.ErrBadRequest
		}
	}
	page, err := controller.service.List(ctx.Context(), identity, help.ListQuery{
		Language:      ctx.Query("language"),
		CategoryID:    categoryID,
		DisplayInPage: ctx.Query("display_in_page"),
		Limit:         limit,
		Offset:        offset,
	})
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
	options, err := controller.service.LookupOptions(ctx.Context(), identity)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(options)
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
	article, err := controller.service.Get(ctx.Context(), identity, id)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(article)
}

func (controller *Controller) Create(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	var request articleRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	article, err := controller.service.Create(ctx.Context(), identity, toCommand(request))
	if err != nil {
		return controllerError(err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(article)
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
	var request articleRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	article, err := controller.service.Update(ctx.Context(), identity, help.UpdateCommand{ID: id, CreateCommand: toCommand(request)})
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(article)
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

func toCommand(request articleRequest) help.CreateCommand {
	return help.CreateCommand{
		Language:      request.Language,
		Topic:         request.Topic,
		Description:   request.Description,
		CategoryID:    request.CategoryID,
		Order:         request.Order,
		DisplayInPage: request.DisplayInPage,
	}
}

func decodeStrict(ctx fiber.Ctx, target any) error {
	body := ctx.Body()
	if len(body) == 0 || len(body) > maxRequestBodyBytes {
		return help.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return help.ErrInvalidInput
	}
	var trailing any
	if !errors.Is(decoder.Decode(&trailing), io.EOF) {
		return help.ErrInvalidInput
	}
	return nil
}

func parseOptionalInt(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id < 1 {
		return 0, help.ErrInvalidInput
	}
	return id, nil
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, help.ErrInvalidInput), errors.Is(err, help.ErrInvalidLanguage), errors.Is(err, help.ErrInvalidCategory):
		return fiber.ErrBadRequest
	case errors.Is(err, help.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/announcements"
	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 1 << 20

type Service interface {
	List(context.Context, authentication.VerifiedIdentity, announcements.ListQuery) (announcements.Page, error)
	Options(context.Context, authentication.VerifiedIdentity) (announcements.Options, error)
	Get(context.Context, authentication.VerifiedIdentity, int64) (announcements.Announcement, error)
	Create(context.Context, authentication.VerifiedIdentity, announcements.CreateCommand) (announcements.Announcement, error)
	Update(context.Context, authentication.VerifiedIdentity, announcements.UpdateCommand) (announcements.Announcement, error)
	Delete(context.Context, authentication.VerifiedIdentity, int64) error
}

type Controller struct{ service Service }

type announcementMutationRequest struct {
	IsActive     bool    `json:"is_active"`
	Topic        string  `json:"topic"`
	Message      string  `json:"message"`
	Language     string  `json:"language"`
	AutoPublish  bool    `json:"auto_publish"`
	DateStart    *string `json:"date_start"`
	DateEnd      *string `json:"date_end"`
	TranslatedID *int64  `json:"translated_id"`
}

type announcementDataResponse struct {
	Data announcements.Announcement `json:"data"`
}

func New(service Service) *Controller {
	return &Controller{service: service}
}

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/announcements", controller.List)
	router.Get("/announcements/options", controller.Options)
	router.Post("/announcements", controller.Create)
	router.Get("/announcements/:id", controller.Get)
	router.Put("/announcements/:id", controller.Update)
	router.Delete("/announcements/:id", controller.Delete)
}

func (controller *Controller) List(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	limit, offset, err := parsePagination(ctx)
	if err != nil {
		return fiber.ErrBadRequest
	}
	language := strings.TrimSpace(ctx.Query("language"))
	page, err := controller.service.List(ctx.Context(), identity, announcements.ListQuery{Language: language, Limit: limit, Offset: offset})
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(page)
}

func (controller *Controller) Options(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	options, err := controller.service.Options(ctx.Context(), identity)
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
	value, err := controller.service.Get(ctx.Context(), identity, id)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(announcementDataResponse{Data: value})
}

func (controller *Controller) Create(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	var request announcementMutationRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	dateStart, err := parseOptionalTime(request.DateStart)
	if err != nil {
		return fiber.ErrBadRequest
	}
	dateEnd, err := parseOptionalTime(request.DateEnd)
	if err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Create(ctx.Context(), identity, announcements.CreateCommand{
		IsActive:     request.IsActive,
		Topic:        request.Topic,
		Message:      request.Message,
		Language:     request.Language,
		AutoPublish:  request.AutoPublish,
		DateStart:    dateStart,
		DateEnd:      dateEnd,
		TranslatedID: request.TranslatedID,
	})
	if err != nil {
		return controllerError(err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(announcementDataResponse{Data: value})
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
	var request announcementMutationRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	dateStart, err := parseOptionalTime(request.DateStart)
	if err != nil {
		return fiber.ErrBadRequest
	}
	dateEnd, err := parseOptionalTime(request.DateEnd)
	if err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Update(ctx.Context(), identity, announcements.UpdateCommand{
		ID:           id,
		IsActive:     request.IsActive,
		Topic:        request.Topic,
		Message:      request.Message,
		Language:     request.Language,
		AutoPublish:  request.AutoPublish,
		DateStart:    dateStart,
		DateEnd:      dateEnd,
		TranslatedID: request.TranslatedID,
	})
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(announcementDataResponse{Data: value})
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
	return ctx.SendStatus(http.StatusNoContent)
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

func parsePagination(ctx fiber.Ctx) (int, int, error) {
	query := announcements.ListQuery{}
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, announcements.ErrInvalidInput
		}
		query.Limit = limit
	}
	if raw := strings.TrimSpace(ctx.Query("offset")); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, announcements.ErrInvalidInput
		}
		query.Offset = offset
	}
	query, err := query.Normalize()
	return query.Limit, query.Offset, err
}

func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, announcements.ErrInvalidInput
	}
	return id, nil
}

func parseOptionalTime(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func decodeStrict(ctx fiber.Ctx, target any) error {
	body := ctx.Body()
	if len(body) == 0 || len(body) > maxRequestBodyBytes {
		return announcements.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return announcements.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return announcements.ErrInvalidInput
	}
	return nil
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, announcements.ErrInvalidInput),
		errors.Is(err, announcements.ErrDateRange):
		return fiber.ErrBadRequest
	case errors.Is(err, announcements.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, announcements.ErrDateOverlaps),
		errors.Is(err, announcements.ErrDuplicateTranslation),
		errors.Is(err, announcements.ErrInUse):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, announcements.ErrInvalidLanguage):
		return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, authorization.ErrPermissionDenied),
		errors.Is(err, authorization.ErrSystemAdministratorUnsupported),
		errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

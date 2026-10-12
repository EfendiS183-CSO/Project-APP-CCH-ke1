package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/appsettings"
	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxRequestBodyBytes = 16 * 1024

type Service interface {
	List(context.Context, authentication.VerifiedIdentity) ([]appsettings.Settings, error)
	Get(context.Context, authentication.VerifiedIdentity, int64) (appsettings.Settings, error)
	Create(context.Context, authentication.VerifiedIdentity, appsettings.CreateCommand) (appsettings.Settings, error)
	Update(context.Context, authentication.VerifiedIdentity, appsettings.UpdateCommand) (appsettings.Settings, error)
	Delete(context.Context, authentication.VerifiedIdentity, int64) error
}

type Controller struct{ service Service }

type createRequest struct {
	IsDefaultOption            bool    `json:"is_default_option"`
	ShowAnnouncement           bool    `json:"show_announcement"`
	UseAnnouncementTable       bool    `json:"use_announcement_table"`
	MaintenanceMode            bool    `json:"maintenance_mode"`
	MaintenanceFinishDateTime  *string `json:"maintenance_finish_datetime,omitempty"`
	AutoNormalAfterMaintenance bool    `json:"auto_normal_after_maintenance"`
}

type updateRequest struct {
	ShowAnnouncement           bool    `json:"show_announcement"`
	UseAnnouncementTable       bool    `json:"use_announcement_table"`
	MaintenanceMode            bool    `json:"maintenance_mode"`
	MaintenanceFinishDateTime  *string `json:"maintenance_finish_datetime,omitempty"`
	AutoNormalAfterMaintenance bool    `json:"auto_normal_after_maintenance"`
}

func New(service Service) *Controller { return &Controller{service: service} }

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/app-settings", controller.List)
	router.Post("/app-settings", controller.Create)
	router.Get("/app-settings/:id", controller.Get)
	router.Put("/app-settings/:id", controller.Update)
	router.Delete("/app-settings/:id", controller.Delete)
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
	maintenanceFinish, err := parseOptionalDateTime(request.MaintenanceFinishDateTime)
	if err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Create(ctx.Context(), identity, appsettings.CreateCommand{
		IsDefaultOption: request.IsDefaultOption, ShowAnnouncement: request.ShowAnnouncement,
		UseAnnouncementTable: request.UseAnnouncementTable, MaintenanceMode: request.MaintenanceMode,
		MaintenanceFinishDateTime: maintenanceFinish, AutoNormalAfterMaintenance: request.AutoNormalAfterMaintenance,
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
	id, err := strconv.ParseInt(strings.TrimSpace(ctx.Params("id")), 10, 64)
	if err != nil {
		return fiber.ErrBadRequest
	}
	var request updateRequest
	if err := decodeStrict(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	maintenanceFinish, err := parseOptionalDateTime(request.MaintenanceFinishDateTime)
	if err != nil {
		return fiber.ErrBadRequest
	}
	value, err := controller.service.Update(ctx.Context(), identity, appsettings.UpdateCommand{
		OptionID: id, ShowAnnouncement: request.ShowAnnouncement,
		UseAnnouncementTable: request.UseAnnouncementTable, MaintenanceMode: request.MaintenanceMode,
		MaintenanceFinishDateTime: maintenanceFinish, AutoNormalAfterMaintenance: request.AutoNormalAfterMaintenance,
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
	id, err := strconv.ParseInt(strings.TrimSpace(ctx.Params("id")), 10, 64)
	if err != nil {
		return fiber.ErrBadRequest
	}
	if err := controller.service.Delete(ctx.Context(), identity, id); err != nil {
		return controllerError(err)
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

func parseOptionalDateTime(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		// Fall back to the legacy "2006-01-02 15:04:05" layout for convenience.
		parsed, err = time.Parse("2006-01-02 15:04:05", strings.TrimSpace(*value))
		if err != nil {
			return nil, appsettings.ErrInvalidInput
		}
	}
	return &parsed, nil
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
		return appsettings.ErrInvalidInput
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return appsettings.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return appsettings.ErrInvalidInput
	}
	return nil
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, appsettings.ErrInvalidInput), errors.Is(err, appsettings.ErrDeleteRestricted), errors.Is(err, appsettings.ErrCannotSetMaintenanceNow):
		return fiber.ErrBadRequest
	case errors.Is(err, appsettings.ErrNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, appsettings.ErrOnlyOneDefault):
		return fiber.NewError(fiber.StatusConflict, "app settings conflict")
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

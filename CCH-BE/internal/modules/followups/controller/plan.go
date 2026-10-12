package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/followups"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxCreatePlanBodyBytes = 16 * 1024

type CallCenterPlanCreator interface {
	Create(context.Context, authentication.VerifiedIdentity, followups.CreatePlanRequest) (followups.Entry, error)
}

type PlanController struct {
	service CallCenterPlanCreator
}

type createPlanPayload struct {
	StatusCode string    `json:"status_code"`
	PlannedAt  time.Time `json:"planned_at"`
	PlanNotes  string    `json:"plan_notes"`
}

func NewPlanController(service CallCenterPlanCreator) *PlanController {
	return &PlanController{service: service}
}

func (controller *PlanController) Register(router fiber.Router) {
	router.Post("/complaints/call-center/:id/follow-ups", controller.Create)
}

func (controller *PlanController) Create(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "no-store")
	ctx.Set("Pragma", "no-cache")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	if rawLength := ctx.Get("Content-Length"); rawLength != "" {
		length, err := strconv.ParseInt(rawLength, 10, 64)
		if err != nil || length < 0 || length > maxCreatePlanBodyBytes {
			return fiber.ErrRequestEntityTooLarge
		}
	}
	body := ctx.Body()
	if len(body) == 0 {
		return fiber.ErrBadRequest
	}
	if len(body) > maxCreatePlanBodyBytes {
		return fiber.ErrRequestEntityTooLarge
	}
	mediaType, _, err := mime.ParseMediaType(ctx.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fiber.ErrUnsupportedMediaType
	}
	var payload createPlanPayload
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return fiber.ErrBadRequest
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fiber.ErrBadRequest
	}
	entry, err := controller.service.Create(ctx.Context(), identity, followups.CreatePlanRequest{
		ComplaintID: ctx.Params("id"),
		StatusCode:  payload.StatusCode,
		PlannedAt:   payload.PlannedAt,
		PlanNotes:   payload.PlanNotes,
	})
	if err != nil {
		return planControllerError(err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(mapEntry(entry))
}

func planControllerError(err error) error {
	switch {
	case errors.Is(err, followups.ErrInvalidPlanRequest):
		return fiber.ErrBadRequest
	case errors.Is(err, followups.ErrPlanNotAvailable):
		return fiber.ErrNotFound
	case errors.Is(err, followups.ErrPlanAlreadyExists):
		return fiber.ErrConflict
	case errors.Is(err, followups.ErrCallCenterRoleRequired), errors.Is(err, authorization.ErrPermissionDenied),
		errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

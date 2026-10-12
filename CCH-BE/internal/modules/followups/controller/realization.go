package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/followups"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type CallCenterRealizationService interface {
	Realize(context.Context, authentication.VerifiedIdentity, followups.RealizeRequest) (followups.RealizeResult, error)
}

type RealizationController struct {
	service CallCenterRealizationService
}

type realizationPayload struct {
	Outcome string `json:"outcome"`
}

type realizationResponse struct {
	followUpItem
	Replayed bool `json:"replayed"`
}

func NewRealizationController(service CallCenterRealizationService) *RealizationController {
	return &RealizationController{service: service}
}

func (controller *RealizationController) Register(router fiber.Router) {
	router.Post("/complaints/call-center/:id/follow-ups/:status/realize", controller.Realize)
}

func (controller *RealizationController) Realize(ctx fiber.Ctx) error {
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
	var payload realizationPayload
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return fiber.ErrBadRequest
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fiber.ErrBadRequest
	}
	result, err := controller.service.Realize(ctx.Context(), identity, followups.RealizeRequest{
		ComplaintID: ctx.Params("id"),
		StatusCode:  ctx.Params("status"),
		Outcome:     payload.Outcome,
	})
	if err != nil {
		return realizationControllerError(err)
	}
	response := realizationResponse{followUpItem: mapEntry(result.Entry), Replayed: result.Replayed}
	if result.Replayed {
		return ctx.Status(fiber.StatusOK).JSON(response)
	}
	return ctx.Status(fiber.StatusCreated).JSON(response)
}

func realizationControllerError(err error) error {
	switch {
	case errors.Is(err, followups.ErrInvalidRealization):
		return fiber.ErrBadRequest
	case errors.Is(err, followups.ErrRealizationNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, followups.ErrRealizationConflict):
		return fiber.ErrConflict
	case errors.Is(err, followups.ErrCallCenterRoleRequired), errors.Is(err, authorization.ErrPermissionDenied),
		errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

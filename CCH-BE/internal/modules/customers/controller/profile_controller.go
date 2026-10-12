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

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/customers"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxProfileRequestBodyBytes = 32 * 1024

type ProfileManager interface {
	List(context.Context, authentication.VerifiedIdentity, customers.ProfileListQuery) (customers.ProfilePage, error)
	Get(context.Context, authentication.VerifiedIdentity, int64) (customers.Profile, error)
	Create(context.Context, authentication.VerifiedIdentity, customers.CreateProfileCommand) (customers.Profile, error)
	Update(context.Context, authentication.VerifiedIdentity, customers.UpdateProfileCommand) (customers.Profile, error)
	Delete(context.Context, authentication.VerifiedIdentity, int64) error
}

type ProfileController struct{ service ProfileManager }

type profileMutationRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Address  string `json:"address"`
}

type profileDataResponse struct {
	Data customers.Profile `json:"data"`
}

func NewProfileController(service ProfileManager) *ProfileController {
	return &ProfileController{service: service}
}

func (controller *ProfileController) Register(router fiber.Router) {
	router.Get("/customers", controller.List)
	router.Post("/customers", controller.Create)
	router.Get("/customers/:id", controller.Get)
	router.Put("/customers/:id", controller.Update)
	router.Delete("/customers/:id", controller.Delete)
}

func (controller *ProfileController) List(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	limit, offset, err := parseProfilePagination(ctx)
	if err != nil {
		return fiber.ErrBadRequest
	}
	values, err := controller.service.List(ctx.Context(), identity, customers.ProfileListQuery{Limit: limit, Offset: offset})
	if err != nil {
		return profileControllerError(err)
	}
	return ctx.JSON(values)
}

func (controller *ProfileController) Get(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	profileID, err := parseProfileID(ctx.Params("id"))
	if err != nil {
		return fiber.ErrBadRequest
	}
	profile, err := controller.service.Get(ctx.Context(), identity, profileID)
	if err != nil {
		return profileControllerError(err)
	}
	return ctx.JSON(profileDataResponse{Data: profile})
}

func (controller *ProfileController) Create(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	var request profileMutationRequest
	if err := decodeProfileRequest(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	profile, err := controller.service.Create(ctx.Context(), identity, customers.CreateProfileCommand{
		FullName: request.FullName, Email: request.Email, Phone: request.Phone, Address: request.Address,
	})
	if err != nil {
		return profileControllerError(err)
	}
	return ctx.Status(fiber.StatusCreated).JSON(profileDataResponse{Data: profile})
}

func (controller *ProfileController) Update(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	profileID, err := parseProfileID(ctx.Params("id"))
	if err != nil {
		return fiber.ErrBadRequest
	}
	var request profileMutationRequest
	if err := decodeProfileRequest(ctx, &request); err != nil {
		return fiber.ErrBadRequest
	}
	profile, err := controller.service.Update(ctx.Context(), identity, customers.UpdateProfileCommand{
		ID: profileID, FullName: request.FullName, Email: request.Email, Phone: request.Phone, Address: request.Address,
	})
	if err != nil {
		return profileControllerError(err)
	}
	return ctx.JSON(profileDataResponse{Data: profile})
}

func (controller *ProfileController) Delete(ctx fiber.Ctx) error {
	identity, err := controller.identity(ctx)
	if err != nil {
		return err
	}
	profileID, err := parseProfileID(ctx.Params("id"))
	if err != nil {
		return fiber.ErrBadRequest
	}
	if err := controller.service.Delete(ctx.Context(), identity, profileID); err != nil {
		return profileControllerError(err)
	}
	return ctx.SendStatus(http.StatusNoContent)
}

func (controller *ProfileController) identity(ctx fiber.Ctx) (authentication.VerifiedIdentity, error) {
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

func parseProfilePagination(ctx fiber.Ctx) (int, int, error) {
	query := customers.ProfileListQuery{}
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, customers.ErrInvalidProfile
		}
		query.Limit = limit
	}
	if raw := strings.TrimSpace(ctx.Query("offset")); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, customers.ErrInvalidProfile
		}
		query.Offset = offset
	}
	query, err := query.NormalizeRequest()
	return query.Limit, query.Offset, err
}

func parseProfileID(raw string) (int64, error) {
	profileID, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || profileID <= 0 {
		return 0, customers.ErrInvalidProfileID
	}
	return profileID, nil
}

func decodeProfileRequest(ctx fiber.Ctx, target any) error {
	body := ctx.Body()
	if len(body) == 0 || len(body) > maxProfileRequestBodyBytes {
		return customers.ErrInvalidProfile
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return customers.ErrInvalidProfile
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return customers.ErrInvalidProfile
	}
	return nil
}

func profileControllerError(err error) error {
	switch {
	case errors.Is(err, customers.ErrInvalidSearch), errors.Is(err, customers.ErrInvalidProfile), errors.Is(err, customers.ErrInvalidProfileID):
		return fiber.ErrBadRequest
	case errors.Is(err, customers.ErrProfileNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, customers.ErrProfileAlreadyExists), errors.Is(err, customers.ErrProfileInUse):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, authorization.ErrPermissionDenied), errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	case errors.Is(err, customers.ErrOfficeRequired):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

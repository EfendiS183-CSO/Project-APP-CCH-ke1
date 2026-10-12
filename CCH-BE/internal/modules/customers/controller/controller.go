package controller

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/customers"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type CallCenterSearcher interface {
	Search(context.Context, authentication.VerifiedIdentity, customers.SearchQuery) ([]customers.Profile, error)
}

type CallCenterProfileGetter interface {
	GetProfileForTicket(context.Context, authentication.VerifiedIdentity, int64) (customers.Profile, error)
}

type CustomerController struct {
	service CallCenterSearcher
}

type searchResponse struct {
	Data  []customers.Profile `json:"data"`
	Limit int                 `json:"limit"`
}

type profileResponse struct {
	Data customers.Profile `json:"data"`
}

func New(service CallCenterSearcher) *CustomerController {
	return &CustomerController{service: service}
}

func (controller *CustomerController) Register(router fiber.Router) {
	router.Get("/complaints/call-center/customers/search", controller.SearchCallCenter)
	router.Get("/complaints/call-center/customers/:id", controller.GetCallCenterProfile)
}

func (controller *CustomerController) SearchCallCenter(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "private, no-store")
	ctx.Set("Pragma", "no-cache")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	query, err := parseSearchQuery(ctx)
	if err != nil {
		return fiber.ErrBadRequest
	}
	profiles, err := controller.service.Search(ctx.Context(), identity, query)
	if err != nil {
		switch {
		case errors.Is(err, customers.ErrInvalidSearch):
			return fiber.ErrBadRequest
		case errors.Is(err, authorization.ErrPermissionDenied),
			errors.Is(err, authorization.ErrSystemAdministratorUnsupported),
			errors.Is(err, authorization.ErrInvalidUserLevels),
			errors.Is(err, customers.ErrCallCenterRequired),
			errors.Is(err, customers.ErrOfficeRequired):
			return fiber.ErrForbidden
		default:
			return httpapi.AuthorizationHTTPError(err)
		}
	}
	return ctx.JSON(searchResponse{Data: profiles, Limit: query.Limit})
}

func (controller *CustomerController) GetCallCenterProfile(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "private, no-store")
	ctx.Set("Pragma", "no-cache")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	profileGetter, ok := controller.service.(CallCenterProfileGetter)
	if !ok {
		return fiber.ErrServiceUnavailable
	}
	profileID, err := strconv.ParseInt(strings.TrimSpace(ctx.Params("id")), 10, 64)
	if err != nil || profileID <= 0 {
		return fiber.ErrBadRequest
	}
	profile, err := profileGetter.GetProfileForTicket(ctx.Context(), identity, profileID)
	if err != nil {
		switch {
		case errors.Is(err, customers.ErrInvalidProfileID):
			return fiber.ErrBadRequest
		case errors.Is(err, customers.ErrProfileNotFound):
			return fiber.ErrNotFound
		case errors.Is(err, authorization.ErrPermissionDenied),
			errors.Is(err, authorization.ErrSystemAdministratorUnsupported),
			errors.Is(err, authorization.ErrInvalidUserLevels),
			errors.Is(err, customers.ErrCallCenterRequired),
			errors.Is(err, customers.ErrOfficeRequired):
			return fiber.ErrForbidden
		default:
			return httpapi.AuthorizationHTTPError(err)
		}
	}
	return ctx.JSON(profileResponse{Data: profile})
}

func parseSearchQuery(ctx fiber.Ctx) (customers.SearchQuery, error) {
	query := customers.SearchQuery{Term: ctx.Query("q")}
	if rawLimit := strings.TrimSpace(ctx.Query("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil {
			return customers.SearchQuery{}, customers.ErrInvalidSearch
		}
		query.Limit = limit
	}
	return query.NormalizeRequest()
}

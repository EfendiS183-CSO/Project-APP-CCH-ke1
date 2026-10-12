package controller

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/followups"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const responseTimestampLayout = "2006-01-02T15:04:05Z"

type CallCenterFollowUpLister interface {
	ListForCallCenterTicket(context.Context, authentication.VerifiedIdentity, followups.Query) ([]followups.Entry, error)
	ListForCallCenterOffice(context.Context, authentication.VerifiedIdentity, followups.OfficeQuery) ([]followups.Entry, error)
	ListDueTodayForCallCenterOffice(context.Context, authentication.VerifiedIdentity, followups.OfficeQuery) ([]followups.Entry, error)
}

type Controller struct {
	service CallCenterFollowUpLister
}

type listResponse struct {
	Data       []followUpItem   `json:"data"`
	Pagination paginationResult `json:"pagination"`
}

type followUpItem struct {
	ComplaintID string  `json:"complaint_id"`
	OfficeCode  string  `json:"office_code"`
	StatusCode  string  `json:"status_code"`
	PlannedAt   *string `json:"planned_at"`
	PlanNotes   string  `json:"plan_notes"`
	RealizedAt  *string `json:"realized_at"`
	Outcome     string  `json:"outcome"`
}

type paginationResult struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func New(service CallCenterFollowUpLister) *Controller {
	return &Controller{service: service}
}

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/complaints/call-center/:id/follow-ups", controller.ListForCallCenterTicket)
	router.Get("/follow-ups/call-center/office", controller.ListForCallCenterOffice)
	router.Get("/follow-ups/call-center/office/due-today", controller.ListDueTodayForCallCenterOffice)
}

func (controller *Controller) ListForCallCenterTicket(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	query, err := parseQuery(ctx)
	if err != nil {
		return fiber.ErrBadRequest
	}
	entries, err := controller.service.ListForCallCenterTicket(ctx.Context(), identity, query)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(newListResponse(entries, query.Limit, query.Offset))
}

func (controller *Controller) ListForCallCenterOffice(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	query, err := parseOfficeQuery(ctx)
	if err != nil {
		return fiber.ErrBadRequest
	}
	entries, err := controller.service.ListForCallCenterOffice(ctx.Context(), identity, query)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(newListResponse(entries, query.Limit, query.Offset))
}

func (controller *Controller) ListDueTodayForCallCenterOffice(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	query, err := parseOfficeQuery(ctx)
	if err != nil {
		return fiber.ErrBadRequest
	}
	entries, err := controller.service.ListDueTodayForCallCenterOffice(ctx.Context(), identity, query)
	if err != nil {
		return controllerError(err)
	}
	return ctx.JSON(newListResponse(entries, query.Limit, query.Offset))
}

func newListResponse(entries []followups.Entry, limit int, offset int) listResponse {
	items := make([]followUpItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, mapEntry(entry))
	}
	return listResponse{
		Data: items,
		Pagination: paginationResult{
			Limit:  limit,
			Offset: offset,
		},
	}
}

func parseQuery(ctx fiber.Ctx) (followups.Query, error) {
	query := followups.Query{ComplaintID: ctx.Params("id")}
	if rawLimit := strings.TrimSpace(ctx.Query("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil {
			return followups.Query{}, followups.ErrInvalidQuery
		}
		query.Limit = limit
	}
	if rawOffset := strings.TrimSpace(ctx.Query("offset")); rawOffset != "" {
		offset, err := strconv.Atoi(rawOffset)
		if err != nil {
			return followups.Query{}, followups.ErrInvalidQuery
		}
		query.Offset = offset
	}
	return query.NormalizeRequest()
}

func parseOfficeQuery(ctx fiber.Ctx) (followups.OfficeQuery, error) {
	query := followups.OfficeQuery{}
	if rawLimit := strings.TrimSpace(ctx.Query("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil {
			return followups.OfficeQuery{}, followups.ErrInvalidQuery
		}
		query.Limit = limit
	}
	if rawOffset := strings.TrimSpace(ctx.Query("offset")); rawOffset != "" {
		offset, err := strconv.Atoi(rawOffset)
		if err != nil {
			return followups.OfficeQuery{}, followups.ErrInvalidQuery
		}
		query.Offset = offset
	}
	return query.NormalizeRequest()
}

func mapEntry(entry followups.Entry) followUpItem {
	return followUpItem{
		ComplaintID: entry.ComplaintID,
		OfficeCode:  entry.OfficeCode,
		StatusCode:  entry.StatusCode,
		PlannedAt:   formatTime(entry.PlannedAt),
		PlanNotes:   entry.PlanNotes,
		RealizedAt:  formatTime(entry.RealizedAt),
		Outcome:     entry.Outcome,
	}
}

func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(responseTimestampLayout)
	return &formatted
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, followups.ErrInvalidQuery):
		return fiber.ErrBadRequest
	case errors.Is(err, followups.ErrNotFoundOrNotVisible):
		return fiber.ErrNotFound
	case errors.Is(err, followups.ErrCallCenterRoleRequired),
		errors.Is(err, followups.ErrCallCenterOfficeRequired),
		errors.Is(err, authorization.ErrPermissionDenied),
		errors.Is(err, authorization.ErrSystemAdministratorUnsupported),
		errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

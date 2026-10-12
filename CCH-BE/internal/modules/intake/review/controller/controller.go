package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/intake/review"
	"cch.local/backend/internal/modules/intake/submission"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type Reviewer interface {
	ListPending(context.Context, authentication.VerifiedIdentity, review.Query) ([]review.DraftSummary, error)
	GetPending(context.Context, authentication.VerifiedIdentity, int64) (review.PendingDraft, error)
	SearchCustomers(context.Context, authentication.VerifiedIdentity, string) ([]review.CustomerCandidate, error)
}

type DraftTicketCreator interface {
	CreateAfterVerification(context.Context, authentication.VerifiedIdentity, int64, review.CreateRequest) (submission.Result, error)
}

type Controller struct {
	service Reviewer
	creator DraftTicketCreator
}

type queueResponse struct {
	Data       []draftSummary `json:"data"`
	Pagination pagination     `json:"pagination"`
}

type pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type draftSummary struct {
	ID           int64  `json:"id"`
	Summary      string `json:"summary"`
	ReviewReason string `json:"review_reason,omitempty"`
	CreatedAt    string `json:"created_at"`
}

type draftDetailResponse struct {
	ID           int64              `json:"id"`
	Status       string             `json:"status"`
	ReviewReason string             `json:"review_reason,omitempty"`
	TicketID     string             `json:"ticket_id,omitempty"`
	CustomerID   int64              `json:"customer_id,omitempty"`
	Payload      submission.Payload `json:"payload"`
	Summary      string             `json:"summary"`
	ConsentAt    *string            `json:"consent_at,omitempty"`
	CreatedAt    string             `json:"created_at"`
}

type customerCandidate struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
}

type customerSearchResponse struct {
	Data []customerCandidate `json:"data"`
}

type ticketCreateResponse struct {
	Status   string `json:"status"`
	TicketID string `json:"ticket_id,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func New(service Reviewer, creator DraftTicketCreator) *Controller {
	return &Controller{service: service, creator: creator}
}

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/intake/review/drafts", controller.ListPending)
	router.Get("/intake/review/drafts/:id", controller.GetPending)
	router.Get("/intake/review/customers/search", controller.SearchCustomers)
	router.Post("/intake/review/drafts/:id/create-ticket", controller.CreateTicket)
}

func (controller *Controller) CreateTicket(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil || controller.creator == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "no-store")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	draftID, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil || draftID < 1 {
		return fiber.ErrBadRequest
	}
	if rawLength := ctx.Get("Content-Length"); rawLength != "" {
		length, err := strconv.ParseInt(rawLength, 10, 64)
		if err != nil || length < 0 || length > 16*1024 {
			return fiber.ErrRequestEntityTooLarge
		}
	}
	body := ctx.Body()
	if len(body) == 0 || len(body) > 16*1024 {
		return fiber.ErrBadRequest
	}
	mediaType, _, err := mime.ParseMediaType(ctx.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fiber.ErrUnsupportedMediaType
	}
	var request review.CreateRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return fiber.ErrBadRequest
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fiber.ErrBadRequest
	}
	result, err := controller.creator.CreateAfterVerification(ctx.Context(), identity, draftID, request)
	if err != nil {
		return serviceError(err)
	}
	response := ticketCreateResponse{Status: result.Status}
	switch result.Status {
	case "created":
		response.TicketID = result.TicketID
		return ctx.Status(fiber.StatusCreated).JSON(response)
	case "pending_cs", "creating":
		response.Reason = result.Reason
		return ctx.Status(fiber.StatusAccepted).JSON(response)
	default:
		return fiber.ErrServiceUnavailable
	}
}

func (controller *Controller) SearchCustomers(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "no-store")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	candidates, err := controller.service.SearchCustomers(ctx.Context(), identity, ctx.Query("q"))
	if err != nil {
		return serviceError(err)
	}
	items := make([]customerCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, customerCandidate{
			ID:      candidate.ID,
			Name:    candidate.Name,
			Email:   candidate.Email,
			Phone:   candidate.Phone,
			Address: candidate.Address,
		})
	}
	return ctx.JSON(customerSearchResponse{Data: items})
}

func (controller *Controller) ListPending(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "no-store")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	query, err := parseQuery(ctx)
	if err != nil {
		return fiber.ErrBadRequest
	}
	drafts, err := controller.service.ListPending(ctx.Context(), identity, query)
	if err != nil {
		return serviceError(err)
	}
	items := make([]draftSummary, 0, len(drafts))
	for _, draft := range drafts {
		items = append(items, draftSummary{
			ID:           draft.ID,
			Summary:      draft.Summary,
			ReviewReason: draft.ReviewReason,
			CreatedAt:    draft.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return ctx.JSON(queueResponse{
		Data:       items,
		Pagination: pagination{Limit: query.Limit, Offset: query.Offset},
	})
}

func (controller *Controller) GetPending(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "no-store")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	draftID, err := strconv.ParseInt(ctx.Params("id"), 10, 64)
	if err != nil || draftID < 1 {
		return fiber.ErrBadRequest
	}
	draft, err := controller.service.GetPending(ctx.Context(), identity, draftID)
	if err != nil {
		return serviceError(err)
	}
	response := draftDetailResponse{
		ID:           draft.ID,
		Status:       draft.Draft.Status,
		ReviewReason: draft.Draft.ReviewReason,
		TicketID:     draft.Draft.TicketID,
		CustomerID:   draft.Draft.CustomerID,
		Payload:      draft.Draft.Payload,
		Summary:      draft.Draft.Summary,
		CreatedAt:    draft.Draft.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if !draft.Draft.ConsentAt.IsZero() {
		consentAt := draft.Draft.ConsentAt.UTC().Format("2006-01-02T15:04:05Z")
		response.ConsentAt = &consentAt
	}
	return ctx.JSON(response)
}

func parseQuery(ctx fiber.Ctx) (review.Query, error) {
	query := review.Query{}
	if rawLimit := strings.TrimSpace(ctx.Query("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil {
			return review.Query{}, review.ErrInvalidQuery
		}
		query.Limit = limit
	}
	if rawOffset := strings.TrimSpace(ctx.Query("offset")); rawOffset != "" {
		offset, err := strconv.Atoi(rawOffset)
		if err != nil {
			return review.Query{}, review.ErrInvalidQuery
		}
		query.Offset = offset
	}
	return query, nil
}

func serviceError(err error) error {
	switch {
	case errors.Is(err, review.ErrReviewerForbidden):
		return fiber.ErrForbidden
	case errors.Is(err, review.ErrInvalidQuery):
		return fiber.ErrBadRequest
	case errors.Is(err, review.ErrDraftNotFound):
		return fiber.ErrNotFound
	case errors.Is(err, review.ErrIdentityVerificationRequired), errors.Is(err, review.ErrInvalidReviewRequest), errors.Is(err, submission.ErrInvalidSubmission), errors.Is(err, submission.ErrCustomerNotFound):
		return fiber.ErrBadRequest
	case errors.Is(err, submission.ErrDraftState):
		return fiber.NewError(fiber.StatusConflict, "Draft state conflict")
	case errors.Is(err, review.ErrReviewProcessing), errors.Is(err, review.ErrReviewCCHUnavailable):
		return fiber.ErrServiceUnavailable
	default:
		return fiber.ErrServiceUnavailable
	}
}

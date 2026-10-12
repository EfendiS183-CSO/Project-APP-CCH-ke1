package controller

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/documents"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const responseTimestampLayout = "2006-01-02T15:04:05Z"

type CallCenterDocumentLister interface {
	ListForCallCenterTicket(context.Context, authentication.VerifiedIdentity, documents.Query) ([]documents.Metadata, error)
}

type Controller struct {
	service CallCenterDocumentLister
}

type listResponse struct {
	Data       []documentItem   `json:"data"`
	Pagination paginationResult `json:"pagination"`
}

type documentItem struct {
	ComplaintID      string `json:"complaint_id"`
	HistoryID        string `json:"history_id"`
	DocumentTypeCode string `json:"document_type_code"`
	DocumentType     string `json:"document_type"`
	UploadOfficeCode string `json:"upload_office_code"`
	AddedAt          string `json:"added_at"`
	FileCount        int    `json:"file_count"`
}

type paginationResult struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func New(service CallCenterDocumentLister) *Controller {
	return &Controller{service: service}
}

func (controller *Controller) Register(router fiber.Router) {
	router.Get("/complaints/call-center/:id/documents", controller.ListForCallCenterTicket)
}

func (controller *Controller) ListForCallCenterTicket(ctx fiber.Ctx) error {
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
	items, err := controller.service.ListForCallCenterTicket(ctx.Context(), identity, query)
	if err != nil {
		return controllerError(err)
	}
	responseItems := make([]documentItem, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, mapMetadata(item))
	}
	return ctx.JSON(listResponse{
		Data: responseItems,
		Pagination: paginationResult{
			Limit: query.Limit, Offset: query.Offset,
		},
	})
}

func parseQuery(ctx fiber.Ctx) (documents.Query, error) {
	query := documents.Query{ComplaintID: ctx.Params("id")}
	if rawLimit := strings.TrimSpace(ctx.Query("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil {
			return documents.Query{}, documents.ErrInvalidQuery
		}
		query.Limit = limit
	}
	if rawOffset := strings.TrimSpace(ctx.Query("offset")); rawOffset != "" {
		offset, err := strconv.Atoi(rawOffset)
		if err != nil {
			return documents.Query{}, documents.ErrInvalidQuery
		}
		query.Offset = offset
	}
	return query.NormalizeRequest()
}

func mapMetadata(metadata documents.Metadata) documentItem {
	addedAt := ""
	if !metadata.AddedAt.IsZero() {
		addedAt = metadata.AddedAt.UTC().Format(responseTimestampLayout)
	}
	return documentItem{
		ComplaintID: metadata.ComplaintID, HistoryID: metadata.HistoryID,
		DocumentTypeCode: metadata.DocumentTypeCode, DocumentType: metadata.DocumentType,
		UploadOfficeCode: metadata.UploadOfficeCode, AddedAt: addedAt, FileCount: metadata.FileCount,
	}
}

func controllerError(err error) error {
	switch {
	case errors.Is(err, documents.ErrInvalidQuery):
		return fiber.ErrBadRequest
	case errors.Is(err, documents.ErrNotFoundOrNotVisible):
		return fiber.ErrNotFound
	case errors.Is(err, documents.ErrCallCenterRoleRequired), errors.Is(err, authorization.ErrPermissionDenied),
		errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

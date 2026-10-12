package controller

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/documents"
	"cch.local/backend/internal/modules/documents/storage"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

const maxUploadBytes = storage.MaxFileBytes

type CallCenterDocumentUploader interface {
	Upload(context.Context, authentication.VerifiedIdentity, documents.UploadRequest) (documents.UploadResult, error)
}

type UploadController struct {
	service CallCenterDocumentUploader
}

type uploadResponse struct {
	DocumentID   string `json:"document_id"`
	ComplaintID  string `json:"complaint_id"`
	HistoryID    string `json:"history_id"`
	DocumentType string `json:"document_type_code"`
	UploadOffice string `json:"upload_office_code"`
	MediaType    string `json:"media_type"`
	FileSize     int64  `json:"file_size"`
	CreatedAt    string `json:"created_at"`
	Replayed     bool   `json:"replayed"`
}

func NewUploadController(service CallCenterDocumentUploader) *UploadController {
	return &UploadController{service: service}
}

func (controller *UploadController) Register(router fiber.Router) {
	router.Post("/complaints/call-center/:id/documents", controller.Upload)
}

func (controller *UploadController) Upload(ctx fiber.Ctx) error {
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
		if err != nil || length < 0 || length > maxUploadBytes {
			return fiber.ErrRequestEntityTooLarge
		}
	}
	body := ctx.Body()
	if len(body) == 0 {
		return fiber.ErrBadRequest
	}
	if len(body) > maxUploadBytes {
		return fiber.ErrRequestEntityTooLarge
	}
	mediaType := strings.TrimSpace(strings.ToLower(strings.Split(ctx.Get("Content-Type"), ";")[0]))
	if mediaType == "" {
		return fiber.ErrUnsupportedMediaType
	}
	result, err := controller.service.Upload(ctx.Context(), identity, documents.UploadRequest{
		ComplaintID:    ctx.Params("id"),
		HistoryID:      strings.TrimSpace(ctx.Get("X-History-ID")),
		DocumentType:   strings.TrimSpace(ctx.Get("X-Document-Type")),
		FileName:       strings.TrimSpace(ctx.Get("X-File-Name")),
		ContentType:    mediaType,
		IdempotencyKey: strings.TrimSpace(ctx.Get("Idempotency-Key")),
		Content:        body,
	})
	if err != nil {
		return uploadControllerError(err)
	}
	response := uploadResponse{
		DocumentID: result.DocumentID, ComplaintID: result.ComplaintID, HistoryID: result.HistoryID,
		DocumentType: result.DocumentType, UploadOffice: result.UploadOffice, MediaType: result.MediaType,
		FileSize: result.FileSize, CreatedAt: result.CreatedAt.UTC().Format(responseTimestampLayout), Replayed: result.Replayed,
	}
	if result.Replayed {
		return ctx.Status(fiber.StatusOK).JSON(response)
	}
	return ctx.Status(fiber.StatusCreated).JSON(response)
}

func uploadControllerError(err error) error {
	switch {
	case errors.Is(err, documents.ErrInvalidUpload):
		return fiber.ErrBadRequest
	case errors.Is(err, storage.ErrFileTooLarge):
		return fiber.ErrRequestEntityTooLarge
	case errors.Is(err, storage.ErrUnsupportedFile):
		return fiber.ErrUnsupportedMediaType
	case errors.Is(err, storage.ErrMalwareDetected):
		return fiber.NewError(fiber.StatusUnprocessableEntity, "document rejected by malware scanner")
	case errors.Is(err, documents.ErrUploadNotAvailable), errors.Is(err, documents.ErrNotFoundOrNotVisible):
		return fiber.ErrNotFound
	case errors.Is(err, documents.ErrUploadIdempotencyConflict):
		return fiber.ErrConflict
	case errors.Is(err, documents.ErrCallCenterRoleRequired), errors.Is(err, authorization.ErrPermissionDenied),
		errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

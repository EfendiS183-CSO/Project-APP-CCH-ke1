package controller

import (
	"context"
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/documents"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type CallCenterDocumentDownloader interface {
	Download(context.Context, authentication.VerifiedIdentity, documents.DownloadRequest) (documents.DownloadResult, error)
}

type DownloadController struct {
	service CallCenterDocumentDownloader
}

func NewDownloadController(service CallCenterDocumentDownloader) *DownloadController {
	return &DownloadController{service: service}
}

func (controller *DownloadController) Register(router fiber.Router) {
	router.Get("/complaints/call-center/:id/documents/:documentId/file", controller.Download)
}

func (controller *DownloadController) Download(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "private, no-store")
	ctx.Set("X-Content-Type-Options", "nosniff")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	result, err := controller.service.Download(ctx.Context(), identity, documents.DownloadRequest{
		ComplaintID: ctx.Params("id"),
		DocumentID:  ctx.Params("documentId"),
	})
	if err != nil {
		return downloadControllerError(err)
	}
	ctx.Set("Content-Type", result.Metadata.MediaType)
	ctx.Set("Content-Disposition", "attachment; filename=\"document-"+result.Metadata.DocumentID+result.Metadata.Extension+"\"")
	ctx.Set("Content-Length", strconv.FormatInt(result.Metadata.FileSize, 10))
	return ctx.Send(result.Content)
}

func downloadControllerError(err error) error {
	switch {
	case errors.Is(err, documents.ErrInvalidQuery):
		return fiber.ErrBadRequest
	case errors.Is(err, documents.ErrNotFoundOrNotVisible):
		return fiber.ErrNotFound
	case errors.Is(err, documents.ErrCallCenterRoleRequired), errors.Is(err, authorization.ErrPermissionDenied),
		errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	case errors.Is(err, documents.ErrDownloadUnavailable):
		return fiber.ErrServiceUnavailable
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

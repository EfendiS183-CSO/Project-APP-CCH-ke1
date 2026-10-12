package controller

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/documents"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type CallCenterDocumentPreviewer interface {
	Download(context.Context, authentication.VerifiedIdentity, documents.DownloadRequest) (documents.DownloadResult, error)
}

type PreviewController struct {
	service CallCenterDocumentPreviewer
}

func NewPreviewController(service CallCenterDocumentPreviewer) *PreviewController {
	return &PreviewController{service: service}
}

func (controller *PreviewController) Register(router fiber.Router) {
	router.Get("/complaints/call-center/:id/documents/:documentId/preview", controller.Preview)
}

func (controller *PreviewController) Preview(ctx fiber.Ctx) error {
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
	ctx.Set("Content-Disposition", "inline; filename=\"document-"+result.Metadata.DocumentID+result.Metadata.Extension+"\"")
	ctx.Set("Content-Length", strconv.FormatInt(result.Metadata.FileSize, 10))
	return ctx.Send(result.Content)
}

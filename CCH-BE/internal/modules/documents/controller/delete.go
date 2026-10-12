package controller

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/documents"
	"cch.local/backend/internal/platform/authentication"
	"cch.local/backend/internal/platform/httpapi"
)

type CallCenterDocumentDeleter interface {
	Delete(context.Context, authentication.VerifiedIdentity, documents.DeleteRequest) (documents.DeleteResult, error)
}

type DeleteController struct {
	service CallCenterDocumentDeleter
}

func NewDeleteController(service CallCenterDocumentDeleter) *DeleteController {
	return &DeleteController{service: service}
}

func (controller *DeleteController) Register(router fiber.Router) {
	router.Delete("/complaints/call-center/:id/documents/:documentId", controller.Delete)
}

func (controller *DeleteController) Delete(ctx fiber.Ctx) error {
	if controller == nil || controller.service == nil {
		return fiber.ErrServiceUnavailable
	}
	ctx.Set("Cache-Control", "private, no-store")
	identity, ok := httpapi.VerifiedIdentityFromContext(ctx)
	if !ok {
		return fiber.ErrUnauthorized
	}
	if _, err := controller.service.Delete(ctx.Context(), identity, documents.DeleteRequest{
		ComplaintID: ctx.Params("id"),
		DocumentID:  ctx.Params("documentId"),
	}); err != nil {
		return deleteControllerError(err)
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

func deleteControllerError(err error) error {
	switch {
	case errors.Is(err, documents.ErrInvalidQuery):
		return fiber.ErrBadRequest
	case errors.Is(err, documents.ErrNotFoundOrNotVisible):
		return fiber.ErrNotFound
	case errors.Is(err, documents.ErrCallCenterRoleRequired), errors.Is(err, authorization.ErrPermissionDenied),
		errors.Is(err, authorization.ErrSystemAdministratorUnsupported), errors.Is(err, authorization.ErrInvalidUserLevels):
		return fiber.ErrForbidden
	case errors.Is(err, documents.ErrDeleteUnavailable):
		return fiber.ErrConflict
	default:
		return httpapi.AuthorizationHTTPError(err)
	}
}

package documents

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"slices"
	"strings"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type DeleteService struct {
	accessService *authorization.AccessService
	repository    DeleteRepository
	store         interface {
		Delete(context.Context, string) error
	}
}

func NewDeleteService(accessService *authorization.AccessService, repository DeleteRepository, store interface {
	Delete(context.Context, string) error
}) *DeleteService {
	return &DeleteService{accessService: accessService, repository: repository, store: store}
}

func (service *DeleteService) Delete(ctx context.Context, identity authentication.VerifiedIdentity, request DeleteRequest) (DeleteResult, error) {
	if service == nil || service.accessService == nil || service.repository == nil || service.store == nil {
		return DeleteResult{}, ErrServiceNotConfigured
	}
	request.ComplaintID = strings.TrimSpace(request.ComplaintID)
	request.DocumentID = strings.TrimSpace(request.DocumentID)
	if request.ComplaintID == "" || len(request.ComplaintID) > 25 || strings.Contains(request.ComplaintID, ",") ||
		len(request.DocumentID) != 32 {
		return DeleteResult{}, ErrInvalidQuery
	}
	if _, err := hex.DecodeString(request.DocumentID); err != nil {
		return DeleteResult{}, ErrInvalidQuery
	}
	complaintAccess, err := service.accessService.Authorize(ctx, identity, complaintTableName, authorization.ActionView)
	if err != nil {
		return DeleteResult{}, err
	}
	documentAccess, err := service.accessService.Authorize(ctx, identity, documentTableName, authorization.ActionDelete)
	if err != nil {
		return DeleteResult{}, err
	}
	if !isSingleCallCenterRole(complaintAccess.UserLevelIDs) || !isSingleCallCenterRole(documentAccess.UserLevelIDs) {
		return DeleteResult{}, ErrCallCenterRoleRequired
	}
	if complaintAccess.OfficeCode == "" || complaintAccess.OfficeCode != documentAccess.OfficeCode ||
		complaintAccess.ReportTo != documentAccess.ReportTo || !slices.Equal(complaintAccess.UserLevelIDs, documentAccess.UserLevelIDs) {
		return DeleteResult{}, authorization.ErrIdentityMismatch
	}
	result, err := service.repository.DeleteCallCenterDocument(ctx, request.ComplaintID, request.DocumentID, complaintAccess.OfficeCode)
	if errors.Is(err, sql.ErrNoRows) {
		return DeleteResult{}, ErrNotFoundOrNotVisible
	}
	if err != nil {
		return DeleteResult{}, err
	}
	// The metadata row is the source of truth; removing the stored object is a
	// best-effort follow-up so a failed file delete never blocks deletion.
	if service.store != nil && len(result.StorageKey) == 64 {
		_ = service.store.Delete(context.WithoutCancel(ctx), result.StorageKey)
	}
	return result, nil
}

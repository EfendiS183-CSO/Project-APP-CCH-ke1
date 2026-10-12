package documents

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"slices"
	"strings"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/documents/storage"
	"cch.local/backend/internal/platform/authentication"
)

type DownloadService struct {
	accessService *authorization.AccessService
	repository    DownloadRepository
	reader        interface {
		Open(context.Context, string) (io.ReadCloser, error)
	}
}

func NewDownloadService(accessService *authorization.AccessService, repository DownloadRepository, reader interface {
	Open(context.Context, string) (io.ReadCloser, error)
}) *DownloadService {
	return &DownloadService{accessService: accessService, repository: repository, reader: reader}
}

func (service *DownloadService) Download(ctx context.Context, identity authentication.VerifiedIdentity, request DownloadRequest) (DownloadResult, error) {
	if service == nil || service.accessService == nil || service.repository == nil || service.reader == nil {
		return DownloadResult{}, ErrServiceNotConfigured
	}
	request.ComplaintID = strings.TrimSpace(request.ComplaintID)
	request.DocumentID = strings.TrimSpace(request.DocumentID)
	if request.ComplaintID == "" || len(request.ComplaintID) > 25 || strings.Contains(request.ComplaintID, ",") ||
		len(request.DocumentID) != 32 {
		return DownloadResult{}, ErrInvalidQuery
	}
	if _, err := hex.DecodeString(request.DocumentID); err != nil {
		return DownloadResult{}, ErrInvalidQuery
	}
	complaintAccess, err := service.accessService.Authorize(ctx, identity, complaintTableName, authorization.ActionView)
	if err != nil {
		return DownloadResult{}, err
	}
	documentAccess, err := service.accessService.Authorize(ctx, identity, documentTableName, authorization.ActionList)
	if err != nil {
		return DownloadResult{}, err
	}
	if !isSingleCallCenterRole(complaintAccess.UserLevelIDs) || !isSingleCallCenterRole(documentAccess.UserLevelIDs) {
		return DownloadResult{}, ErrCallCenterRoleRequired
	}
	if complaintAccess.OfficeCode == "" || complaintAccess.OfficeCode != documentAccess.OfficeCode ||
		complaintAccess.ReportTo != documentAccess.ReportTo || !slices.Equal(complaintAccess.UserLevelIDs, documentAccess.UserLevelIDs) {
		return DownloadResult{}, authorization.ErrIdentityMismatch
	}
	metadata, err := service.repository.GetCallCenterDocument(ctx, request.ComplaintID, request.DocumentID, complaintAccess.OfficeCode)
	if errors.Is(err, sql.ErrNoRows) {
		return DownloadResult{}, ErrNotFoundOrNotVisible
	}
	if err != nil {
		return DownloadResult{}, err
	}
	if metadata.FileSize < 1 || metadata.FileSize > storage.MaxFileBytes || len(metadata.StorageKey) != 64 {
		return DownloadResult{}, ErrDownloadUnavailable
	}
	file, err := service.reader.Open(ctx, metadata.StorageKey)
	if err != nil {
		return DownloadResult{}, ErrDownloadUnavailable
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, storage.MaxFileBytes+1))
	if err != nil || int64(len(content)) != metadata.FileSize || int64(len(content)) > storage.MaxFileBytes {
		return DownloadResult{}, ErrDownloadUnavailable
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != metadata.SHA256 {
		return DownloadResult{}, ErrDownloadUnavailable
	}
	fileInfo, err := storage.ValidateFile("document"+metadata.Extension, content)
	if err != nil || fileInfo.MediaType != metadata.MediaType {
		return DownloadResult{}, ErrDownloadUnavailable
	}
	return DownloadResult{Metadata: metadata, Content: content}, nil
}

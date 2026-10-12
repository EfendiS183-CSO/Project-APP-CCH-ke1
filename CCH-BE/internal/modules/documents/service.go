package documents

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/documents/storage"
	"cch.local/backend/internal/platform/authentication"
)

const (
	complaintTableName = "cchentri"
	documentTableName  = "cchentridok"
	callCenterRoleID   = 3
)

type Service struct {
	accessService *authorization.AccessService
	repository    Repository
}

func NewService(accessService *authorization.AccessService, repository Repository) *Service {
	return &Service{accessService: accessService, repository: repository}
}

func (service *Service) ListForCallCenterTicket(ctx context.Context, identity authentication.VerifiedIdentity, query Query) ([]Metadata, error) {
	if service == nil || service.accessService == nil || service.repository == nil {
		return nil, ErrServiceNotConfigured
	}
	query.OfficeCode = ""
	query, err := query.NormalizeRequest()
	if err != nil {
		return nil, err
	}
	complaintAccess, err := service.accessService.Authorize(ctx, identity, complaintTableName, authorization.ActionView)
	if err != nil {
		return nil, err
	}
	documentAccess, err := service.accessService.Authorize(ctx, identity, documentTableName, authorization.ActionList)
	if err != nil {
		return nil, err
	}
	if !isSingleCallCenterRole(complaintAccess.UserLevelIDs) || !isSingleCallCenterRole(documentAccess.UserLevelIDs) {
		return nil, ErrCallCenterRoleRequired
	}
	if complaintAccess.OfficeCode == "" || complaintAccess.OfficeCode != documentAccess.OfficeCode ||
		complaintAccess.ReportTo != documentAccess.ReportTo || !slices.Equal(complaintAccess.UserLevelIDs, documentAccess.UserLevelIDs) {
		return nil, authorization.ErrIdentityMismatch
	}
	query.OfficeCode = complaintAccess.OfficeCode
	query, err = query.Normalize()
	if err != nil {
		return nil, err
	}
	metadata, err := service.repository.ListForOpenCallCenterTicket(ctx, query)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFoundOrNotVisible
	}
	return metadata, err
}

func isSingleCallCenterRole(userLevelIDs []int64) bool {
	return len(userLevelIDs) == 1 && userLevelIDs[0] == callCenterRoleID
}

type UploadService struct {
	accessService *authorization.AccessService
	repository    UploadRepository
	ingestor      *storage.Ingestor
}

func NewUploadService(accessService *authorization.AccessService, repository UploadRepository, ingestor *storage.Ingestor) *UploadService {
	return &UploadService{accessService: accessService, repository: repository, ingestor: ingestor}
}

func (service *UploadService) Upload(ctx context.Context, identity authentication.VerifiedIdentity, request UploadRequest) (UploadResult, error) {
	if service == nil || service.accessService == nil || service.repository == nil || service.ingestor == nil {
		return UploadResult{}, ErrServiceNotConfigured
	}
	request, err := normalizeUploadRequest(request)
	if err != nil {
		return UploadResult{}, err
	}
	complaintAccess, err := service.accessService.Authorize(ctx, identity, complaintTableName, authorization.ActionView)
	if err != nil {
		return UploadResult{}, err
	}
	documentAccess, err := service.accessService.Authorize(ctx, identity, documentTableName, authorization.ActionAdd)
	if err != nil {
		return UploadResult{}, err
	}
	if !isSingleCallCenterRole(complaintAccess.UserLevelIDs) || !isSingleCallCenterRole(documentAccess.UserLevelIDs) {
		return UploadResult{}, ErrCallCenterRoleRequired
	}
	if complaintAccess.OfficeCode == "" || complaintAccess.OfficeCode != documentAccess.OfficeCode ||
		complaintAccess.ReportTo != documentAccess.ReportTo || !slices.Equal(complaintAccess.UserLevelIDs, documentAccess.UserLevelIDs) ||
		complaintAccess.Username == "" || len(complaintAccess.Username) > 20 {
		return UploadResult{}, authorization.ErrIdentityMismatch
	}
	fileInfo, err := storage.ValidateFile(request.FileName, request.Content)
	if err != nil {
		return UploadResult{}, err
	}
	if request.ContentType != fileInfo.MediaType {
		return UploadResult{}, ErrInvalidUpload
	}
	ingested, err := service.ingestor.Ingest(ctx, request.FileName, request.Content)
	if err != nil {
		return UploadResult{}, err
	}
	documentID, err := newDocumentID()
	if err != nil {
		service.deleteStoredObject(ingested.StorageKey)
		return UploadResult{}, err
	}
	requestHash, err := uploadRequestHash(request, complaintAccess.OfficeCode, complaintAccess.Username, ingested.SHA256)
	if err != nil {
		service.deleteStoredObject(ingested.StorageKey)
		return UploadResult{}, err
	}
	command := UploadCommand{
		UploadRequest: request,
		OfficeCode:    complaintAccess.OfficeCode,
		Actor:         complaintAccess.Username,
		StorageKey:    ingested.StorageKey,
		MediaType:     fileInfo.MediaType,
		Extension:     fileInfo.Extension,
		FileSize:      ingested.SizeBytes,
		SHA256:        ingested.SHA256,
		DocumentID:    documentID,
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
		RequestHash:   requestHash,
	}
	result, err := service.repository.CreateCallCenterDocument(ctx, command)
	if err != nil {
		if !errors.Is(err, ErrUploadCommitUncertain) {
			service.deleteStoredObject(ingested.StorageKey)
		}
		return UploadResult{}, err
	}
	if result.Replayed {
		service.deleteStoredObject(ingested.StorageKey)
	}
	return result, nil
}

func normalizeUploadRequest(request UploadRequest) (UploadRequest, error) {
	request.ComplaintID = strings.TrimSpace(request.ComplaintID)
	request.HistoryID = strings.TrimSpace(request.HistoryID)
	request.DocumentType = strings.TrimSpace(request.DocumentType)
	request.FileName = strings.TrimSpace(request.FileName)
	request.ContentType = strings.TrimSpace(strings.ToLower(request.ContentType))
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.ComplaintID == "" || len(request.ComplaintID) > 25 || strings.Contains(request.ComplaintID, ",") ||
		request.HistoryID == "" || len(request.HistoryID) > 25 || strings.Contains(request.HistoryID, ",") ||
		!validDocumentCode(request.DocumentType) || request.FileName == "" || len([]rune(request.FileName)) > 255 ||
		strings.ContainsAny(request.FileName, "\x00\r\n") || !validUploadIdempotencyKey(request.IdempotencyKey) {
		return UploadRequest{}, ErrInvalidUpload
	}
	return request, nil
}

func validDocumentCode(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if !((character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')) {
			return false
		}
	}
	return true
}

func validUploadIdempotencyKey(value string) bool {
	if len(value) < 16 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !((character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("-_.:", character)) {
			return false
		}
	}
	return true
}

func uploadRequestHash(request UploadRequest, office string, actor string, contentHash string) (string, error) {
	canonical := struct {
		ComplaintID  string
		HistoryID    string
		DocumentType string
		FileName     string
		ContentType  string
		Office       string
		Actor        string
		ContentHash  string
	}{request.ComplaintID, request.HistoryID, request.DocumentType, request.FileName, request.ContentType, office, actor, contentHash}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func newDocumentID() (string, error) {
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(randomBytes[:]), nil
}

func (service *UploadService) deleteStoredObject(storageKey string) {
	cleanupContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = service.ingestor.Delete(cleanupContext, storageKey)
}

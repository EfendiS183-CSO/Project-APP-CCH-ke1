package documents

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

var (
	ErrInvalidQuery              = errors.New("invalid complaint document query")
	ErrNotFoundOrNotVisible      = errors.New("complaint not found or not visible")
	ErrServiceNotConfigured      = errors.New("document service is not configured")
	ErrCallCenterRoleRequired    = errors.New("Call Center role is required")
	ErrInvalidUpload             = errors.New("invalid complaint document upload")
	ErrUploadNotAvailable        = errors.New("document upload is not available for this complaint")
	ErrUploadIdempotencyConflict = errors.New("document upload idempotency key was reused")
	ErrUploadCommitUncertain     = errors.New("document upload commit result is uncertain")
	ErrUploadSchemaNotReady      = errors.New("document upload schema is not ready")
	ErrDownloadUnavailable       = errors.New("document content is unavailable")
	ErrDeleteUnavailable         = errors.New("document deletion is unavailable")
)

type Query struct {
	ComplaintID string
	OfficeCode  string
	Limit       int
	Offset      int
}

func (query Query) NormalizeRequest() (Query, error) {
	query.ComplaintID = strings.TrimSpace(query.ComplaintID)
	if query.ComplaintID == "" || len(query.ComplaintID) > 25 || strings.Contains(query.ComplaintID, ",") {
		return Query{}, ErrInvalidQuery
	}
	if query.Limit == 0 {
		query.Limit = DefaultPageSize
	}
	if query.Limit < 1 || query.Limit > MaxPageSize || query.Offset < 0 {
		return Query{}, ErrInvalidQuery
	}
	return query, nil
}

func (query Query) Normalize() (Query, error) {
	query, err := query.NormalizeRequest()
	if err != nil {
		return Query{}, err
	}
	query.OfficeCode = strings.TrimSpace(query.OfficeCode)
	if query.OfficeCode == "" || len(query.OfficeCode) > 7 || strings.Contains(query.OfficeCode, ",") {
		return Query{}, ErrInvalidQuery
	}
	return query, nil
}

type Metadata struct {
	ComplaintID      string
	HistoryID        string
	DocumentTypeCode string
	DocumentType     string
	UploadOfficeCode string
	AddedAt          time.Time
	FileCount        int
}

type Repository interface {
	ListForOpenCallCenterTicket(context.Context, Query) ([]Metadata, error)
}

type UploadRequest struct {
	ComplaintID    string
	HistoryID      string
	DocumentType   string
	FileName       string
	ContentType    string
	IdempotencyKey string
	Content        []byte
}

type UploadCommand struct {
	UploadRequest
	OfficeCode  string
	Actor       string
	StorageKey  string
	MediaType   string
	Extension   string
	FileSize    int64
	SHA256      string
	DocumentID  string
	CreatedAt   time.Time
	RequestHash string
}

type UploadResult struct {
	DocumentID   string
	ComplaintID  string
	HistoryID    string
	DocumentType string
	UploadOffice string
	MediaType    string
	Extension    string
	FileSize     int64
	CreatedAt    time.Time
	StorageKey   string
	Replayed     bool
}

type UploadRepository interface {
	CreateCallCenterDocument(context.Context, UploadCommand) (UploadResult, error)
}

type DownloadRequest struct {
	ComplaintID string
	DocumentID  string
}

type DownloadMetadata struct {
	DocumentID string
	StorageKey string
	MediaType  string
	Extension  string
	FileSize   int64
	SHA256     string
}

type DownloadRepository interface {
	GetCallCenterDocument(context.Context, string, string, string) (DownloadMetadata, error)
}

type DownloadResult struct {
	Metadata DownloadMetadata
	Content  []byte
}

type DeleteRequest struct {
	ComplaintID string
	DocumentID  string
}

type DeleteResult struct {
	ComplaintID string
	DocumentID  string
	StorageKey  string
}

type DeleteRepository interface {
	DeleteCallCenterDocument(context.Context, string, string, string) (DeleteResult, error)
}

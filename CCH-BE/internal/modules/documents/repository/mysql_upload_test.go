package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/documents"
)

func TestMySQLRepositoryCreatesDocumentMetadataTransactionally(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := validUploadCommand()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(findDocumentByIdempotencySQL)).WithArgs(command.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{"document_id", "request_hash", "complaint_id", "history_id", "document_type", "office_code", "media_type", "file_extension", "file_size", "storage_key", "created_at"}))
	mock.ExpectQuery(regexp.QuoteMeta(lockCallCenterDocumentContextSQL)).
		WithArgs(command.HistoryID, command.DocumentType, command.ComplaintID, command.OfficeCode, command.OfficeCode, command.OfficeCode).
		WillReturnRows(sqlmock.NewRows([]string{"Kantor_Asal_Update"}).AddRow(command.OfficeCode))
	mock.ExpectExec(regexp.QuoteMeta(insertBEDocumentSQL)).
		WithArgs(command.DocumentID, command.ComplaintID, command.HistoryID, command.DocumentType,
			command.OfficeCode, command.Actor, command.MediaType, command.Extension, command.FileSize,
			command.SHA256, command.StorageKey, command.IdempotencyKey, command.RequestHash, command.CreatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := NewMySQLRepository(database).CreateCallCenterDocument(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if result.DocumentID != command.DocumentID || result.ComplaintID != command.ComplaintID || result.HistoryID != command.HistoryID || result.StorageKey != command.StorageKey || result.Replayed {
		t.Fatalf("upload result = %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryReplaysExactUploadByIdempotencyKey(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := validUploadCommand()
	createdAt := command.CreatedAt.Add(-time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(findDocumentByIdempotencySQL)).WithArgs(command.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{"document_id", "request_hash", "complaint_id", "history_id", "document_type", "office_code", "media_type", "file_extension", "file_size", "storage_key", "created_at"}).
			AddRow("abcdef0123456789abcdef0123456789", command.RequestHash, command.ComplaintID, command.HistoryID,
				command.DocumentType, command.OfficeCode, command.MediaType, command.Extension, command.FileSize, "other-storage-key", createdAt))
	mock.ExpectCommit()

	result, err := NewMySQLRepository(database).CreateCallCenterDocument(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.StorageKey != "other-storage-key" || result.CreatedAt != createdAt {
		t.Fatalf("replayed result = %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsChangedUploadReplayAndUnavailableContext(t *testing.T) {
	for _, test := range []struct {
		name         string
		existingHash string
		contextRows  *sqlmock.Rows
		wantError    error
	}{
		{name: "changed request", existingHash: "different-request-hash", wantError: documents.ErrUploadIdempotencyConflict},
		{name: "history or type unavailable", contextRows: sqlmock.NewRows([]string{"Kantor_Asal_Update"}), wantError: documents.ErrUploadNotAvailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			command := validUploadCommand()
			mock.ExpectBegin()
			if test.existingHash != "" {
				mock.ExpectQuery(regexp.QuoteMeta(findDocumentByIdempotencySQL)).WithArgs(command.IdempotencyKey).
					WillReturnRows(sqlmock.NewRows([]string{"document_id", "request_hash", "complaint_id", "history_id", "document_type", "office_code", "media_type", "file_extension", "file_size", "storage_key", "created_at"}).
						AddRow("abcdef0123456789abcdef0123456789", test.existingHash, command.ComplaintID, command.HistoryID,
							command.DocumentType, command.OfficeCode, command.MediaType, command.Extension, command.FileSize, command.StorageKey, command.CreatedAt))
			} else {
				mock.ExpectQuery(regexp.QuoteMeta(findDocumentByIdempotencySQL)).WithArgs(command.IdempotencyKey).
					WillReturnRows(sqlmock.NewRows([]string{"document_id", "request_hash", "complaint_id", "history_id", "document_type", "office_code", "media_type", "file_extension", "file_size", "storage_key", "created_at"}))
				mock.ExpectQuery(regexp.QuoteMeta(lockCallCenterDocumentContextSQL)).
					WillReturnRows(test.contextRows)
			}
			mock.ExpectRollback()
			_, err = NewMySQLRepository(database).CreateCallCenterDocument(context.Background(), command)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("CreateCallCenterDocument() error = %v, want %v", err, test.wantError)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMySQLRepositoryMarksCommitFailureAsUncertain(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := validUploadCommand()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(findDocumentByIdempotencySQL)).WithArgs(command.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{"document_id", "request_hash", "complaint_id", "history_id", "document_type", "office_code", "media_type", "file_extension", "file_size", "storage_key", "created_at"}))
	mock.ExpectQuery(regexp.QuoteMeta(lockCallCenterDocumentContextSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"Kantor_Asal_Update"}).AddRow(command.OfficeCode))
	mock.ExpectExec(regexp.QuoteMeta(insertBEDocumentSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errors.New("connection lost during commit"))

	_, err = NewMySQLRepository(database).CreateCallCenterDocument(context.Background(), command)
	if !errors.Is(err, documents.ErrUploadCommitUncertain) {
		t.Fatalf("CreateCallCenterDocument() error = %v, want uncertain commit", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func validUploadCommand() documents.UploadCommand {
	return documents.UploadCommand{
		UploadRequest: documents.UploadRequest{
			ComplaintID: "TKT-001", HistoryID: "HIST-001", DocumentType: "BA1", FileName: "report.pdf",
			IdempotencyKey: "upload-operation-key-0001", Content: []byte("%PDF-1.7\ncontent"),
		},
		OfficeCode: "40005CC", Actor: "agent.test", StorageKey: "storage-key",
		MediaType: "application/pdf", Extension: ".pdf", FileSize: 16, SHA256: "document-sha256", DocumentID: "0123456789abcdef0123456789abcdef",
		CreatedAt: time.Date(2026, time.October, 1, 10, 0, 0, 0, time.UTC), RequestHash: "request-hash",
	}
}

var _ = mysql.MySQLError{}

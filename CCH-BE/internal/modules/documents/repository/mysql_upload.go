package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/documents"
)

const (
	lockCallCenterDocumentContextSQL = `SELECT h.Kantor_Asal_Update
	FROM cchentri AS c
	INNER JOIN cchentrimas AS h ON h.ID_Pengaduan = c.ID_Pengaduan AND h.ID_Histori = ?
	INNER JOIN cch_jenis_dokumen AS t ON t.Kode_Jenis_Dokumen = ? AND t.Status_Aktif = 'Y'
	WHERE c.ID_Pengaduan = ? AND c.Status_Akhir <> '109'
		AND (c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0)
		AND h.Kantor_Asal_Update = ?
	FOR UPDATE`
	findDocumentByIdempotencySQL = `SELECT document_id, request_hash, complaint_id, history_id,
	document_type, office_code, media_type, file_extension, file_size, storage_key, created_at
	FROM cch_be_document_files WHERE idempotency_key = ? FOR UPDATE`
	insertBEDocumentSQL = `INSERT INTO cch_be_document_files
	(document_id, complaint_id, history_id, document_type, office_code, uploaded_by,
	 media_type, file_extension, file_size, sha256, storage_key, idempotency_key, request_hash, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
)

func (repository *MySQLRepository) CreateCallCenterDocument(ctx context.Context, command documents.UploadCommand) (documents.UploadResult, error) {
	if repository == nil || repository.db == nil {
		return documents.UploadResult{}, ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return documents.UploadResult{}, fmt.Errorf("begin document metadata transaction: %w", err)
	}
	defer transaction.Rollback()

	if result, err := loadUploadReplay(ctx, transaction, command); err == nil {
		if err := transaction.Commit(); err != nil {
			return documents.UploadResult{}, fmt.Errorf("finish document upload replay: %w", err)
		}
		return result, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return documents.UploadResult{}, err
	}
	var historyOffice string
	if err := transaction.QueryRowContext(ctx, lockCallCenterDocumentContextSQL,
		command.HistoryID, command.DocumentType, command.ComplaintID,
		command.OfficeCode, command.OfficeCode, command.OfficeCode,
	).Scan(&historyOffice); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return documents.UploadResult{}, documents.ErrUploadNotAvailable
		}
		return documents.UploadResult{}, fmt.Errorf("validate complaint history and document type: %w", err)
	}
	if historyOffice != command.OfficeCode {
		return documents.UploadResult{}, documents.ErrUploadNotAvailable
	}
	if _, err := transaction.ExecContext(ctx, insertBEDocumentSQL,
		command.DocumentID, command.ComplaintID, command.HistoryID, command.DocumentType,
		command.OfficeCode, command.Actor, command.MediaType, command.Extension, command.FileSize,
		command.SHA256, command.StorageKey, command.IdempotencyKey, command.RequestHash, command.CreatedAt,
	); err != nil {
		var mysqlError *mysql.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			result, replayErr := loadUploadReplay(ctx, transaction, command)
			if replayErr != nil {
				if errors.Is(replayErr, sql.ErrNoRows) {
					return documents.UploadResult{}, fmt.Errorf("insert BE document metadata: %w", err)
				}
				return documents.UploadResult{}, replayErr
			}
			if err := transaction.Commit(); err != nil {
				return documents.UploadResult{}, fmt.Errorf("finish concurrent document upload replay: %w", err)
			}
			return result, nil
		}
		return documents.UploadResult{}, fmt.Errorf("insert BE document metadata: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return documents.UploadResult{}, fmt.Errorf("%w: %v", documents.ErrUploadCommitUncertain, err)
	}
	return uploadResult(command, false), nil
}

func VerifyUploadSchema(ctx context.Context, database *sql.DB) error {
	if database == nil {
		return ErrRepositoryNotConfigured
	}
	rows, err := database.QueryContext(ctx, "SELECT document_id FROM cch_be_document_files LIMIT 0")
	if err != nil {
		return fmt.Errorf("verify document upload migration: %w: %v", documents.ErrUploadSchemaNotReady, err)
	}
	defer rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("verify document upload migration: %w: %v", documents.ErrUploadSchemaNotReady, err)
	}
	return nil
}

func loadUploadReplay(ctx context.Context, transaction *sql.Tx, command documents.UploadCommand) (documents.UploadResult, error) {
	var result documents.UploadResult
	var storedRequestHash string
	if err := transaction.QueryRowContext(ctx, findDocumentByIdempotencySQL, command.IdempotencyKey).Scan(
		&result.DocumentID, &storedRequestHash, &result.ComplaintID, &result.HistoryID,
		&result.DocumentType, &result.UploadOffice, &result.MediaType, &result.Extension, &result.FileSize,
		&result.StorageKey, &result.CreatedAt,
	); err != nil {
		return documents.UploadResult{}, err
	}
	if storedRequestHash != command.RequestHash {
		return documents.UploadResult{}, documents.ErrUploadIdempotencyConflict
	}
	result.Replayed = true
	return result, nil
}

func uploadResult(command documents.UploadCommand, replayed bool) documents.UploadResult {
	return documents.UploadResult{
		DocumentID: command.DocumentID, ComplaintID: command.ComplaintID, HistoryID: command.HistoryID,
		DocumentType: command.DocumentType, UploadOffice: command.OfficeCode, MediaType: command.MediaType,
		Extension: command.Extension, FileSize: command.FileSize, CreatedAt: command.CreatedAt, StorageKey: command.StorageKey, Replayed: replayed,
	}
}

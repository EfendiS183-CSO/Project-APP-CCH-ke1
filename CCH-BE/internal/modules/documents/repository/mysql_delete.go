package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cch.local/backend/internal/modules/documents"
)

const (
	deleteDocumentContextSQL = `SELECT d.storage_key, d.office_code
	FROM cch_be_document_files AS d
	INNER JOIN cchentri AS c ON c.ID_Pengaduan = d.complaint_id
	INNER JOIN cchentrimas AS h ON h.ID_Pengaduan = d.complaint_id AND h.ID_Histori = d.history_id
	WHERE d.document_id = ?
		AND d.complaint_id = ?
		AND c.Status_Akhir <> '109'
		AND (c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0)
	LIMIT 1
	FOR UPDATE`
	deleteBEDocumentSQL = `DELETE FROM cch_be_document_files WHERE document_id = ? AND complaint_id = ?`
)

func (repository *MySQLRepository) DeleteCallCenterDocument(ctx context.Context, complaintID string, documentID string, officeCode string) (documents.DeleteResult, error) {
	if repository == nil || repository.db == nil {
		return documents.DeleteResult{}, ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return documents.DeleteResult{}, fmt.Errorf("begin document deletion transaction: %w", err)
	}
	defer transaction.Rollback()

	var storageKey string
	var uploadOfficeCode string
	if err := transaction.QueryRowContext(ctx, deleteDocumentContextSQL,
		documentID, complaintID, officeCode, officeCode,
	).Scan(&storageKey, &uploadOfficeCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return documents.DeleteResult{}, sql.ErrNoRows
		}
		return documents.DeleteResult{}, fmt.Errorf("lock visible complaint document for deletion: %w", err)
	}
	if uploadOfficeCode != officeCode {
		return documents.DeleteResult{}, documents.ErrDeleteUnavailable
	}
	result, err := transaction.ExecContext(ctx, deleteBEDocumentSQL, documentID, complaintID)
	if err != nil {
		return documents.DeleteResult{}, fmt.Errorf("delete complaint document metadata: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return documents.DeleteResult{}, fmt.Errorf("read complaint document deletion result: %w", err)
	}
	if affected != 1 {
		return documents.DeleteResult{}, sql.ErrNoRows
	}
	if err := transaction.Commit(); err != nil {
		return documents.DeleteResult{}, fmt.Errorf("finish complaint document deletion: %w", err)
	}
	return documents.DeleteResult{ComplaintID: complaintID, DocumentID: documentID, StorageKey: storageKey}, nil
}

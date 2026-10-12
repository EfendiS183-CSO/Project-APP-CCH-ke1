package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cch.local/backend/internal/modules/documents"
)

const getCallCenterDocumentSQL = `SELECT
	d.document_id,
	d.storage_key,
	d.media_type,
	d.file_extension,
	d.file_size,
	d.sha256
FROM cch_be_document_files AS d
INNER JOIN cchentri AS c ON c.ID_Pengaduan = d.complaint_id
INNER JOIN cchentrimas AS h ON h.ID_Pengaduan = d.complaint_id AND h.ID_Histori = d.history_id
WHERE d.document_id = ?
	AND d.complaint_id = ?
	AND c.Status_Akhir <> '109'
	AND (c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0)
LIMIT 1`

func (repository *MySQLRepository) GetCallCenterDocument(ctx context.Context, complaintID string, documentID string, officeCode string) (documents.DownloadMetadata, error) {
	if repository == nil || repository.db == nil {
		return documents.DownloadMetadata{}, ErrRepositoryNotConfigured
	}
	var metadata documents.DownloadMetadata
	if err := repository.db.QueryRowContext(ctx, getCallCenterDocumentSQL,
		documentID, complaintID, officeCode, officeCode,
	).Scan(&metadata.DocumentID, &metadata.StorageKey, &metadata.MediaType, &metadata.Extension, &metadata.FileSize, &metadata.SHA256); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return documents.DownloadMetadata{}, sql.ErrNoRows
		}
		return documents.DownloadMetadata{}, fmt.Errorf("get visible complaint document: %w", err)
	}
	return metadata, nil
}

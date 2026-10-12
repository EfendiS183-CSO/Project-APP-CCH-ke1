package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cch.local/backend/internal/modules/documents"
)

const callCenterTicketDocumentsSQL = `SELECT
	c.ID_Pengaduan,
	d.ID_Histori,
	d.Jenis_Dokumen,
	d.Deskripsi_Jenis_Dokumen,
	d.Kantor_Upload,
	d.Tanggal_Tambah,
	d.File_Count
FROM cchentri AS c
LEFT JOIN (
	SELECT all_documents.* FROM (
		SELECT
			d.ID_Pengaduan,
			d.ID_Histori,
			d.Jenis_Dokumen,
			COALESCE(j.Deskripsi_Jenis_Dokumen, '') AS Deskripsi_Jenis_Dokumen,
			COALESCE(d.Kantor_Upload, '') AS Kantor_Upload,
			d.Tanggal_Tambah,
			CASE WHEN COALESCE(d.Nama_File, '') = '' THEN 0
				ELSE 1 + CHAR_LENGTH(d.Nama_File) - CHAR_LENGTH(REPLACE(d.Nama_File, ',', '')) END AS File_Count
		FROM cchentridok AS d
		INNER JOIN cchentrimas AS h
			ON h.ID_Pengaduan = d.ID_Pengaduan AND h.ID_Histori = d.ID_Histori
		LEFT JOIN cch_jenis_dokumen AS j ON j.Kode_Jenis_Dokumen = d.Jenis_Dokumen
		WHERE d.ID_Pengaduan = ?
		UNION ALL
		SELECT
			d.complaint_id AS ID_Pengaduan,
			d.history_id AS ID_Histori,
			d.document_type AS Jenis_Dokumen,
			COALESCE(j.Deskripsi_Jenis_Dokumen, '') AS Deskripsi_Jenis_Dokumen,
			d.office_code AS Kantor_Upload,
			MAX(d.created_at) AS Tanggal_Tambah,
			COUNT(*) AS File_Count
		FROM cch_be_document_files AS d
		INNER JOIN cchentrimas AS h
			ON h.ID_Pengaduan = d.complaint_id AND h.ID_Histori = d.history_id
		LEFT JOIN cch_jenis_dokumen AS j ON j.Kode_Jenis_Dokumen = d.document_type
		WHERE d.complaint_id = ?
		GROUP BY d.complaint_id, d.history_id, d.document_type, j.Deskripsi_Jenis_Dokumen, d.office_code
	) AS all_documents
	ORDER BY all_documents.Tanggal_Tambah DESC, all_documents.ID_Histori DESC, all_documents.Jenis_Dokumen ASC
	LIMIT ? OFFSET ?
) AS d ON d.ID_Pengaduan = c.ID_Pengaduan
WHERE c.ID_Pengaduan = ?
	AND c.Status_Akhir <> '109'
	AND (c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0)
ORDER BY d.Tanggal_Tambah DESC, d.ID_Histori DESC, d.Jenis_Dokumen ASC`

var ErrRepositoryNotConfigured = errors.New("document repository is not configured")

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: database}
}

func (repository *MySQLRepository) ListForOpenCallCenterTicket(ctx context.Context, query documents.Query) ([]documents.Metadata, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	rows, err := repository.db.QueryContext(ctx, callCenterTicketDocumentsSQL,
		query.ComplaintID, query.ComplaintID, query.Limit, query.Offset,
		query.ComplaintID, query.OfficeCode, query.OfficeCode,
	)
	if err != nil {
		return nil, fmt.Errorf("list complaint document metadata: %w", err)
	}
	defer rows.Close()
	metadata := make([]documents.Metadata, 0, query.Limit)
	parentFound := false
	for rows.Next() {
		parentFound = true
		var complaintID string
		var historyID sql.NullString
		var documentTypeCode sql.NullString
		var documentType sql.NullString
		var uploadOfficeCode sql.NullString
		var addedAt sql.NullTime
		var fileCount sql.NullInt64
		if err := rows.Scan(&complaintID, &historyID, &documentTypeCode, &documentType, &uploadOfficeCode, &addedAt, &fileCount); err != nil {
			return nil, fmt.Errorf("scan complaint document metadata: %w", err)
		}
		if !historyID.Valid {
			continue
		}
		item := documents.Metadata{
			ComplaintID: complaintID,
			HistoryID:   historyID.String,
		}
		if documentTypeCode.Valid {
			item.DocumentTypeCode = documentTypeCode.String
		}
		if documentType.Valid {
			item.DocumentType = documentType.String
		}
		if uploadOfficeCode.Valid {
			item.UploadOfficeCode = uploadOfficeCode.String
		}
		if addedAt.Valid {
			item.AddedAt = addedAt.Time
		}
		if fileCount.Valid && fileCount.Int64 > 0 {
			item.FileCount = int(fileCount.Int64)
		}
		metadata = append(metadata, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read complaint document metadata: %w", err)
	}
	if !parentFound {
		return nil, sql.ErrNoRows
	}
	return metadata, nil
}

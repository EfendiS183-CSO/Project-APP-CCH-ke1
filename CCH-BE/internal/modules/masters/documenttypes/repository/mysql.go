package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/documenttypes"
)

const listDocumentTypesSQL = `SELECT Kode_Jenis_Dokumen, Deskripsi_Jenis_Dokumen, Status_Aktif, Tanggal_Tambah, Ditambah_Oleh, Tanggal_Ubah, Diubah_Oleh
FROM cch_jenis_dokumen
ORDER BY Kode_Jenis_Dokumen ASC`
const getDocumentTypeSQL = `SELECT Kode_Jenis_Dokumen, Deskripsi_Jenis_Dokumen, Status_Aktif, Tanggal_Tambah, Ditambah_Oleh, Tanggal_Ubah, Diubah_Oleh
FROM cch_jenis_dokumen
WHERE Kode_Jenis_Dokumen = ?
LIMIT 1`
const insertDocumentTypeSQL = `INSERT INTO cch_jenis_dokumen
(Kode_Jenis_Dokumen, Deskripsi_Jenis_Dokumen, Status_Aktif, Tanggal_Tambah, Ditambah_Oleh, Tanggal_Ubah, Diubah_Oleh)
VALUES (?, ?, ?, ?, ?, ?, ?)`
const updateDocumentTypeSQL = `UPDATE cch_jenis_dokumen
SET Deskripsi_Jenis_Dokumen = ?, Status_Aktif = ?, Tanggal_Ubah = ?, Diubah_Oleh = ?
WHERE Kode_Jenis_Dokumen = ?`
const deactivateDocumentTypeSQL = `UPDATE cch_jenis_dokumen
SET Status_Aktif = 'N', Tanggal_Ubah = ?, Diubah_Oleh = ?
WHERE Kode_Jenis_Dokumen = ? AND (Status_Aktif IS NULL OR Status_Aktif <> 'N')`
const documentTypeExistsSQL = `SELECT Kode_Jenis_Dokumen FROM cch_jenis_dokumen WHERE Kode_Jenis_Dokumen = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("document type repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ documenttypes.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context) ([]documenttypes.DocumentType, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listDocumentTypesSQL)
	if err != nil {
		return nil, fmt.Errorf("list document types: %w", err)
	}
	defer rows.Close()
	values := make([]documenttypes.DocumentType, 0)
	for rows.Next() {
		value, err := scanDocumentType(rows)
		if err != nil {
			return nil, fmt.Errorf("scan document type: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read document types: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, code string) (documenttypes.DocumentType, error) {
	if repository == nil || repository.db == nil {
		return documenttypes.DocumentType{}, ErrRepositoryNotConfigured
	}
	value, err := scanDocumentType(repository.db.QueryRowContext(ctx, getDocumentTypeSQL, code))
	if err != nil {
		return documenttypes.DocumentType{}, fmt.Errorf("get document type: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value documenttypes.DocumentType) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if _, err := repository.db.ExecContext(ctx, insertDocumentTypeSQL, value.Code, value.Description, activeValue(value.Active), value.CreatedAt, value.CreatedBy, value.UpdatedAt, value.UpdatedBy); err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return documenttypes.ErrAlreadyExists
		}
		return fmt.Errorf("insert document type: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value documenttypes.DocumentType) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, updateDocumentTypeSQL, value.Description, activeValue(value.Active), value.UpdatedAt, value.UpdatedBy, value.Code)
	if err != nil {
		return fmt.Errorf("update document type: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check document type update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, value.Code)
}

func (repository *MySQLRepository) Deactivate(ctx context.Context, code string, actor string, updatedAt time.Time) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, deactivateDocumentTypeSQL, updatedAt, actor, code)
	if err != nil {
		return fmt.Errorf("deactivate document type: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check document type deactivation: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, code)
}

func (repository *MySQLRepository) checkExists(ctx context.Context, code string) error {
	var existingCode string
	err := repository.db.QueryRowContext(ctx, documentTypeExistsSQL, code).Scan(&existingCode)
	if errors.Is(err, sql.ErrNoRows) {
		return documenttypes.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check document type: %w", err)
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanDocumentType(row rowScanner) (documenttypes.DocumentType, error) {
	var value documenttypes.DocumentType
	var status, createdBy, updatedBy sql.NullString
	var createdAt, updatedAt sql.NullTime
	if err := row.Scan(&value.Code, &value.Description, &status, &createdAt, &createdBy, &updatedAt, &updatedBy); err != nil {
		return documenttypes.DocumentType{}, err
	}
	value.Active = status.Valid && status.String == "Y"
	if createdAt.Valid {
		value.CreatedAt = &createdAt.Time
	}
	if createdBy.Valid {
		value.CreatedBy = createdBy.String
	}
	if updatedAt.Valid {
		value.UpdatedAt = &updatedAt.Time
	}
	if updatedBy.Valid {
		value.UpdatedBy = updatedBy.String
	}
	return value, nil
}

func activeValue(active bool) string {
	if active {
		return "Y"
	}
	return "N"
}

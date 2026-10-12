package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/complainttypes"
)

const listComplaintTypesSQL = `SELECT Kode_Jenis, Deskripsi, Status_Aktif, Update_Terakhir
FROM cch_jenis_pengaduan
ORDER BY Kode_Jenis ASC`

const getComplaintTypeSQL = `SELECT Kode_Jenis, Deskripsi, Status_Aktif, Update_Terakhir
FROM cch_jenis_pengaduan
WHERE Kode_Jenis = ?
LIMIT 1`

const insertComplaintTypeSQL = `INSERT INTO cch_jenis_pengaduan
(Kode_Jenis, Deskripsi, Status_Aktif, Update_Terakhir)
VALUES (?, ?, ?, ?)`

const updateComplaintTypeSQL = `UPDATE cch_jenis_pengaduan
SET Deskripsi = ?, Status_Aktif = ?, Update_Terakhir = ?
WHERE Kode_Jenis = ?`

const deactivateComplaintTypeSQL = `UPDATE cch_jenis_pengaduan
SET Status_Aktif = 'N', Update_Terakhir = ?
WHERE Kode_Jenis = ? AND Status_Aktif <> 'N'`

const complaintTypeExistsSQL = `SELECT Kode_Jenis
FROM cch_jenis_pengaduan
WHERE Kode_Jenis = ?
LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("complaint type repository is not configured")

type MySQLRepository struct {
	db *sql.DB
}

var _ complainttypes.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: database}
}

func (repository *MySQLRepository) List(ctx context.Context) ([]complainttypes.ComplaintType, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listComplaintTypesSQL)
	if err != nil {
		return nil, fmt.Errorf("list complaint types: %w", err)
	}
	defer rows.Close()
	values := make([]complainttypes.ComplaintType, 0)
	for rows.Next() {
		value, err := scanComplaintType(rows)
		if err != nil {
			return nil, fmt.Errorf("scan complaint type: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read complaint types: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, code int64) (complainttypes.ComplaintType, error) {
	if repository == nil || repository.db == nil {
		return complainttypes.ComplaintType{}, ErrRepositoryNotConfigured
	}
	value, err := scanComplaintType(repository.db.QueryRowContext(ctx, getComplaintTypeSQL, code))
	if err != nil {
		return complainttypes.ComplaintType{}, fmt.Errorf("get complaint type: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value complainttypes.ComplaintType) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if _, err := repository.db.ExecContext(ctx, insertComplaintTypeSQL, value.Code, value.Description, activeValue(value.Active), value.UpdatedAt); err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return complainttypes.ErrAlreadyExists
		}
		return fmt.Errorf("insert complaint type: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value complainttypes.ComplaintType) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, updateComplaintTypeSQL, value.Description, activeValue(value.Active), value.UpdatedAt, value.Code)
	if err != nil {
		return fmt.Errorf("update complaint type: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check complaint type update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, value.Code)
}

func (repository *MySQLRepository) Deactivate(ctx context.Context, code int64, updatedAt time.Time) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, deactivateComplaintTypeSQL, updatedAt, code)
	if err != nil {
		return fmt.Errorf("deactivate complaint type: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check complaint type deactivation: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, code)
}

func (repository *MySQLRepository) checkExists(ctx context.Context, code int64) error {
	var existingCode int64
	err := repository.db.QueryRowContext(ctx, complaintTypeExistsSQL, code).Scan(&existingCode)
	if errors.Is(err, sql.ErrNoRows) {
		return complainttypes.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check complaint type: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanComplaintType(row rowScanner) (complainttypes.ComplaintType, error) {
	var value complainttypes.ComplaintType
	var description sql.NullString
	var status string
	var updatedAt sql.NullTime
	if err := row.Scan(&value.Code, &description, &status, &updatedAt); err != nil {
		return complainttypes.ComplaintType{}, err
	}
	if description.Valid {
		value.Description = &description.String
	}
	value.Active = status == "Y"
	if updatedAt.Valid {
		value.UpdatedAt = &updatedAt.Time
	}
	return value, nil
}

func activeValue(active bool) string {
	if active {
		return "Y"
	}
	return "N"
}

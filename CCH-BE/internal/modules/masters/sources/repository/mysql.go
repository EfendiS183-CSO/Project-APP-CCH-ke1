package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cch.local/backend/internal/modules/masters/sources"
)

const listSourcesSQL = `SELECT ID, Sumber, Apakah_Aktif, Tanggal_Tambah, Tanggal_Ubah
FROM cch_sumber_pengaduan
ORDER BY ID ASC`

const getSourceSQL = `SELECT ID, Sumber, Apakah_Aktif, Tanggal_Tambah, Tanggal_Ubah
FROM cch_sumber_pengaduan
WHERE ID = ?
LIMIT 1`

const insertSourceSQL = `INSERT INTO cch_sumber_pengaduan
(Sumber, Apakah_Aktif, Tanggal_Tambah)
VALUES (?, ?, ?)`

const updateSourceSQL = `UPDATE cch_sumber_pengaduan
SET Sumber = ?, Apakah_Aktif = ?, Tanggal_Ubah = ?
WHERE ID = ?`

const deactivateSourceSQL = `UPDATE cch_sumber_pengaduan
SET Apakah_Aktif = 'N', Tanggal_Ubah = ?
WHERE ID = ? AND Apakah_Aktif <> 'N'`

const sourceExistsSQL = `SELECT ID FROM cch_sumber_pengaduan WHERE ID = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("complaint source repository is not configured")

type MySQLRepository struct {
	db *sql.DB
}

var _ sources.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: database}
}

func (repository *MySQLRepository) List(ctx context.Context) ([]sources.Source, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listSourcesSQL)
	if err != nil {
		return nil, fmt.Errorf("list complaint sources: %w", err)
	}
	defer rows.Close()
	values := make([]sources.Source, 0)
	for rows.Next() {
		value, err := scanSource(rows)
		if err != nil {
			return nil, fmt.Errorf("scan complaint source: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read complaint sources: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, id int64) (sources.Source, error) {
	if repository == nil || repository.db == nil {
		return sources.Source{}, ErrRepositoryNotConfigured
	}
	value, err := scanSource(repository.db.QueryRowContext(ctx, getSourceSQL, id))
	if err != nil {
		return sources.Source{}, fmt.Errorf("get complaint source: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, source sources.Source) (sources.Source, error) {
	if repository == nil || repository.db == nil {
		return sources.Source{}, ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, insertSourceSQL, source.Name, activeValue(source.Active), source.CreatedAt)
	if err != nil {
		return sources.Source{}, fmt.Errorf("insert complaint source: %w", err)
	}
	if source.ID, err = result.LastInsertId(); err != nil {
		return sources.Source{}, fmt.Errorf("read complaint source ID: %w", err)
	}
	return source, nil
}

func (repository *MySQLRepository) Update(ctx context.Context, source sources.Source) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, updateSourceSQL, source.Name, activeValue(source.Active), source.UpdatedAt, source.ID)
	if err != nil {
		return fmt.Errorf("update complaint source: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check complaint source update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, source.ID)
}

func (repository *MySQLRepository) Deactivate(ctx context.Context, id int64, updatedAt time.Time) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, deactivateSourceSQL, updatedAt, id)
	if err != nil {
		return fmt.Errorf("deactivate complaint source: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check complaint source deactivation: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, id)
}

func (repository *MySQLRepository) checkExists(ctx context.Context, id int64) error {
	var existingID int64
	err := repository.db.QueryRowContext(ctx, sourceExistsSQL, id).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return sources.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check complaint source: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanSource(row rowScanner) (sources.Source, error) {
	var value sources.Source
	var active string
	var createdAt sql.NullTime
	var updatedAt sql.NullTime
	if err := row.Scan(&value.ID, &value.Name, &active, &createdAt, &updatedAt); err != nil {
		return sources.Source{}, err
	}
	value.Active = active == "Y"
	if createdAt.Valid {
		value.CreatedAt = &createdAt.Time
	}
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

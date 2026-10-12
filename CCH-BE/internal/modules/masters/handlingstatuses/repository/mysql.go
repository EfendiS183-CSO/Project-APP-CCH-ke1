package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/handlingstatuses"
)

const listStatusesSQL = `SELECT ID_Status, Deskripsi_Status, Aktif FROM cch_jenis_penanganan ORDER BY ID_Status ASC`
const getStatusSQL = `SELECT ID_Status, Deskripsi_Status, Aktif FROM cch_jenis_penanganan WHERE ID_Status = ? LIMIT 1`
const insertStatusSQL = `INSERT INTO cch_jenis_penanganan (ID_Status, Deskripsi_Status, Aktif) VALUES (?, ?, ?)`
const updateStatusSQL = `UPDATE cch_jenis_penanganan SET Deskripsi_Status = ?, Aktif = ? WHERE ID_Status = ?`
const deactivateStatusSQL = `UPDATE cch_jenis_penanganan SET Aktif = 'N' WHERE ID_Status = ? AND (Aktif IS NULL OR Aktif <> 'N')`
const statusExistsSQL = `SELECT ID_Status FROM cch_jenis_penanganan WHERE ID_Status = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("handling status repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ handlingstatuses.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context) ([]handlingstatuses.Status, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listStatusesSQL)
	if err != nil {
		return nil, fmt.Errorf("list handling statuses: %w", err)
	}
	defer rows.Close()
	values := make([]handlingstatuses.Status, 0)
	for rows.Next() {
		value, err := scanStatus(rows)
		if err != nil {
			return nil, fmt.Errorf("scan handling status: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read handling statuses: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, code string) (handlingstatuses.Status, error) {
	if repository == nil || repository.db == nil {
		return handlingstatuses.Status{}, ErrRepositoryNotConfigured
	}
	value, err := scanStatus(repository.db.QueryRowContext(ctx, getStatusSQL, code))
	if err != nil {
		return handlingstatuses.Status{}, fmt.Errorf("get handling status: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value handlingstatuses.Status) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if _, err := repository.db.ExecContext(ctx, insertStatusSQL, value.Code, value.Description, activeValue(value.Active)); err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return handlingstatuses.ErrAlreadyExists
		}
		return fmt.Errorf("insert handling status: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value handlingstatuses.Status) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, updateStatusSQL, value.Description, activeValue(value.Active), value.Code)
	if err != nil {
		return fmt.Errorf("update handling status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check handling status update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, value.Code)
}

func (repository *MySQLRepository) Deactivate(ctx context.Context, code string) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, deactivateStatusSQL, code)
	if err != nil {
		return fmt.Errorf("deactivate handling status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check handling status deactivation: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, code)
}

func (repository *MySQLRepository) checkExists(ctx context.Context, code string) error {
	var existingCode string
	err := repository.db.QueryRowContext(ctx, statusExistsSQL, code).Scan(&existingCode)
	if errors.Is(err, sql.ErrNoRows) {
		return handlingstatuses.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check handling status: %w", err)
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanStatus(row rowScanner) (handlingstatuses.Status, error) {
	var value handlingstatuses.Status
	var active sql.NullString
	if err := row.Scan(&value.Code, &value.Description, &active); err != nil {
		return handlingstatuses.Status{}, err
	}
	value.Active = active.Valid && active.String == "Y"
	return value, nil
}

func activeValue(active bool) string {
	if active {
		return "Y"
	}
	return "N"
}

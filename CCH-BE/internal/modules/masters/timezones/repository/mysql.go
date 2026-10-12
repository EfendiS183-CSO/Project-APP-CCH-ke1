package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/timezones"
)

const listTimezonesSQL = `SELECT ID, Timezone, Default FROM cch_timezone ORDER BY Timezone ASC`
const getTimezoneSQL = `SELECT ID, Timezone, Default FROM cch_timezone WHERE ID = ? LIMIT 1`
const insertTimezoneSQL = `INSERT INTO cch_timezone (Timezone, Default) VALUES (?, ?)`
const defaultTimezoneExistsSQL = `SELECT ID FROM cch_timezone WHERE Default = 'Y' AND ID <> ? LIMIT 1`
const updateTimezoneSQL = `UPDATE cch_timezone SET Timezone = ?, Default = ? WHERE ID = ?`
const deleteTimezoneSQL = `DELETE FROM cch_timezone WHERE ID = ? AND Default = 'N'`
const getDefaultTimezoneExistsSQL = `SELECT ID FROM cch_timezone WHERE ID = ? AND Default = 'Y' LIMIT 1`
const timezoneExistsSQL = `SELECT ID FROM cch_timezone WHERE ID = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("timezone repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ timezones.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context) ([]timezones.Timezone, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listTimezonesSQL)
	if err != nil {
		return nil, fmt.Errorf("list timezones: %w", err)
	}
	defer rows.Close()
	values := make([]timezones.Timezone, 0)
	for rows.Next() {
		value, err := scanTimezone(rows)
		if err != nil {
			return nil, fmt.Errorf("scan timezone: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read timezones: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, id int64) (timezones.Timezone, error) {
	if repository == nil || repository.db == nil {
		return timezones.Timezone{}, ErrRepositoryNotConfigured
	}
	value, err := scanTimezone(repository.db.QueryRowContext(ctx, getTimezoneSQL, id))
	if err != nil {
		return timezones.Timezone{}, fmt.Errorf("get timezone: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value timezones.Timezone) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if err := repository.checkDuplicates(ctx, value); err != nil {
		return err
	}
	if _, err := repository.db.ExecContext(ctx, insertTimezoneSQL, value.Timezone, boolToEnum(value.IsDefault)); err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return timezones.ErrDuplicate
		}
		return fmt.Errorf("insert timezone: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value timezones.Timezone) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if err := repository.checkDuplicates(ctx, value); err != nil {
		return err
	}
	result, err := repository.db.ExecContext(ctx, updateTimezoneSQL, value.Timezone, boolToEnum(value.IsDefault), value.ID)
	if err != nil {
		return fmt.Errorf("update timezone: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check timezone update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, value.ID)
}

func (repository *MySQLRepository) Delete(ctx context.Context, id int64) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	// Guard: deleting the active default is forbidden by the legacy UI convention.
	var defaultID int64
	if err := repository.db.QueryRowContext(ctx, getDefaultTimezoneExistsSQL, id).Scan(&defaultID); err == nil {
		return timezones.ErrCannotDeleteDefault
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check default timezone: %w", err)
	}
	result, err := repository.db.ExecContext(ctx, deleteTimezoneSQL, id)
	if err != nil {
		return fmt.Errorf("delete timezone: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check timezone delete: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, id)
}

func (repository *MySQLRepository) checkExists(ctx context.Context, id int64) error {
	var existingID int64
	err := repository.db.QueryRowContext(ctx, timezoneExistsSQL, id).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return timezones.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check timezone: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) checkDuplicates(ctx context.Context, value timezones.Timezone) error {
	var defaultID int64
	err := repository.db.QueryRowContext(ctx, defaultTimezoneExistsSQL, value.ID).Scan(&defaultID)
	if err == nil {
		return timezones.ErrOnlyOneDefault
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check timezone default: %w", err)
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanTimezone(row rowScanner) (timezones.Timezone, error) {
	var value timezones.Timezone
	var isDefault string
	if err := row.Scan(&value.ID, &value.Timezone, &isDefault); err != nil {
		return timezones.Timezone{}, err
	}
	value.IsDefault = isDefault == "Y"
	return value, nil
}

func boolToEnum(enabled bool) string {
	if enabled {
		return "Y"
	}
	return "N"
}

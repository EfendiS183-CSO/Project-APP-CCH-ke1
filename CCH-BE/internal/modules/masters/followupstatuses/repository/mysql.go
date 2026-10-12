package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/followupstatuses"
)

const listFollowUpStatusesSQL = `SELECT Kode_FollowUp, Deskripsi_FollowUp FROM cchfust ORDER BY Kode_FollowUp ASC`
const getFollowUpStatusSQL = `SELECT Kode_FollowUp, Deskripsi_FollowUp FROM cchfust WHERE Kode_FollowUp = ? LIMIT 1`
const insertFollowUpStatusSQL = `INSERT INTO cchfust (Kode_FollowUp, Deskripsi_FollowUp) VALUES (?, ?)`
const updateFollowUpStatusSQL = `UPDATE cchfust SET Deskripsi_FollowUp = ? WHERE Kode_FollowUp = ?`
const lockFollowUpStatusSQL = `SELECT Kode_FollowUp FROM cchfust WHERE Kode_FollowUp = ? FOR UPDATE`
const followUpStatusInUseSQL = `SELECT ID_Pengaduan FROM cchfu WHERE Status = ? LIMIT 1`
const deleteFollowUpStatusSQL = `DELETE FROM cchfust WHERE Kode_FollowUp = ?`
const followUpStatusExistsSQL = `SELECT Kode_FollowUp FROM cchfust WHERE Kode_FollowUp = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("follow-up status repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ followupstatuses.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context) ([]followupstatuses.Status, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listFollowUpStatusesSQL)
	if err != nil {
		return nil, fmt.Errorf("list follow-up statuses: %w", err)
	}
	defer rows.Close()
	values := make([]followupstatuses.Status, 0)
	for rows.Next() {
		value, err := scanStatus(rows)
		if err != nil {
			return nil, fmt.Errorf("scan follow-up status: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read follow-up statuses: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) LookupOptions(ctx context.Context) ([]followupstatuses.Status, error) {
	return repository.List(ctx)
}

func (repository *MySQLRepository) Get(ctx context.Context, code string) (followupstatuses.Status, error) {
	if repository == nil || repository.db == nil {
		return followupstatuses.Status{}, ErrRepositoryNotConfigured
	}
	value, err := scanStatus(repository.db.QueryRowContext(ctx, getFollowUpStatusSQL, code))
	if err != nil {
		return followupstatuses.Status{}, fmt.Errorf("get follow-up status: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value followupstatuses.Status) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if _, err := repository.db.ExecContext(ctx, insertFollowUpStatusSQL, value.Code, value.Description); err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return followupstatuses.ErrAlreadyExists
		}
		return fmt.Errorf("insert follow-up status: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value followupstatuses.Status) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, updateFollowUpStatusSQL, value.Description, value.Code)
	if err != nil {
		return fmt.Errorf("update follow-up status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check follow-up status update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, value.Code)
}

func (repository *MySQLRepository) Delete(ctx context.Context, code string) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin follow-up status delete: %w", err)
	}
	defer transaction.Rollback()
	var existingCode string
	if err := transaction.QueryRowContext(ctx, lockFollowUpStatusSQL, code).Scan(&existingCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return followupstatuses.ErrNotFound
		}
		return fmt.Errorf("lock follow-up status: %w", err)
	}
	var complaintID string
	if err := transaction.QueryRowContext(ctx, followUpStatusInUseSQL, code).Scan(&complaintID); err == nil {
		return followupstatuses.ErrInUse
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check follow-up status references: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, deleteFollowUpStatusSQL, code); err != nil {
		return fmt.Errorf("delete follow-up status: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit follow-up status delete: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) checkExists(ctx context.Context, code string) error {
	var existingCode string
	err := repository.db.QueryRowContext(ctx, followUpStatusExistsSQL, code).Scan(&existingCode)
	if errors.Is(err, sql.ErrNoRows) {
		return followupstatuses.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check follow-up status: %w", err)
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanStatus(row rowScanner) (followupstatuses.Status, error) {
	var value followupstatuses.Status
	var description sql.NullString
	if err := row.Scan(&value.Code, &description); err != nil {
		return followupstatuses.Status{}, err
	}
	if description.Valid {
		value.Description = &description.String
	}
	return value, nil
}

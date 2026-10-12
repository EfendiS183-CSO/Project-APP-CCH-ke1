package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cch.local/backend/internal/modules/regionals"
)

const countRegionalOfficesSQL = `SELECT COUNT(*) FROM cch_regional`
const listRegionalOfficesSQL = `SELECT Kode_Kantor, Nama_Kantor, Singkatan, Jenis, Regional, Apakah_Aktif
FROM cch_regional ORDER BY Singkatan ASC, Kode_Kantor ASC LIMIT ? OFFSET ?`
const getRegionalOfficeSQL = `SELECT Kode_Kantor, Nama_Kantor, Singkatan, Jenis, Regional, Apakah_Aktif
FROM cch_regional WHERE Kode_Kantor = ? LIMIT 1`
const listRegionalLookupSQL = `SELECT Kode_Kantor, Nama_Kantor, Jenis FROM cch_kantor ORDER BY Kode_Kantor ASC`
const lockRegionalSourceSQL = `SELECT Kode_Kantor FROM cch_kantor
WHERE Kode_Kantor = ? AND Jenis IN ('Divre', 'KP') FOR UPDATE`
const lockAssignedOfficeSQL = `SELECT Kode_Kantor FROM cch_kantor WHERE Kode_Kantor = ? LIMIT 1 FOR UPDATE`
const updateRegionalSourceSQL = `UPDATE cch_kantor
SET Nama_Kantor = ?, Singkatan = ?, Jenis = ?, Regional = ?, Apakah_Aktif = ?
WHERE Kode_Kantor = ? AND Jenis IN ('Divre', 'KP')`

var ErrRepositoryNotConfigured = errors.New("regional office repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ regionals.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context, query regionals.ListQuery) (regionals.Page, error) {
	if repository == nil || repository.db == nil {
		return regionals.Page{}, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return regionals.Page{}, err
	}
	var total int64
	if err := repository.db.QueryRowContext(ctx, countRegionalOfficesSQL).Scan(&total); err != nil {
		return regionals.Page{}, fmt.Errorf("count regional offices: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, listRegionalOfficesSQL, query.Limit, query.Offset)
	if err != nil {
		return regionals.Page{}, fmt.Errorf("list regional offices: %w", err)
	}
	defer rows.Close()
	values := make([]regionals.Office, 0, query.Limit)
	for rows.Next() {
		value, err := scanOffice(rows)
		if err != nil {
			return regionals.Page{}, fmt.Errorf("scan regional office: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return regionals.Page{}, fmt.Errorf("read regional offices: %w", err)
	}
	return regionals.Page{Data: values, Total: total, Limit: query.Limit, Offset: query.Offset}, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, code string) (regionals.Office, error) {
	if repository == nil || repository.db == nil {
		return regionals.Office{}, ErrRepositoryNotConfigured
	}
	value, err := scanOffice(repository.db.QueryRowContext(ctx, getRegionalOfficeSQL, code))
	if err != nil {
		return regionals.Office{}, fmt.Errorf("get regional office: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) LookupOptions(ctx context.Context) ([]regionals.OfficeOption, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listRegionalLookupSQL)
	if err != nil {
		return nil, fmt.Errorf("list regional office lookup options: %w", err)
	}
	defer rows.Close()
	options := make([]regionals.OfficeOption, 0)
	for rows.Next() {
		var option regionals.OfficeOption
		if err := rows.Scan(&option.Code, &option.Name, &option.Type); err != nil {
			return nil, fmt.Errorf("scan regional office lookup option: %w", err)
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read regional office lookup options: %w", err)
	}
	return options, nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value regionals.Office) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin regional office update: %w", err)
	}
	defer transaction.Rollback()

	var lockedCode string
	if err := transaction.QueryRowContext(ctx, lockRegionalSourceSQL, value.Code).Scan(&lockedCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return regionals.ErrNotFound
		}
		return fmt.Errorf("lock regional office: %w", err)
	}
	var parentCode string
	if err := transaction.QueryRowContext(ctx, lockAssignedOfficeSQL, value.RegionalCode).Scan(&parentCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return regionals.ErrInvalidRegional
		}
		return fmt.Errorf("validate regional assignment: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, updateRegionalSourceSQL,
		value.Name, value.Abbreviation, value.Type, value.RegionalCode, yn(value.Active), value.Code); err != nil {
		return fmt.Errorf("update regional office source row: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit regional office update: %w", err)
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanOffice(row rowScanner) (regionals.Office, error) {
	var value regionals.Office
	var active string
	if err := row.Scan(&value.Code, &value.Name, &value.Abbreviation, &value.Type, &value.RegionalCode, &active); err != nil {
		return regionals.Office{}, err
	}
	value.Active = active == "Y"
	return value, nil
}

func yn(value bool) string {
	if value {
		return "Y"
	}
	return "N"
}

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/holidaycalendar"
)

const listHolidaysSQL = `SELECT Tanggal, Keterangan, Tanggal_Tambah, Ditambah_Oleh
FROM cch_kalender_libur
ORDER BY Tanggal ASC`

const getHolidaySQL = `SELECT Tanggal, Keterangan, Tanggal_Tambah, Ditambah_Oleh
FROM cch_kalender_libur
WHERE Tanggal = ?
LIMIT 1`

const insertHolidaySQL = `INSERT INTO cch_kalender_libur
(Tanggal, Keterangan, Tanggal_Tambah, Ditambah_Oleh)
VALUES (?, ?, ?, ?)`

const updateHolidaySQL = `UPDATE cch_kalender_libur
SET Keterangan = ?
WHERE Tanggal = ?`

const deleteHolidaySQL = `DELETE FROM cch_kalender_libur WHERE Tanggal = ?`

const holidayExistsSQL = `SELECT Tanggal FROM cch_kalender_libur WHERE Tanggal = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("holiday calendar repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ holidaycalendar.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context) ([]holidaycalendar.Holiday, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listHolidaysSQL)
	if err != nil {
		return nil, fmt.Errorf("list holiday calendar: %w", err)
	}
	defer rows.Close()
	values := make([]holidaycalendar.Holiday, 0)
	for rows.Next() {
		value, err := scanHoliday(rows)
		if err != nil {
			return nil, fmt.Errorf("scan holiday calendar: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read holiday calendar: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, date string) (holidaycalendar.Holiday, error) {
	if repository == nil || repository.db == nil {
		return holidaycalendar.Holiday{}, ErrRepositoryNotConfigured
	}
	value, err := scanHoliday(repository.db.QueryRowContext(ctx, getHolidaySQL, date))
	if err != nil {
		return holidaycalendar.Holiday{}, fmt.Errorf("get holiday calendar: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value holidaycalendar.Holiday) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if _, err := repository.db.ExecContext(ctx, insertHolidaySQL, value.Date, value.Description, value.CreatedAt, value.CreatedBy); err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return holidaycalendar.ErrAlreadyExists
		}
		return fmt.Errorf("insert holiday calendar: %w", err)
	}
	return nil
}

// Update changes only the description. The legacy DATE primary key is treated
// as immutable so historical references and calendar ordering stay stable.
func (repository *MySQLRepository) Update(ctx context.Context, value holidaycalendar.Holiday) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, updateHolidaySQL, value.Description, value.Date)
	if err != nil {
		return fmt.Errorf("update holiday calendar: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check holiday calendar update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, value.Date)
}

func (repository *MySQLRepository) Delete(ctx context.Context, date string) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, deleteHolidaySQL, date)
	if err != nil {
		return fmt.Errorf("delete holiday calendar: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check holiday calendar delete: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, date)
}

func (repository *MySQLRepository) checkExists(ctx context.Context, date string) error {
	var existingDate time.Time
	err := repository.db.QueryRowContext(ctx, holidayExistsSQL, date).Scan(&existingDate)
	if errors.Is(err, sql.ErrNoRows) {
		return holidaycalendar.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check holiday calendar: %w", err)
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanHoliday(row rowScanner) (holidaycalendar.Holiday, error) {
	var value holidaycalendar.Holiday
	var date time.Time
	var createdAt sql.NullTime
	var createdBy sql.NullString
	if err := row.Scan(&date, &value.Description, &createdAt, &createdBy); err != nil {
		return holidaycalendar.Holiday{}, err
	}
	value.Date = date.Format(holidaycalendar.DateLayout)
	if createdAt.Valid {
		value.CreatedAt = &createdAt.Time
	}
	if createdBy.Valid {
		value.CreatedBy = createdBy.String
	}
	return value, nil
}

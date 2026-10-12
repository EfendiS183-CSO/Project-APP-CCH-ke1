package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/shippingservices"
)

const listShippingServicesSQL = `SELECT Kode_Layanan, Deskripsi, URL_Lacak, Waktu_Akhir, Status_Aktif
FROM cch_jenis_layanan
ORDER BY Kode_Layanan ASC`

const getShippingServiceSQL = `SELECT Kode_Layanan, Deskripsi, URL_Lacak, Waktu_Akhir, Status_Aktif
FROM cch_jenis_layanan
WHERE Kode_Layanan = ?
LIMIT 1`

const insertShippingServiceSQL = `INSERT INTO cch_jenis_layanan
(Kode_Layanan, Deskripsi, URL_Lacak, Waktu_Akhir, Status_Aktif)
VALUES (?, ?, ?, ?, ?)`

const updateShippingServiceSQL = `UPDATE cch_jenis_layanan
SET Deskripsi = ?, URL_Lacak = ?, Waktu_Akhir = ?, Status_Aktif = ?
WHERE Kode_Layanan = ?`

const deactivateShippingServiceSQL = `UPDATE cch_jenis_layanan
SET Status_Aktif = 'N', Waktu_Akhir = ?
WHERE Kode_Layanan = ? AND Status_Aktif <> 'N'`

const shippingServiceExistsSQL = `SELECT Kode_Layanan FROM cch_jenis_layanan WHERE Kode_Layanan = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("shipping service repository is not configured")

type MySQLRepository struct {
	db *sql.DB
}

var _ shippingservices.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: database}
}

func (repository *MySQLRepository) List(ctx context.Context) ([]shippingservices.ShippingService, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listShippingServicesSQL)
	if err != nil {
		return nil, fmt.Errorf("list shipping services: %w", err)
	}
	defer rows.Close()
	values := make([]shippingservices.ShippingService, 0)
	for rows.Next() {
		value, err := scanShippingService(rows)
		if err != nil {
			return nil, fmt.Errorf("scan shipping service: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read shipping services: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, code string) (shippingservices.ShippingService, error) {
	if repository == nil || repository.db == nil {
		return shippingservices.ShippingService{}, ErrRepositoryNotConfigured
	}
	value, err := scanShippingService(repository.db.QueryRowContext(ctx, getShippingServiceSQL, code))
	if err != nil {
		return shippingservices.ShippingService{}, fmt.Errorf("get shipping service: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value shippingservices.ShippingService) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if _, err := repository.db.ExecContext(ctx, insertShippingServiceSQL,
		value.Code, value.Description, value.TrackingURL, value.UpdatedAt, activeValue(value.Active)); err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return shippingservices.ErrAlreadyExists
		}
		return fmt.Errorf("insert shipping service: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value shippingservices.ShippingService) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, updateShippingServiceSQL,
		value.Description, value.TrackingURL, value.UpdatedAt, activeValue(value.Active), value.Code)
	if err != nil {
		return fmt.Errorf("update shipping service: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check shipping service update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, value.Code)
}

func (repository *MySQLRepository) Deactivate(ctx context.Context, code string, updatedAt time.Time) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, deactivateShippingServiceSQL, updatedAt, code)
	if err != nil {
		return fmt.Errorf("deactivate shipping service: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check shipping service deactivation: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, code)
}

func (repository *MySQLRepository) checkExists(ctx context.Context, code string) error {
	var existingCode string
	err := repository.db.QueryRowContext(ctx, shippingServiceExistsSQL, code).Scan(&existingCode)
	if errors.Is(err, sql.ErrNoRows) {
		return shippingservices.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check shipping service: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanShippingService(row rowScanner) (shippingservices.ShippingService, error) {
	var value shippingservices.ShippingService
	var updatedAt sql.NullTime
	var status sql.NullString
	if err := row.Scan(&value.Code, &value.Description, &value.TrackingURL, &updatedAt, &status); err != nil {
		return shippingservices.ShippingService{}, err
	}
	value.Active = status.Valid && status.String == "Y"
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

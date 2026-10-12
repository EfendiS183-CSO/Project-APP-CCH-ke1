package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/offices"
)

const listOfficesSQL = `SELECT Kode_Kantor, Nama_Kantor, Singkatan, Jenis, Regional, Apakah_Aktif, Sebagai_Tujuan, Sebagai_Tembusan
FROM cch_kantor ORDER BY Kode_Kantor ASC`
const getOfficeSQL = `SELECT Kode_Kantor, Nama_Kantor, Singkatan, Jenis, Regional, Apakah_Aktif, Sebagai_Tujuan, Sebagai_Tembusan
FROM cch_kantor WHERE Kode_Kantor = ? LIMIT 1`
const listOfficeTypesSQL = `SELECT DISTINCT Jenis FROM cch_kantor ORDER BY Jenis ASC`
const listRegionalOfficesSQL = `SELECT Kode_Kantor, Nama_Kantor, Jenis FROM cch_kantor
WHERE Apakah_Aktif = 'Y' AND Jenis IN ('KP', 'Divre') AND CHAR_LENGTH(Kode_Kantor) <= 5 ORDER BY Kode_Kantor ASC`
const officeTypeExistsSQL = `SELECT 1 FROM cch_kantor WHERE Jenis = ? LIMIT 1`
const regionalOfficeExistsSQL = `SELECT 1 FROM cch_kantor
WHERE Kode_Kantor = ? AND Apakah_Aktif = 'Y' AND Jenis IN ('KP', 'Divre') AND CHAR_LENGTH(Kode_Kantor) <= 5 LIMIT 1 FOR UPDATE`
const insertOfficeSQL = `INSERT INTO cch_kantor
(Kode_Kantor, Nama_Kantor, Singkatan, Jenis, Regional, Apakah_Aktif, Sebagai_Tujuan, Sebagai_Tembusan)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
const updateOfficeSQL = `UPDATE cch_kantor SET Nama_Kantor = ?, Singkatan = ?, Jenis = ?, Regional = ?,
Apakah_Aktif = ?, Sebagai_Tujuan = ?, Sebagai_Tembusan = ? WHERE Kode_Kantor = ?`
const deactivateOfficeSQL = `UPDATE cch_kantor SET Apakah_Aktif = 'N' WHERE Kode_Kantor = ? AND Apakah_Aktif <> 'N'`
const lockOfficeSQL = `SELECT Kode_Kantor FROM cch_kantor WHERE Kode_Kantor = ? FOR UPDATE`
const officeExistsSQL = `SELECT Kode_Kantor FROM cch_kantor WHERE Kode_Kantor = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("office repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ offices.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context) ([]offices.Office, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listOfficesSQL)
	if err != nil {
		return nil, fmt.Errorf("list offices: %w", err)
	}
	defer rows.Close()
	values := make([]offices.Office, 0)
	for rows.Next() {
		value, err := scanOffice(rows)
		if err != nil {
			return nil, fmt.Errorf("scan office: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read offices: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, code string) (offices.Office, error) {
	if repository == nil || repository.db == nil {
		return offices.Office{}, ErrRepositoryNotConfigured
	}
	value, err := scanOffice(repository.db.QueryRowContext(ctx, getOfficeSQL, code))
	if err != nil {
		return offices.Office{}, fmt.Errorf("get office: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) LookupOptions(ctx context.Context) (offices.LookupOptions, error) {
	if repository == nil || repository.db == nil {
		return offices.LookupOptions{}, ErrRepositoryNotConfigured
	}
	options := offices.LookupOptions{Types: make([]string, 0), RegionalOffices: make([]offices.OfficeOption, 0)}
	typeRows, err := repository.db.QueryContext(ctx, listOfficeTypesSQL)
	if err != nil {
		return offices.LookupOptions{}, fmt.Errorf("list office types: %w", err)
	}
	for typeRows.Next() {
		var officeType string
		if err := typeRows.Scan(&officeType); err != nil {
			typeRows.Close()
			return offices.LookupOptions{}, fmt.Errorf("scan office type: %w", err)
		}
		options.Types = append(options.Types, officeType)
	}
	if err := typeRows.Err(); err != nil {
		typeRows.Close()
		return offices.LookupOptions{}, fmt.Errorf("read office types: %w", err)
	}
	if err := typeRows.Close(); err != nil {
		return offices.LookupOptions{}, fmt.Errorf("close office type rows: %w", err)
	}
	regionalRows, err := repository.db.QueryContext(ctx, listRegionalOfficesSQL)
	if err != nil {
		return offices.LookupOptions{}, fmt.Errorf("list regional offices: %w", err)
	}
	defer regionalRows.Close()
	for regionalRows.Next() {
		var option offices.OfficeOption
		if err := regionalRows.Scan(&option.Code, &option.Name, &option.Type); err != nil {
			return offices.LookupOptions{}, fmt.Errorf("scan regional office: %w", err)
		}
		options.RegionalOffices = append(options.RegionalOffices, option)
	}
	if err := regionalRows.Err(); err != nil {
		return offices.LookupOptions{}, fmt.Errorf("read regional offices: %w", err)
	}
	return options, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value offices.Office) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin office insert: %w", err)
	}
	defer transaction.Rollback()
	if err := validateOfficeReferences(ctx, transaction, value); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, insertOfficeSQL, value.Code, value.Name, value.Abbreviation, value.Type, value.RegionalCode, yn(value.Active), yn(value.AsDestination), yn(value.AsCopy)); err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return offices.ErrAlreadyExists
		}
		return fmt.Errorf("insert office: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit office insert: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value offices.Office) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin office update: %w", err)
	}
	defer transaction.Rollback()
	var existingCode string
	if err := transaction.QueryRowContext(ctx, lockOfficeSQL, value.Code).Scan(&existingCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return offices.ErrNotFound
		}
		return fmt.Errorf("lock office for update: %w", err)
	}
	if err := validateOfficeReferences(ctx, transaction, value); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, updateOfficeSQL, value.Name, value.Abbreviation, value.Type, value.RegionalCode, yn(value.Active), yn(value.AsDestination), yn(value.AsCopy), value.Code); err != nil {
		return fmt.Errorf("update office: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit office update: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) Deactivate(ctx context.Context, code string) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin office deactivation: %w", err)
	}
	defer transaction.Rollback()
	var existingCode string
	if err := transaction.QueryRowContext(ctx, lockOfficeSQL, code).Scan(&existingCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return offices.ErrNotFound
		}
		return fmt.Errorf("lock office for deactivation: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, deactivateOfficeSQL, code); err != nil {
		return fmt.Errorf("deactivate office: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit office deactivation: %w", err)
	}
	return nil
}

func validateOfficeReferences(ctx context.Context, transaction *sql.Tx, value offices.Office) error {
	var exists int
	if err := transaction.QueryRowContext(ctx, officeTypeExistsSQL, value.Type).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return offices.ErrInvalidType
		}
		return fmt.Errorf("validate office type: %w", err)
	}
	if err := transaction.QueryRowContext(ctx, regionalOfficeExistsSQL, value.RegionalCode).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return offices.ErrInvalidRegional
		}
		return fmt.Errorf("validate regional office: %w", err)
	}
	return nil
}

func scanOffice(row rowScanner) (offices.Office, error) {
	var value offices.Office
	var active, destination, copyOffice string
	if err := row.Scan(&value.Code, &value.Name, &value.Abbreviation, &value.Type, &value.RegionalCode, &active, &destination, &copyOffice); err != nil {
		return offices.Office{}, err
	}
	value.Active = active == "Y"
	value.AsDestination = destination == "Y"
	value.AsCopy = copyOffice == "Y"
	return value, nil
}

type rowScanner interface{ Scan(...any) error }

func yn(value bool) string {
	if value {
		return "Y"
	}
	return "N"
}

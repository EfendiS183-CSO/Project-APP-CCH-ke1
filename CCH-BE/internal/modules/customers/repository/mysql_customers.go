package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"cch.local/backend/internal/modules/customers"
)

const callCenterCustomerSearchQuery = `SELECT ID_Pelanggan, Nama_Lengkap, Email, Nomor_HP, Alamat
FROM cchentricus
WHERE Kantor_Tambah = ?
	AND (Nama_Lengkap LIKE ? ESCAPE '!' OR Nomor_HP LIKE ? ESCAPE '!')
ORDER BY Nama_Lengkap ASC, ID_Pelanggan ASC
LIMIT ?`

const callCenterTicketProfileQuery = `SELECT ID_Pelanggan, Nama_Lengkap, Email, Nomor_HP, Alamat
FROM cchentricus
WHERE ID_Pelanggan = ? AND Kantor_Tambah = ?
LIMIT 1`

var ErrRepositoryNotConfigured = fmt.Errorf("customer repository is not configured")

type MySQLCustomerRepository struct {
	db *sql.DB
}

var _ customers.SearchRepository = (*MySQLCustomerRepository)(nil)

func NewMySQLCustomerRepository(db *sql.DB) *MySQLCustomerRepository {
	return &MySQLCustomerRepository{db: db}
}

func (repository *MySQLCustomerRepository) SearchCallCenterProfiles(ctx context.Context, query customers.SearchQuery) ([]customers.Profile, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	searchPattern := escapeSearchPrefix(query.Term)
	rows, err := repository.db.QueryContext(ctx, callCenterCustomerSearchQuery, query.OfficeCode, searchPattern, searchPattern, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("search Call Center customer profiles: %w", err)
	}
	defer rows.Close()
	profiles := make([]customers.Profile, 0, query.Limit)
	for rows.Next() {
		var profile customers.Profile
		var address sql.NullString
		if err := rows.Scan(&profile.ID, &profile.FullName, &profile.Email, &profile.Phone, &address); err != nil {
			return nil, fmt.Errorf("scan Call Center customer profile: %w", err)
		}
		if address.Valid {
			profile.Address = address.String
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read Call Center customer profiles: %w", err)
	}
	return profiles, nil
}

func (repository *MySQLCustomerRepository) GetCallCenterProfileForOffice(ctx context.Context, profileID int64, officeCode string) (customers.Profile, error) {
	if repository == nil || repository.db == nil {
		return customers.Profile{}, ErrRepositoryNotConfigured
	}
	var profile customers.Profile
	var address sql.NullString
	err := repository.db.QueryRowContext(ctx, callCenterTicketProfileQuery, profileID, officeCode).
		Scan(&profile.ID, &profile.FullName, &profile.Email, &profile.Phone, &address)
	if err != nil {
		return customers.Profile{}, fmt.Errorf("get customer profile for ticket: %w", err)
	}
	if address.Valid {
		profile.Address = address.String
	}
	return profile, nil
}

func escapeSearchPrefix(term string) string {
	var builder strings.Builder
	for _, character := range term {
		if character == '!' || character == '%' || character == '_' {
			builder.WriteRune('!')
		}
		builder.WriteRune(character)
	}
	builder.WriteRune('%')
	return builder.String()
}

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/customers"
)

const countProfilesQuery = `SELECT COUNT(*) FROM cchentricus WHERE Kantor_Tambah = ?`

const listProfilesQuery = `SELECT ID_Pelanggan, Nama_Lengkap, Email, Nomor_HP, Alamat
FROM cchentricus WHERE Kantor_Tambah = ?
ORDER BY Nama_Lengkap ASC, ID_Pelanggan ASC LIMIT ? OFFSET ?`
const getProfileQuery = `SELECT ID_Pelanggan, Nama_Lengkap, Email, Nomor_HP, Alamat
FROM cchentricus WHERE ID_Pelanggan = ? AND Kantor_Tambah = ? LIMIT 1`
const duplicateProfileNameQuery = `SELECT ID_Pelanggan FROM cchentricus WHERE Nama_Lengkap = ? LIMIT 1 FOR UPDATE`
const insertProfileQuery = `INSERT INTO cchentricus
(Nama_Lengkap, Email, Nomor_HP, Alamat, Kantor_Tambah, Tanggal_Tambah, Ditambah_Oleh, Kantor_Ubah, Tanggal_Ubah, Diubah_Oleh)
VALUES (?, ?, ?, ?, ?, NOW(), ?, ?, NOW(), ?)`
const lockProfileQuery = `SELECT ID_Pelanggan FROM cchentricus
WHERE ID_Pelanggan = ? AND Kantor_Tambah = ? FOR UPDATE`
const updateProfileQuery = `UPDATE cchentricus SET Nama_Lengkap = ?, Email = ?, Nomor_HP = ?, Alamat = ?,
Kantor_Ubah = ?, Tanggal_Ubah = NOW(), Diubah_Oleh = ?
WHERE ID_Pelanggan = ? AND Kantor_Tambah = ?`
const ticketProfileReferenceQuery = `SELECT ID_Pengaduan FROM cchentri WHERE ID_Pelanggan = ? LIMIT 1`
const draftProfileReferenceQuery = `SELECT id FROM cch_chatbot_intake_drafts WHERE customer_id = ? LIMIT 1`
const deleteProfileQuery = `DELETE FROM cchentricus WHERE ID_Pelanggan = ? AND Kantor_Tambah = ?`

func (repository *MySQLCustomerRepository) ListProfiles(ctx context.Context, query customers.ProfileListQuery) (customers.ProfilePage, error) {
	if repository == nil || repository.db == nil {
		return customers.ProfilePage{}, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return customers.ProfilePage{}, err
	}
	var total int64
	if err := repository.db.QueryRowContext(ctx, countProfilesQuery, query.OfficeCode).Scan(&total); err != nil {
		return customers.ProfilePage{}, fmt.Errorf("count customer profiles: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, listProfilesQuery, query.OfficeCode, query.Limit, query.Offset)
	if err != nil {
		return customers.ProfilePage{}, fmt.Errorf("list customer profiles: %w", err)
	}
	defer rows.Close()
	profiles := make([]customers.Profile, 0, query.Limit)
	for rows.Next() {
		profile, err := scanProfile(rows)
		if err != nil {
			return customers.ProfilePage{}, fmt.Errorf("scan customer profile: %w", err)
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return customers.ProfilePage{}, fmt.Errorf("read customer profiles: %w", err)
	}
	return customers.ProfilePage{Data: profiles, Total: total, Limit: query.Limit, Offset: query.Offset}, nil
}

func (repository *MySQLCustomerRepository) GetProfile(ctx context.Context, profileID int64, officeCode string) (customers.Profile, error) {
	if repository == nil || repository.db == nil {
		return customers.Profile{}, ErrRepositoryNotConfigured
	}
	profile, err := scanProfile(repository.db.QueryRowContext(ctx, getProfileQuery, profileID, officeCode))
	if err != nil {
		return customers.Profile{}, fmt.Errorf("get customer profile: %w", err)
	}
	return profile, nil
}

func (repository *MySQLCustomerRepository) CreateProfile(ctx context.Context, profile customers.Profile, officeCode string, actor string) (customers.Profile, error) {
	if repository == nil || repository.db == nil {
		return customers.Profile{}, ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return customers.Profile{}, fmt.Errorf("begin customer profile insert: %w", err)
	}
	defer transaction.Rollback()
	var existingID int64
	if err := transaction.QueryRowContext(ctx, duplicateProfileNameQuery, profile.FullName).Scan(&existingID); err == nil {
		return customers.Profile{}, customers.ErrProfileAlreadyExists
	} else if !errors.Is(err, sql.ErrNoRows) {
		return customers.Profile{}, fmt.Errorf("check duplicate customer profile name: %w", err)
	}
	result, err := transaction.ExecContext(ctx, insertProfileQuery,
		profile.FullName, profile.Email, profile.Phone, profile.Address, officeCode, actor, officeCode, actor)
	if err != nil {
		var mysqlError *mysql.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return customers.Profile{}, customers.ErrProfileAlreadyExists
		}
		return customers.Profile{}, fmt.Errorf("insert customer profile: %w", err)
	}
	profile.ID, err = result.LastInsertId()
	if err != nil {
		return customers.Profile{}, fmt.Errorf("read new customer profile ID: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return customers.Profile{}, fmt.Errorf("commit customer profile insert: %w", err)
	}
	return profile, nil
}

func (repository *MySQLCustomerRepository) UpdateProfile(ctx context.Context, profileID int64, officeCode string, profile customers.Profile, actorOffice string, actor string) (customers.Profile, error) {
	if repository == nil || repository.db == nil {
		return customers.Profile{}, ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return customers.Profile{}, fmt.Errorf("begin customer profile update: %w", err)
	}
	defer transaction.Rollback()
	var existingID int64
	if err := transaction.QueryRowContext(ctx, lockProfileQuery, profileID, officeCode).Scan(&existingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return customers.Profile{}, sql.ErrNoRows
		}
		return customers.Profile{}, fmt.Errorf("lock customer profile: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, updateProfileQuery, profile.FullName, profile.Email, profile.Phone, profile.Address, actorOffice, actor, profileID, officeCode); err != nil {
		return customers.Profile{}, fmt.Errorf("update customer profile: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return customers.Profile{}, fmt.Errorf("commit customer profile update: %w", err)
	}
	profile.ID = profileID
	return profile, nil
}

func (repository *MySQLCustomerRepository) DeleteProfile(ctx context.Context, profileID int64, officeCode string) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin customer profile delete: %w", err)
	}
	defer transaction.Rollback()
	var existingID int64
	if err := transaction.QueryRowContext(ctx, lockProfileQuery, profileID, officeCode).Scan(&existingID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return customers.ErrProfileNotFound
		}
		return fmt.Errorf("lock customer profile for delete: %w", err)
	}
	if err := ensureProfileUnreferenced(ctx, transaction, profileID); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, deleteProfileQuery, profileID, officeCode); err != nil {
		return fmt.Errorf("delete customer profile: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit customer profile delete: %w", err)
	}
	return nil
}

func ensureProfileUnreferenced(ctx context.Context, transaction *sql.Tx, profileID int64) error {
	var reference string
	if err := transaction.QueryRowContext(ctx, ticketProfileReferenceQuery, profileID).Scan(&reference); err == nil {
		return customers.ErrProfileInUse
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check ticket profile reference: %w", err)
	}
	var draftID int64
	if err := transaction.QueryRowContext(ctx, draftProfileReferenceQuery, profileID).Scan(&draftID); err == nil {
		return customers.ErrProfileInUse
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check intake draft profile reference: %w", err)
	}
	return nil
}

type profileRowScanner interface{ Scan(...any) error }

func scanProfile(row profileRowScanner) (customers.Profile, error) {
	var profile customers.Profile
	var address sql.NullString
	if err := row.Scan(&profile.ID, &profile.FullName, &profile.Email, &profile.Phone, &address); err != nil {
		return customers.Profile{}, err
	}
	if address.Valid {
		profile.Address = address.String
	}
	return profile, nil
}

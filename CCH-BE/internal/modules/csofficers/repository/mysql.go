package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cch.local/backend/internal/modules/csofficers"
)

const countCSOfficersSQL = `SELECT COUNT(*) FROM cch_cs_officer
WHERE (? = '' OR Regional = ?)
AND (? = '' OR Kantor = ?)
AND (? = '' OR Status_Sekarang = ?)
AND (? = '' OR Username LIKE ? ESCAPE '!' OR Nama_Lengkap LIKE ? ESCAPE '!')`

const listCSOfficersSQL = `SELECT Username, Regional, Kantor, Nama_Lengkap, Status_Sekarang,
Email, Phone, Login_Terakhir, Logout_Terakhir
FROM cch_cs_officer
WHERE (? = '' OR Regional = ?)
AND (? = '' OR Kantor = ?)
AND (? = '' OR Status_Sekarang = ?)
AND (? = '' OR Username LIKE ? ESCAPE '!' OR Nama_Lengkap LIKE ? ESCAPE '!')
ORDER BY Nama_Lengkap ASC, Username ASC LIMIT ? OFFSET ?`

const getCSOfficerSQL = `SELECT Username, Regional, Kantor, Nama_Lengkap, Status_Sekarang,
Email, Phone, Login_Terakhir, Logout_Terakhir
FROM cch_cs_officer WHERE Username = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("CS officer repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ csofficers.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context, query csofficers.ListQuery) (csofficers.Page, error) {
	if repository == nil || repository.db == nil {
		return csofficers.Page{}, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return csofficers.Page{}, err
	}
	filterArgs := queryArgs(query)
	var total int64
	if err := repository.db.QueryRowContext(ctx, countCSOfficersSQL, filterArgs...).Scan(&total); err != nil {
		return csofficers.Page{}, fmt.Errorf("count CS officers: %w", err)
	}
	args := append(filterArgs, query.Limit, query.Offset)
	rows, err := repository.db.QueryContext(ctx, listCSOfficersSQL, args...)
	if err != nil {
		return csofficers.Page{}, fmt.Errorf("list CS officers: %w", err)
	}
	defer rows.Close()
	values := make([]csofficers.Officer, 0, query.Limit)
	for rows.Next() {
		value, err := scanOfficer(rows)
		if err != nil {
			return csofficers.Page{}, fmt.Errorf("scan CS officer: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return csofficers.Page{}, fmt.Errorf("read CS officers: %w", err)
	}
	return csofficers.Page{Data: values, Total: total, Limit: query.Limit, Offset: query.Offset}, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, username string) (csofficers.Officer, error) {
	if repository == nil || repository.db == nil {
		return csofficers.Officer{}, ErrRepositoryNotConfigured
	}
	value, err := scanOfficer(repository.db.QueryRowContext(ctx, getCSOfficerSQL, username))
	if err != nil {
		return csofficers.Officer{}, fmt.Errorf("get CS officer: %w", err)
	}
	return value, nil
}

func queryArgs(query csofficers.ListQuery) []any {
	pattern := ""
	if query.Search != "" {
		pattern = escapeLike(query.Search) + "%"
	}
	return []any{
		query.Regional, query.Regional,
		query.Office, query.Office,
		query.Status, query.Status,
		query.Search, pattern, pattern,
	}
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "!", "!!")
	value = strings.ReplaceAll(value, "%", "!%")
	value = strings.ReplaceAll(value, "_", "!_")
	return value
}

type rowScanner interface{ Scan(...any) error }

func scanOfficer(row rowScanner) (csofficers.Officer, error) {
	var value csofficers.Officer
	var regional, office, fullName, status, email, phone sql.NullString
	var lastLogin, lastLogout sql.NullTime
	if err := row.Scan(&value.Username, &regional, &office, &fullName, &status, &email, &phone, &lastLogin, &lastLogout); err != nil {
		return csofficers.Officer{}, err
	}
	value.Regional = stringPointer(regional)
	value.Office = stringPointer(office)
	value.FullName = stringPointer(fullName)
	value.Status = stringPointer(status)
	value.Email = stringPointer(email)
	value.Phone = stringPointer(phone)
	value.LastLogin = timePointer(lastLogin)
	value.LastLogout = timePointer(lastLogout)
	return value, nil
}

func stringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func timePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	utc := value.Time.UTC()
	return &utc
}

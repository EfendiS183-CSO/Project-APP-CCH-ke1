package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cch.local/backend/internal/modules/followups"
)

const followUpListQuerySQL = `SELECT
	c.ID_Pengaduan,
	f.ID_Pengaduan,
	f.Kantor,
	f.Status,
	f.Tanggal_Rencana,
	f.Catatan_Rencana,
	f.Tanggal_Realisasi,
	f.Keterangan
FROM cchentri AS c
LEFT JOIN (
	SELECT ID_Pengaduan, Kantor, Status, Tanggal_Rencana, Catatan_Rencana, Tanggal_Realisasi, Keterangan
	FROM cchfual
	WHERE ID_Pengaduan = ?
	ORDER BY Tanggal_Rencana DESC, Kantor ASC, Status ASC
	LIMIT ? OFFSET ?
) AS f ON f.ID_Pengaduan = c.ID_Pengaduan
WHERE c.ID_Pengaduan = ?
	AND c.Status_Akhir <> '109'
	AND (c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0)
ORDER BY f.Tanggal_Rencana DESC, f.Kantor ASC, f.Status ASC`

const callCenterOfficeListQuerySQL = `SELECT
	ID_Pengaduan,
	Kantor,
	Status,
	Tanggal_Rencana,
	Catatan_Rencana,
	Tanggal_Realisasi,
	Keterangan
FROM cchfu
WHERE Kantor = ?
ORDER BY Tanggal_Rencana DESC, ID_Pengaduan ASC, Status ASC
LIMIT ? OFFSET ?`

const dueTodayOfficeListQuerySQL = `SELECT
	ID_Pengaduan,
	Kantor,
	Status,
	Tanggal_Rencana,
	Catatan_Rencana,
	Tanggal_Realisasi,
	Keterangan
FROM cchfuhi
WHERE Kantor = ?
ORDER BY Tanggal_Rencana ASC, ID_Pengaduan ASC, Status ASC
LIMIT ? OFFSET ?`

var ErrRepositoryNotConfigured = errors.New("follow-up repository is not configured")

type MySQLRepository struct {
	db *sql.DB
}

var _ followups.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (repository *MySQLRepository) ListForOpenCallCenterTicket(ctx context.Context, query followups.Query) ([]followups.Entry, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	rows, err := repository.db.QueryContext(ctx, followUpListQuerySQL,
		query.ComplaintID,
		query.Limit,
		query.Offset,
		query.ComplaintID,
		query.OfficeCode,
		query.OfficeCode,
	)
	if err != nil {
		return nil, fmt.Errorf("list follow-ups for open Call Center ticket: %w", err)
	}
	defer rows.Close()

	entries := make([]followups.Entry, 0, query.Limit)
	parentFound := false
	for rows.Next() {
		parentFound = true
		entry, hasFollowUp, err := scanFollowUp(rows)
		if err != nil {
			return nil, fmt.Errorf("scan follow-up entry: %w", err)
		}
		if hasFollowUp {
			entries = append(entries, entry)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read follow-up entries: %w", err)
	}
	if !parentFound {
		return nil, sql.ErrNoRows
	}
	return entries, nil
}

func (repository *MySQLRepository) ListForCallCenterOffice(ctx context.Context, query followups.OfficeQuery) ([]followups.Entry, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	rows, err := repository.db.QueryContext(ctx, callCenterOfficeListQuerySQL, query.OfficeCode, query.Limit, query.Offset)
	if err != nil {
		return nil, fmt.Errorf("list follow-ups for Call Center office: %w", err)
	}
	defer rows.Close()

	entries := make([]followups.Entry, 0, query.Limit)
	for rows.Next() {
		entry, err := scanOfficeFollowUp(rows)
		if err != nil {
			return nil, fmt.Errorf("scan Call Center office follow-up: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read Call Center office follow-ups: %w", err)
	}
	return entries, nil
}

func (repository *MySQLRepository) ListDueTodayForCallCenterOffice(ctx context.Context, query followups.OfficeQuery) ([]followups.Entry, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	rows, err := repository.db.QueryContext(ctx, dueTodayOfficeListQuerySQL, query.OfficeCode, query.Limit, query.Offset)
	if err != nil {
		return nil, fmt.Errorf("list due-today follow-ups for Call Center office: %w", err)
	}
	defer rows.Close()

	entries := make([]followups.Entry, 0, query.Limit)
	for rows.Next() {
		entry, err := scanOfficeFollowUp(rows)
		if err != nil {
			return nil, fmt.Errorf("scan due-today Call Center follow-up: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read due-today Call Center follow-ups: %w", err)
	}
	return entries, nil
}

func scanOfficeFollowUp(rows *sql.Rows) (followups.Entry, error) {
	var complaintID string
	var officeCode string
	var statusCode string
	var plannedAt sql.NullTime
	var planNotes sql.NullString
	var realizedAt sql.NullTime
	var outcome sql.NullString
	if err := rows.Scan(&complaintID, &officeCode, &statusCode, &plannedAt, &planNotes, &realizedAt, &outcome); err != nil {
		return followups.Entry{}, err
	}
	entry := followups.Entry{
		ComplaintID: complaintID,
		OfficeCode:  officeCode,
		StatusCode:  statusCode,
		PlanNotes:   nullableString(planNotes),
		Outcome:     nullableString(outcome),
	}
	if plannedAt.Valid {
		date := plannedAt.Time
		entry.PlannedAt = &date
	}
	if realizedAt.Valid {
		date := realizedAt.Time
		entry.RealizedAt = &date
	}
	return entry, nil
}

func scanFollowUp(rows *sql.Rows) (followups.Entry, bool, error) {
	var parentComplaintID string
	var followUpComplaintID sql.NullString
	var officeCode sql.NullString
	var statusCode sql.NullString
	var plannedAt sql.NullTime
	var planNotes sql.NullString
	var realizedAt sql.NullTime
	var outcome sql.NullString
	if err := rows.Scan(
		&parentComplaintID,
		&followUpComplaintID,
		&officeCode,
		&statusCode,
		&plannedAt,
		&planNotes,
		&realizedAt,
		&outcome,
	); err != nil {
		return followups.Entry{}, false, err
	}
	if !followUpComplaintID.Valid {
		return followups.Entry{}, false, nil
	}
	entry := followups.Entry{
		ComplaintID: followUpComplaintID.String,
		OfficeCode:  nullableString(officeCode),
		StatusCode:  nullableString(statusCode),
		PlanNotes:   nullableString(planNotes),
		Outcome:     nullableString(outcome),
	}
	if plannedAt.Valid {
		date := plannedAt.Time
		entry.PlannedAt = &date
	}
	if realizedAt.Valid {
		date := realizedAt.Time
		entry.RealizedAt = &date
	}
	return entry, true, nil
}

func nullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

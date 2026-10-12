package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"cch.local/backend/internal/modules/intake/submission"
)

const (
	activeOfficeLookupQuery = "SELECT 1 FROM cch_kantor WHERE Kode_Kantor = ? AND Apakah_Aktif = 'Y'"

	getDraftQuery = `SELECT conversation_id, status, review_reason, ticket_id, customer_id,
	intake_payload, summary, consent_at, created_at
FROM cch_chatbot_intake_drafts
WHERE conversation_id = ?
LIMIT 1`

	insertDraftQuery = `INSERT IGNORE INTO cch_chatbot_intake_drafts
	(conversation_id, user_id, status, customer_id, intake_payload, summary, consent_at, review_reason)
VALUES (?, 0, ?, ?, ?, ?, NOW(), ?)`

	markDraftCreatedQuery = `UPDATE cch_chatbot_intake_drafts
SET status = 'created', ticket_id = ?, review_reason = NULL
WHERE conversation_id = ? AND status = 'creating'`

	markDraftPendingQuery = `UPDATE cch_chatbot_intake_drafts
SET status = 'pending_cs', review_reason = ?
WHERE conversation_id = ? AND status = 'creating'`

	matchCustomerBaseQuery = `SELECT ID_Pelanggan, Nama_Lengkap, Email, Nomor_HP
FROM cchentricus
WHERE Nama_Lengkap = ? AND Nomor_HP = ?`

	matchTicketQuery = `SELECT ID_Pengaduan
FROM cchentri
WHERE ID_Pelanggan = ?
	AND Nomor_Kiriman = ?
	AND Kantor_Asal = ?
	AND Kantor_Tujuan = ?
	AND Jenis_Pengaduan = ?
	AND Sumber_Pengaduan = ?
	AND Ringkasan_Pengaduan = ?
	AND Uraian_Pengaduan = ?
	AND Tanggal_Tambah >= ?
ORDER BY Tanggal_Tambah ASC, ID_Pengaduan ASC
LIMIT 2`
)

var ErrRepositoryNotConfigured = errors.New("intake submission repository is not configured")

type MySQLRepository struct {
	db *sql.DB
}

var _ submission.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (repository *MySQLRepository) GetDraft(ctx context.Context, conversationID string) (submission.Draft, bool, error) {
	if repository == nil || repository.db == nil {
		return submission.Draft{}, false, ErrRepositoryNotConfigured
	}
	var draft submission.Draft
	var reviewReason sql.NullString
	var ticketID sql.NullString
	var customerID sql.NullInt64
	var payloadJSON string
	var consentAt sql.NullTime
	if err := repository.db.QueryRowContext(ctx, getDraftQuery, conversationID).Scan(
		&draft.ConversationID,
		&draft.Status,
		&reviewReason,
		&ticketID,
		&customerID,
		&payloadJSON,
		&draft.Summary,
		&consentAt,
		&draft.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return submission.Draft{}, false, nil
		}
		return submission.Draft{}, false, fmt.Errorf("load intake draft: %w", err)
	}
	if err := json.Unmarshal([]byte(payloadJSON), &draft.Payload); err != nil {
		return submission.Draft{}, false, fmt.Errorf("decode intake draft payload: %w", err)
	}
	if reviewReason.Valid {
		draft.ReviewReason = reviewReason.String
	}
	if ticketID.Valid {
		draft.TicketID = ticketID.String
	}
	if customerID.Valid {
		draft.CustomerID = customerID.Int64
	}
	if consentAt.Valid {
		draft.ConsentAt = consentAt.Time
	}
	return draft, true, nil
}

func (repository *MySQLRepository) ValidateReferences(ctx context.Context, payload submission.Payload, originOfficeCode string, sourceID int64) (submission.Payload, error) {
	if repository == nil || repository.db == nil {
		return submission.Payload{}, ErrRepositoryNotConfigured
	}
	checks := []struct {
		query string
		args  []any
	}{
		{query: activeOfficeLookupQuery, args: []any{originOfficeCode}},
		{query: "SELECT 1 FROM cch_jenis_pengaduan WHERE Kode_Jenis = ? AND Status_Aktif = 'Y'", args: []any{payload.ComplaintTypeCode}},
		{query: "SELECT 1 FROM cch_jenis_layanan WHERE Kode_Layanan = ?", args: []any{payload.ShippingServiceCode}},
		{query: "SELECT 1 FROM cch_sumber_pengaduan WHERE ID = ? AND Apakah_Aktif = 'Y'", args: []any{sourceID}},
		{query: activeOfficeLookupQuery, args: []any{payload.ShipmentOriginOffice}},
		{query: activeOfficeLookupQuery, args: []any{payload.ShipmentTargetOffice}},
	}
	for _, check := range checks {
		if valid, err := repository.referenceExists(ctx, check.query, check.args...); err != nil {
			return submission.Payload{}, err
		} else if !valid {
			return submission.Payload{}, submission.ErrInvalidSubmission
		}
	}
	destinationOffice := payload.DestinationOfficeCode
	if destinationOffice == "" {
		destinationOffice = payload.ShipmentTargetOffice
	}
	validDestination, err := repository.referenceExists(ctx,
		"SELECT 1 FROM cch_kantor WHERE Kode_Kantor = ? AND Sebagai_Tujuan = 'Y' AND Apakah_Aktif = 'Y' AND Kode_Kantor <> ?",
		destinationOffice,
		originOfficeCode,
	)
	if err != nil {
		return submission.Payload{}, err
	}
	if !validDestination {
		return submission.Payload{}, submission.ErrInvalidSubmission
	}
	payload.DestinationOfficeCode = destinationOffice
	return payload, nil
}

func (repository *MySQLRepository) FindCustomerMatches(ctx context.Context, payload submission.Payload) ([]submission.CustomerProfile, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	query := matchCustomerBaseQuery
	args := []any{payload.CustomerName, payload.CustomerPhone}
	if payload.CustomerEmail != "" {
		query += " AND Email = ?"
		args = append(args, payload.CustomerEmail)
	}
	query += " ORDER BY ID_Pelanggan ASC LIMIT 2"
	rows, err := repository.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("match customer profile for intake: %w", err)
	}
	defer rows.Close()
	profiles := make([]submission.CustomerProfile, 0, 2)
	for rows.Next() {
		var profile submission.CustomerProfile
		var email sql.NullString
		if err := rows.Scan(&profile.ID, &profile.FullName, &email, &profile.Phone); err != nil {
			return nil, fmt.Errorf("scan intake customer match: %w", err)
		}
		if email.Valid {
			profile.Email = email.String
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read intake customer matches: %w", err)
	}
	return profiles, nil
}

func (repository *MySQLRepository) InsertDraftIfAbsent(ctx context.Context, draft submission.Draft) (bool, error) {
	if repository == nil || repository.db == nil {
		return false, ErrRepositoryNotConfigured
	}
	payloadJSON, err := json.Marshal(draft.Payload)
	if err != nil {
		return false, fmt.Errorf("encode intake draft payload: %w", err)
	}
	var customerID any
	if draft.CustomerID > 0 {
		customerID = draft.CustomerID
	}
	result, err := repository.db.ExecContext(ctx, insertDraftQuery,
		draft.ConversationID,
		draft.Status,
		customerID,
		string(payloadJSON),
		draft.Summary,
		nullableText(draft.ReviewReason),
	)
	if err != nil {
		return false, fmt.Errorf("claim intake conversation draft: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read intake draft insert result: %w", err)
	}
	return rowsAffected == 1, nil
}

func (repository *MySQLRepository) MarkDraftCreated(ctx context.Context, conversationID string, ticketID string) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, markDraftCreatedQuery, ticketID, conversationID)
	if err != nil {
		return fmt.Errorf("mark intake draft created: %w", err)
	}
	return requireOneRowAffected("mark intake draft created", result)
}

func (repository *MySQLRepository) MarkDraftPendingCS(ctx context.Context, conversationID string, reason string) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	result, err := repository.db.ExecContext(ctx, markDraftPendingQuery, reason, conversationID)
	if err != nil {
		return fmt.Errorf("mark intake draft for CS review: %w", err)
	}
	return requireOneRowAffected("mark intake draft for CS review", result)
}

func (repository *MySQLRepository) FindMatchingTicketIDs(ctx context.Context, draft submission.Draft, originOfficeCode string, sourceID int64) ([]string, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, matchTicketQuery,
		draft.CustomerID,
		draft.Payload.ShipmentNumber,
		originOfficeCode,
		draft.Payload.DestinationOfficeCode,
		draft.Payload.ComplaintTypeCode,
		sourceID,
		draft.Summary,
		draft.Payload.ComplaintDescription,
		draft.ConsentAt,
	)
	if err != nil {
		return nil, fmt.Errorf("reconcile CCH intake ticket: %w", err)
	}
	defer rows.Close()
	ticketIDs := make([]string, 0, 2)
	for rows.Next() {
		var ticketID string
		if err := rows.Scan(&ticketID); err != nil {
			return nil, fmt.Errorf("scan reconciled CCH ticket ID: %w", err)
		}
		ticketIDs = append(ticketIDs, ticketID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read reconciled CCH tickets: %w", err)
	}
	return ticketIDs, nil
}

func (repository *MySQLRepository) referenceExists(ctx context.Context, query string, args ...any) (bool, error) {
	var exists int
	err := repository.db.QueryRowContext(ctx, query, args...).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("validate intake reference: %w", err)
	}
	return true, nil
}

func requireOneRowAffected(operation string, result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s affected-row check: %w", operation, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%s expected one creating draft, affected %d", operation, rowsAffected)
	}
	return nil
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

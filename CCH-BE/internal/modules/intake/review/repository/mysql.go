package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"cch.local/backend/internal/modules/intake/review"
	"cch.local/backend/internal/modules/intake/submission"
)

const (
	listPendingQuery = `SELECT id, summary, review_reason, created_at
FROM cch_chatbot_intake_drafts
WHERE status = 'pending_cs'
ORDER BY created_at ASC, id ASC
LIMIT ? OFFSET ?`

	getPendingQuery = `SELECT id, conversation_id, status, review_reason, ticket_id, customer_id,
intake_payload, summary, consent_at, created_at
FROM cch_chatbot_intake_drafts
WHERE id = ? AND status = 'pending_cs'
LIMIT 1`

	searchCustomersQuery = `SELECT ID_Pelanggan, Nama_Lengkap, Email, Nomor_HP, Alamat
FROM cchentricus
WHERE CAST(ID_Pelanggan AS CHAR) = ?
	OR Nama_Lengkap LIKE ? ESCAPE '\\'
	OR Nomor_HP LIKE ? ESCAPE '\\'
	OR Email LIKE ? ESCAPE '\\'
ORDER BY Nama_Lengkap ASC, ID_Pelanggan ASC
LIMIT 20`

	getCustomerByIDQuery = `SELECT ID_Pelanggan, Nama_Lengkap, Email, Nomor_HP, Alamat
FROM cchentricus
WHERE ID_Pelanggan = ?
LIMIT 1`

	claimPendingDraftQuery = `UPDATE cch_chatbot_intake_drafts
SET status = 'creating', customer_id = ?, intake_payload = ?, review_reason = NULL
WHERE id = ? AND status = 'pending_cs' AND consent_at IS NOT NULL`
)

var ErrRepositoryNotConfigured = errors.New("intake review repository is not configured")

type MySQLRepository struct {
	db *sql.DB
}

var _ review.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (repository *MySQLRepository) ListPending(ctx context.Context, query review.Query) ([]review.DraftSummary, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listPendingQuery, query.Limit, query.Offset)
	if err != nil {
		return nil, fmt.Errorf("list pending intake drafts: %w", err)
	}
	defer rows.Close()
	drafts := make([]review.DraftSummary, 0, query.Limit)
	for rows.Next() {
		var draft review.DraftSummary
		var summary sql.NullString
		var reason sql.NullString
		if err := rows.Scan(&draft.ID, &summary, &reason, &draft.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending intake draft: %w", err)
		}
		if summary.Valid {
			draft.Summary = summary.String
		}
		if reason.Valid {
			draft.ReviewReason = reason.String
		}
		drafts = append(drafts, draft)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending intake drafts: %w", err)
	}
	return drafts, nil
}

func (repository *MySQLRepository) GetPending(ctx context.Context, draftID int64) (review.PendingDraft, bool, error) {
	if repository == nil || repository.db == nil {
		return review.PendingDraft{}, false, ErrRepositoryNotConfigured
	}
	var result review.PendingDraft
	var reason sql.NullString
	var ticketID sql.NullString
	var customerID sql.NullInt64
	var payloadJSON string
	var summary sql.NullString
	var consentAt sql.NullTime
	err := repository.db.QueryRowContext(ctx, getPendingQuery, draftID).Scan(
		&result.ID,
		&result.Draft.ConversationID,
		&result.Draft.Status,
		&reason,
		&ticketID,
		&customerID,
		&payloadJSON,
		&summary,
		&consentAt,
		&result.Draft.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return review.PendingDraft{}, false, nil
	}
	if err != nil {
		return review.PendingDraft{}, false, fmt.Errorf("load pending intake draft: %w", err)
	}
	if err := json.Unmarshal([]byte(payloadJSON), &result.Draft.Payload); err != nil {
		return review.PendingDraft{}, false, fmt.Errorf("decode pending intake payload: %w", err)
	}
	if reason.Valid {
		result.Draft.ReviewReason = reason.String
	}
	if ticketID.Valid {
		result.Draft.TicketID = ticketID.String
	}
	if customerID.Valid {
		result.Draft.CustomerID = customerID.Int64
	}
	if summary.Valid {
		result.Draft.Summary = summary.String
	}
	if consentAt.Valid {
		result.Draft.ConsentAt = consentAt.Time
	}
	return result, true, nil
}

func (repository *MySQLRepository) SearchCustomers(ctx context.Context, query string) ([]review.CustomerCandidate, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	escapedQuery := escapeLikePattern(query)
	pattern := "%" + escapedQuery + "%"
	rows, err := repository.db.QueryContext(ctx, searchCustomersQuery, query, pattern, pattern, pattern)
	if err != nil {
		return nil, fmt.Errorf("search intake review customer profiles: %w", err)
	}
	defer rows.Close()
	candidates := make([]review.CustomerCandidate, 0, 20)
	for rows.Next() {
		var candidate review.CustomerCandidate
		var email sql.NullString
		var phone sql.NullString
		var address sql.NullString
		if err := rows.Scan(&candidate.ID, &candidate.Name, &email, &phone, &address); err != nil {
			return nil, fmt.Errorf("scan intake review customer profile: %w", err)
		}
		if email.Valid {
			candidate.Email = email.String
		}
		if phone.Valid {
			candidate.Phone = phone.String
		}
		if address.Valid {
			candidate.Address = address.String
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read intake review customer profiles: %w", err)
	}
	return candidates, nil
}

func (repository *MySQLRepository) GetCustomerByID(ctx context.Context, customerID int64) (review.CustomerCandidate, bool, error) {
	if repository == nil || repository.db == nil {
		return review.CustomerCandidate{}, false, ErrRepositoryNotConfigured
	}
	var candidate review.CustomerCandidate
	var email sql.NullString
	var phone sql.NullString
	var address sql.NullString
	err := repository.db.QueryRowContext(ctx, getCustomerByIDQuery, customerID).Scan(
		&candidate.ID,
		&candidate.Name,
		&email,
		&phone,
		&address,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return review.CustomerCandidate{}, false, nil
	}
	if err != nil {
		return review.CustomerCandidate{}, false, fmt.Errorf("load selected intake review customer: %w", err)
	}
	if email.Valid {
		candidate.Email = email.String
	}
	if phone.Valid {
		candidate.Phone = phone.String
	}
	if address.Valid {
		candidate.Address = address.String
	}
	return candidate, true, nil
}

type reviewAttestation struct {
	Reviewer         string    `json:"reviewed_by"`
	IdentityVerified bool      `json:"identity_verified"`
	ReviewedAt       time.Time `json:"reviewed_at"`
}

type claimedDraftPayload struct {
	submission.Payload
	CSReview reviewAttestation `json:"_cs_review"`
}

func (repository *MySQLRepository) ClaimPendingDraft(ctx context.Context, draftID int64, customerID int64, payload submission.Payload, reviewer string, reviewedAt time.Time) (bool, error) {
	if repository == nil || repository.db == nil {
		return false, ErrRepositoryNotConfigured
	}
	reviewer = strings.TrimSpace(reviewer)
	if reviewer == "" || reviewedAt.IsZero() {
		return false, review.ErrInvalidReviewRequest
	}
	payloadJSON, err := json.Marshal(claimedDraftPayload{
		Payload: payload,
		CSReview: reviewAttestation{
			Reviewer:         reviewer,
			IdentityVerified: true,
			ReviewedAt:       reviewedAt.UTC(),
		},
	})
	if err != nil {
		return false, fmt.Errorf("encode CS-reviewed intake payload: %w", err)
	}
	result, err := repository.db.ExecContext(ctx, claimPendingDraftQuery, customerID, string(payloadJSON), draftID)
	if err != nil {
		return false, fmt.Errorf("claim pending intake draft for ticket creation: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read CS intake draft claim result: %w", err)
	}
	return rowsAffected == 1, nil
}

func escapeLikePattern(query string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
}

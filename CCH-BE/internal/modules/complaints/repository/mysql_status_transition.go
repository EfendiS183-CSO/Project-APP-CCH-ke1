package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"cch.local/backend/internal/modules/complaints"
)

const (
	claimStatusTransitionSQL = `INSERT IGNORE INTO cch_be_status_transition_idempotency
	(idempotency_key, request_hash, complaint_id, history_id, status_code, applied_at)
	VALUES (?, ?, ?, '', ?, ?)`
	loadStatusTransitionSQL = `SELECT request_hash, complaint_id, history_id, status_code, applied_at
	FROM cch_be_status_transition_idempotency WHERE idempotency_key = ? FOR UPDATE`
	lockStatusParentSQL = `SELECT Status_Akhir, Kantor_Asal, Kantor_Tujuan, COALESCE(Kantor_Tembusan, '')
	FROM cchentri WHERE ID_Pengaduan = ? FOR UPDATE`
	activeStatusSQL = `SELECT ID_Status FROM cch_jenis_penanganan
	WHERE ID_Status = ? AND Aktif = 'Y' FOR UPDATE`
	latestStatusHistoryIDSQL = `SELECT ID_Histori, Tanggal_Update FROM cchentrimas
	WHERE ID_Pengaduan = ? ORDER BY ID_Histori DESC LIMIT 1`
	finalizeStatusTransitionSQL = `UPDATE cch_be_status_transition_idempotency
	SET history_id = ?, applied_at = ? WHERE idempotency_key = ? AND request_hash = ?`
	statusOfficeRegionsSQL            = `SELECT Kode_Kantor, COALESCE(Regional, '') FROM cch_kantor WHERE Kode_Kantor IN (%s)`
	statusCleanupEmptyDistributionSQL = `DELETE FROM cchentridet WHERE ID_Pengaduan = ?
	AND Kantor_Asal_Update = '' AND Kantor_Tujuan_Update = '' AND Status_Update = ''`
	statusCleanupPlaceholderDistributionSQL = `DELETE FROM cchentridet WHERE ID_Pengaduan = ?
	AND Kantor_Asal_Update = ? AND Kantor_Tujuan_Update = '-'`
	insertStatusHistorySQL = `INSERT INTO cchentrimas
	(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Kantor_Tujuan_Update,
	 Tanggal_Update, Deskripsi_Update, Petugas_Update, Durasi_Antar_Status)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)`
	insertStatusDistributionSQL = `INSERT INTO cchentridet
	(ID_Pengaduan, ID_Histori, Status_Update, Kantor_Asal_Update, Kantor_Tujuan_Update)
	VALUES (?, ?, ?, ?, ?)`
	updateStatusParentSQL = `UPDATE cchentri SET Status_Akhir = ?, Kantor_Status = ?,
	Tanggal_Status = ?, Request_Status = '0' WHERE ID_Pengaduan = ?`
	latestStatusHistorySQL = `SELECT ID_Histori, Tanggal_Update FROM cchentrimas
	WHERE ID_Pengaduan = ? ORDER BY ID_Histori DESC LIMIT 2`
	updateStatusDurationSQL = `UPDATE cchentrimas SET Durasi_Antar_Status = ?, Tanggal_Update = ?
	WHERE ID_Pengaduan = ? AND ID_Histori = ? AND Status_Update = ? AND Kantor_Asal_Update = ?`
)

var (
	ErrStatusTransitionRepositoryNotConfigured = errors.New("status transition repository is not configured")
	ErrStatusTransitionSchemaNotReady          = errors.New("status transition idempotency schema is not ready")
)

type MySQLStatusTransitionRepository struct {
	db  *sql.DB
	now func() time.Time
}

func NewMySQLStatusTransitionRepository(db *sql.DB) *MySQLStatusTransitionRepository {
	return &MySQLStatusTransitionRepository{db: db, now: time.Now}
}

func VerifyStatusTransitionSchema(ctx context.Context, database *sql.DB) error {
	if database == nil {
		return ErrStatusTransitionRepositoryNotConfigured
	}
	rows, err := database.QueryContext(ctx, "SELECT idempotency_key FROM cch_be_status_transition_idempotency LIMIT 0")
	if err != nil {
		return fmt.Errorf("verify status transition migration: %w: %v", ErrStatusTransitionSchemaNotReady, err)
	}
	defer rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("verify status transition migration: %w: %v", ErrStatusTransitionSchemaNotReady, err)
	}
	return nil
}

func (repository *MySQLStatusTransitionRepository) ApplyCallCenterStatusTransition(ctx context.Context, command complaints.StatusTransitionCommand) (complaints.StatusTransitionResult, error) {
	if repository == nil || repository.db == nil {
		return complaints.StatusTransitionResult{}, ErrStatusTransitionRepositoryNotConfigured
	}
	request, err := command.StatusTransitionRequest.Normalize()
	if err != nil || command.OfficeCode == "" || len(command.OfficeCode) > 7 || strings.Contains(command.OfficeCode, ",") ||
		command.Actor == "" || len(command.Actor) > 20 {
		return complaints.StatusTransitionResult{}, complaints.ErrInvalidStatusTransition
	}
	command.StatusTransitionRequest = request

	now := repository.now().UTC()
	requestHash, err := hashStatusTransitionRequest(command)
	if err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("hash status transition request: %w", err)
	}
	transaction, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("begin status transition transaction: %w", err)
	}
	defer transaction.Rollback()

	claim, err := transaction.ExecContext(ctx, claimStatusTransitionSQL,
		request.IdempotencyKey, requestHash, request.ComplaintID, request.StatusCode, now,
	)
	if err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("claim status transition idempotency key: %w", err)
	}
	claimedRows, err := claim.RowsAffected()
	if err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("check status transition idempotency claim: %w", err)
	}
	if claimedRows == 0 {
		result, err := replayStatusTransition(ctx, transaction, request.IdempotencyKey, requestHash)
		if err != nil {
			return complaints.StatusTransitionResult{}, err
		}
		if err := transaction.Commit(); err != nil {
			return complaints.StatusTransitionResult{}, fmt.Errorf("finish status transition replay: %w", err)
		}
		return result, nil
	}

	var currentStatus string
	var originOffice string
	var destinationOffice string
	var copyOffices string
	if err := transaction.QueryRowContext(ctx, lockStatusParentSQL, request.ComplaintID).Scan(
		&currentStatus, &originOffice, &destinationOffice, &copyOffices,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return complaints.StatusTransitionResult{}, complaints.ErrStatusTransitionNotAvailable
		}
		return complaints.StatusTransitionResult{}, fmt.Errorf("lock complaint for status transition: %w", err)
	}
	if originOffice != command.OfficeCode || currentStatus == "109" {
		return complaints.StatusTransitionResult{}, complaints.ErrStatusTransitionNotAvailable
	}
	if currentStatus != request.ExpectedStatusCode {
		return complaints.StatusTransitionResult{}, complaints.ErrStatusTransitionConflict
	}
	var activeStatus string
	if err := transaction.QueryRowContext(ctx, activeStatusSQL, request.StatusCode).Scan(&activeStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return complaints.StatusTransitionResult{}, complaints.ErrStatusTransitionNotAvailable
		}
		return complaints.StatusTransitionResult{}, fmt.Errorf("validate active complaint status: %w", err)
	}
	var latestHistoryID string
	var latestUpdatedAt sql.NullTime
	if err := transaction.QueryRowContext(ctx, latestStatusHistoryIDSQL, request.ComplaintID).Scan(&latestHistoryID, &latestUpdatedAt); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return complaints.StatusTransitionResult{}, fmt.Errorf("load latest complaint history identifier: %w", err)
	}
	transitionTime := repository.now().UTC()
	if latestUpdatedAt.Valid && transitionTime.Truncate(time.Second).Before(latestUpdatedAt.Time.UTC().Truncate(time.Second)) {
		transitionTime = latestUpdatedAt.Time.UTC().Truncate(time.Second).Add(time.Second)
	}
	updatedAt := transitionTime.Truncate(time.Second)
	historyID, err := nextStatusHistoryID(transitionTime, latestHistoryID)
	if err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("generate status history identifier: %w", err)
	}
	finalize, err := transaction.ExecContext(ctx, finalizeStatusTransitionSQL, historyID, updatedAt, request.IdempotencyKey, requestHash)
	if err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("store status transition result: %w", err)
	}
	finalizedRows, err := finalize.RowsAffected()
	if err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("check status transition result: %w", err)
	}
	if finalizedRows != 1 {
		return complaints.StatusTransitionResult{}, errors.New("status transition idempotency claim was not finalized")
	}

	targetOffices, joinedTargetOffices, err := loadStatusDistributionTargets(ctx, transaction, originOffice, destinationOffice, copyOffices, command.OfficeCode)
	if err != nil {
		return complaints.StatusTransitionResult{}, err
	}
	if len(joinedTargetOffices) > 100 {
		return complaints.StatusTransitionResult{}, complaints.ErrInvalidStatusTransition
	}
	if _, err := transaction.ExecContext(ctx, statusCleanupEmptyDistributionSQL, request.ComplaintID); err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("clean empty complaint distributions: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, statusCleanupPlaceholderDistributionSQL, request.ComplaintID, command.OfficeCode); err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("clean placeholder complaint distributions: %w", err)
	}
	for _, targetOffice := range targetOffices {
		if _, err := transaction.ExecContext(ctx, insertStatusDistributionSQL,
			request.ComplaintID, historyID, request.StatusCode, command.OfficeCode, targetOffice,
		); err != nil {
			return complaints.StatusTransitionResult{}, fmt.Errorf("insert complaint status distribution: %w", err)
		}
	}
	if _, err := transaction.ExecContext(ctx, insertStatusHistorySQL,
		request.ComplaintID, historyID, request.StatusCode, command.OfficeCode, joinedTargetOffices,
		updatedAt, request.Description, command.Actor,
	); err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("insert complaint status history: %w", err)
	}
	parentUpdate, err := transaction.ExecContext(ctx, updateStatusParentSQL,
		request.StatusCode, command.OfficeCode, updatedAt, request.ComplaintID,
	)
	if err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("update complaint status snapshot: %w", err)
	}
	updatedRows, err := parentUpdate.RowsAffected()
	if err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("check complaint status snapshot update: %w", err)
	}
	if updatedRows != 1 {
		return complaints.StatusTransitionResult{}, fmt.Errorf("update complaint status snapshot: expected one affected row, got %d", updatedRows)
	}
	if err := updateNewHistoryDuration(ctx, transaction, request.ComplaintID, historyID, request.StatusCode, command.OfficeCode, updatedAt); err != nil {
		return complaints.StatusTransitionResult{}, err
	}
	if err := transaction.Commit(); err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("commit complaint status transition: %w", err)
	}
	return complaints.StatusTransitionResult{
		ComplaintID: request.ComplaintID,
		HistoryID:   historyID,
		StatusCode:  request.StatusCode,
		UpdatedAt:   updatedAt.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func replayStatusTransition(ctx context.Context, transaction *sql.Tx, key string, requestHash string) (complaints.StatusTransitionResult, error) {
	var storedHash string
	var result complaints.StatusTransitionResult
	var appliedAt sql.NullTime
	if err := transaction.QueryRowContext(ctx, loadStatusTransitionSQL, key).Scan(
		&storedHash, &result.ComplaintID, &result.HistoryID, &result.StatusCode, &appliedAt,
	); err != nil {
		return complaints.StatusTransitionResult{}, fmt.Errorf("load status transition idempotency result: %w", err)
	}
	if storedHash != requestHash {
		return complaints.StatusTransitionResult{}, complaints.ErrIdempotencyConflict
	}
	if result.HistoryID == "" || !appliedAt.Valid {
		return complaints.StatusTransitionResult{}, errors.New("status transition idempotency record is incomplete")
	}
	result.UpdatedAt = appliedAt.Time.UTC().Format("2006-01-02T15:04:05Z")
	result.Replayed = true
	return result, nil
}

func loadStatusDistributionTargets(ctx context.Context, transaction *sql.Tx, originOffice string, destinationOffice string, copyOffices string, actingOffice string) ([]string, string, error) {
	offices := uniqueOfficeCodes(originOffice, destinationOffice, copyOffices)
	if len(offices) == 0 {
		return nil, "", nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(offices)), ",")
	query := fmt.Sprintf(statusOfficeRegionsSQL, placeholders)
	arguments := make([]any, len(offices))
	for index, office := range offices {
		arguments[index] = office
	}
	rows, err := transaction.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, "", fmt.Errorf("load regional complaint offices: %w", err)
	}
	defer rows.Close()
	regions := make(map[string]string, len(offices))
	for rows.Next() {
		var office string
		var regional string
		if err := rows.Scan(&office, &regional); err != nil {
			return nil, "", fmt.Errorf("scan regional complaint office: %w", err)
		}
		regions[office] = regional
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("read regional complaint offices: %w", err)
	}
	targets := make([]string, 0, len(offices)*2)
	seen := make(map[string]struct{}, len(offices)*2)
	addTarget := func(office string) {
		office = strings.TrimSpace(office)
		if len(office) < 5 || office == actingOffice {
			return
		}
		if _, exists := seen[office]; exists {
			return
		}
		seen[office] = struct{}{}
		targets = append(targets, office)
	}
	for _, office := range offices {
		if office != actingOffice {
			addTarget(office)
		}
	}
	for _, office := range offices {
		addTarget(regions[office])
	}
	return targets, strings.Join(targets, ","), nil
}

func uniqueOfficeCodes(originOffice string, destinationOffice string, copyOffices string) []string {
	values := []string{originOffice, destinationOffice}
	values = append(values, strings.Split(copyOffices, ",")...)
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func updateNewHistoryDuration(ctx context.Context, transaction *sql.Tx, complaintID string, historyID string, statusCode string, officeCode string, updatedAt time.Time) error {
	rows, err := transaction.QueryContext(ctx, latestStatusHistorySQL, complaintID)
	if err != nil {
		return fmt.Errorf("load complaint status duration timestamps: %w", err)
	}
	defer rows.Close()
	var timestamps []time.Time
	for rows.Next() {
		var storedHistoryID string
		var updatedAt time.Time
		if err := rows.Scan(&storedHistoryID, &updatedAt); err != nil {
			return fmt.Errorf("scan complaint status duration timestamp: %w", err)
		}
		timestamps = append(timestamps, updatedAt)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read complaint status duration timestamps: %w", err)
	}
	duration := int64(0)
	if len(timestamps) > 1 {
		duration = int64(timestamps[0].Sub(timestamps[1]).Seconds())
	}
	update, err := transaction.ExecContext(ctx, updateStatusDurationSQL, duration, updatedAt, complaintID, historyID, statusCode, officeCode)
	if err != nil {
		return fmt.Errorf("update complaint status duration: %w", err)
	}
	updatedRows, err := update.RowsAffected()
	if err != nil {
		return fmt.Errorf("check complaint status duration update: %w", err)
	}
	if updatedRows != 1 {
		return fmt.Errorf("update complaint status duration: expected one affected row, got %d", updatedRows)
	}
	return nil
}

func hashStatusTransitionRequest(command complaints.StatusTransitionCommand) (string, error) {
	canonical := struct {
		ComplaintID        string
		ExpectedStatusCode string
		StatusCode         string
		Description        string
		OfficeCode         string
		Actor              string
	}{
		ComplaintID:        command.ComplaintID,
		ExpectedStatusCode: command.ExpectedStatusCode,
		StatusCode:         command.StatusCode,
		Description:        command.Description,
		OfficeCode:         command.OfficeCode,
		Actor:              command.Actor,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func nextStatusHistoryID(now time.Time, latestHistoryID string) (string, error) {
	prefix := now.Format("20060102150405") + fmt.Sprintf("%02d", now.Nanosecond()/10_000_000)
	if latestHistoryID == "" {
		return prefix + "000000001", nil
	}
	if len(latestHistoryID) < 16 || len(latestHistoryID) > 25 {
		return "", errors.New("latest history identifier has an unsupported format")
	}
	latestPrefix := latestHistoryID[:16]
	if prefix > latestPrefix {
		return prefix + "000000001", nil
	}
	if len(latestHistoryID) == 16 {
		return latestPrefix + "000000001", nil
	}
	suffix := latestHistoryID[16:]
	sequence, err := strconv.ParseUint(suffix, 36, 64)
	if err != nil || sequence >= math.MaxUint64 {
		return "", errors.New("latest history identifier sequence is invalid or exhausted")
	}
	nextSuffix := strings.ToUpper(strconv.FormatUint(sequence+1, 36))
	if len(nextSuffix) > 9 {
		return "", errors.New("history identifier sequence exceeds the legacy column width")
	}
	return latestPrefix + strings.Repeat("0", 9-len(nextSuffix)) + nextSuffix, nil
}

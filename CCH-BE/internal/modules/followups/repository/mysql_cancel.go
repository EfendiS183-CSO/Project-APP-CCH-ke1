package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"cch.local/backend/internal/modules/followups"
)

const (
	lockCancelParentSQL = `SELECT Status_Akhir FROM cchentri
	WHERE ID_Pengaduan = ? AND Status_Akhir <> '109'
	AND (Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan, ''), ' ', '')) > 0)
	FOR UPDATE`
	lockCancelPlanSQL = `SELECT Tanggal_Realisasi FROM cchfu
	WHERE ID_Pengaduan = ? AND Kantor = ? AND Status = ? FOR UPDATE`
	deleteCancelPlanSQL = `DELETE FROM cchfu
	WHERE ID_Pengaduan = ? AND Kantor = ? AND Status = ? AND Tanggal_Realisasi IS NULL`
)

func (repository *MySQLRepository) CancelCallCenterPlan(ctx context.Context, command followups.CancelPlanCommand) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	request, err := command.CancelPlanRequest.Normalize()
	if err != nil || command.OfficeCode == "" || len(command.OfficeCode) > 7 || strings.Contains(command.OfficeCode, ",") {
		return followups.ErrInvalidPlanRequest
	}
	transaction, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin follow-up cancellation transaction: %w", err)
	}
	defer transaction.Rollback()
	var parentStatus string
	if err := transaction.QueryRowContext(ctx, lockCancelParentSQL, request.ComplaintID, command.OfficeCode, command.OfficeCode).Scan(&parentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return followups.ErrPlanCancellationNotFound
		}
		return fmt.Errorf("lock complaint for follow-up cancellation: %w", err)
	}
	var realizedAt sql.NullTime
	if err := transaction.QueryRowContext(ctx, lockCancelPlanSQL, request.ComplaintID, command.OfficeCode, request.StatusCode).Scan(&realizedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return followups.ErrPlanCancellationNotFound
		}
		return fmt.Errorf("lock follow-up for cancellation: %w", err)
	}
	if realizedAt.Valid {
		return followups.ErrPlanCancellationConflict
	}
	result, err := transaction.ExecContext(ctx, deleteCancelPlanSQL, request.ComplaintID, command.OfficeCode, request.StatusCode)
	if err != nil {
		return fmt.Errorf("delete follow-up plan: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check follow-up cancellation: %w", err)
	}
	if rows != 1 {
		return followups.ErrPlanCancellationNotFound
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit follow-up cancellation: %w", err)
	}
	return nil
}

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
	lockEditPlanSQL = `SELECT Tanggal_Realisasi, COALESCE(Catatan_Rencana, '')
	FROM cchfu WHERE ID_Pengaduan = ? AND Kantor = ? AND Status = ? FOR UPDATE`
	updateEditPlanSQL = `UPDATE cchfu SET Tanggal_Rencana = ?, Catatan_Rencana = ?
	WHERE ID_Pengaduan = ? AND Kantor = ? AND Status = ? AND Tanggal_Realisasi IS NULL`
)

func (repository *MySQLRepository) EditCallCenterPlan(ctx context.Context, command followups.EditPlanCommand) (followups.Entry, error) {
	if repository == nil || repository.db == nil {
		return followups.Entry{}, ErrRepositoryNotConfigured
	}
	request, err := command.EditPlanRequest.Normalize()
	if err != nil || command.OfficeCode == "" || len(command.OfficeCode) > 7 || strings.Contains(command.OfficeCode, ",") {
		return followups.Entry{}, followups.ErrInvalidPlanRequest
	}
	transaction, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return followups.Entry{}, fmt.Errorf("begin follow-up edit transaction: %w", err)
	}
	defer transaction.Rollback()
	var realizedAt sql.NullTime
	var previousNotes string
	if err := transaction.QueryRowContext(ctx, lockEditPlanSQL, request.ComplaintID, command.OfficeCode, request.StatusCode).Scan(&realizedAt, &previousNotes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return followups.Entry{}, followups.ErrPlanEditNotFound
		}
		return followups.Entry{}, fmt.Errorf("lock follow-up for edit: %w", err)
	}
	if realizedAt.Valid {
		return followups.Entry{}, followups.ErrPlanEditConflict
	}
	result, err := transaction.ExecContext(ctx, updateEditPlanSQL, request.PlannedAt, request.PlanNotes, request.ComplaintID, command.OfficeCode, request.StatusCode)
	if err != nil {
		return followups.Entry{}, fmt.Errorf("update follow-up plan: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return followups.Entry{}, fmt.Errorf("check follow-up edit: %w", err)
	}
	if rows != 1 {
		return followups.Entry{}, followups.ErrPlanEditNotFound
	}
	if err := transaction.Commit(); err != nil {
		return followups.Entry{}, fmt.Errorf("commit follow-up edit: %w", err)
	}
	return followups.Entry{ComplaintID: request.ComplaintID, OfficeCode: command.OfficeCode, StatusCode: request.StatusCode, PlannedAt: &request.PlannedAt, PlanNotes: request.PlanNotes}, nil
}

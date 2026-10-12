package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cch.local/backend/internal/modules/followups"
)

const (
	lockCallCenterFollowUpSQL = `SELECT Tanggal_Rencana, COALESCE(Catatan_Rencana, ''),
	Tanggal_Realisasi, COALESCE(Petugas_Realisasi, ''), COALESCE(Keterangan, '')
	FROM cchfu WHERE ID_Pengaduan = ? AND Kantor = ? AND Status = ? FOR UPDATE`
	updateCallCenterFollowUpRealizationSQL = `UPDATE cchfu SET Tanggal_Realisasi = ?,
	Petugas_Realisasi = ?, Keterangan = ?
	WHERE ID_Pengaduan = ? AND Kantor = ? AND Status = ? AND Tanggal_Realisasi IS NULL`
)

func (repository *MySQLRepository) RealizeCallCenterPlan(ctx context.Context, command followups.RealizeCommand) (followups.RealizeResult, error) {
	if repository == nil || repository.db == nil {
		return followups.RealizeResult{}, ErrRepositoryNotConfigured
	}
	request, err := command.RealizeRequest.Normalize()
	if err != nil || command.OfficeCode == "" || len(command.OfficeCode) > 7 || strings.Contains(command.OfficeCode, ",") ||
		command.Actor == "" || len(command.Actor) > 20 {
		return followups.RealizeResult{}, followups.ErrInvalidRealization
	}
	command.RealizeRequest = request
	transaction, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return followups.RealizeResult{}, fmt.Errorf("begin follow-up realization transaction: %w", err)
	}
	defer transaction.Rollback()

	var plannedAt sql.NullTime
	var planNotes string
	var realizedAt sql.NullTime
	var realizedBy string
	var existingOutcome string
	if err := transaction.QueryRowContext(ctx, lockCallCenterFollowUpSQL,
		request.ComplaintID, command.OfficeCode, request.StatusCode,
	).Scan(&plannedAt, &planNotes, &realizedAt, &realizedBy, &existingOutcome); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return followups.RealizeResult{}, followups.ErrRealizationNotFound
		}
		return followups.RealizeResult{}, fmt.Errorf("lock follow-up for realization: %w", err)
	}
	entry := followups.Entry{
		ComplaintID: request.ComplaintID,
		OfficeCode:  command.OfficeCode,
		StatusCode:  request.StatusCode,
		PlanNotes:   planNotes,
		Outcome:     request.Outcome,
	}
	if plannedAt.Valid {
		planned := plannedAt.Time
		entry.PlannedAt = &planned
	}
	if realizedAt.Valid {
		realized := realizedAt.Time
		entry.RealizedAt = &realized
		if realizedBy != command.Actor || existingOutcome != request.Outcome {
			return followups.RealizeResult{}, followups.ErrRealizationConflict
		}
		if err := transaction.Commit(); err != nil {
			return followups.RealizeResult{}, fmt.Errorf("finish follow-up realization replay: %w", err)
		}
		return followups.RealizeResult{Entry: entry, Replayed: true}, nil
	}
	realized := time.Now().UTC().Truncate(time.Second)
	update, err := transaction.ExecContext(ctx, updateCallCenterFollowUpRealizationSQL,
		realized, command.Actor, request.Outcome, request.ComplaintID, command.OfficeCode, request.StatusCode,
	)
	if err != nil {
		return followups.RealizeResult{}, fmt.Errorf("update follow-up realization: %w", err)
	}
	updatedRows, err := update.RowsAffected()
	if err != nil {
		return followups.RealizeResult{}, fmt.Errorf("check follow-up realization update: %w", err)
	}
	if updatedRows != 1 {
		return followups.RealizeResult{}, followups.ErrRealizationConflict
	}
	if err := transaction.Commit(); err != nil {
		return followups.RealizeResult{}, fmt.Errorf("commit follow-up realization: %w", err)
	}
	entry.RealizedAt = &realized
	return followups.RealizeResult{Entry: entry}, nil
}

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/followups"
)

const (
	lockFollowUpParentSQL = `SELECT Status_Akhir FROM cchentri
	WHERE ID_Pengaduan = ? AND Status_Akhir <> '109'
	AND (Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(Semua_Tujuan, ''), ' ', '')) > 0)
	FOR UPDATE`
	validateFollowUpStatusSQL = `SELECT Kode_FollowUp FROM cchfust WHERE Kode_FollowUp = ? FOR UPDATE`
	insertFollowUpPlanSQL     = `INSERT INTO cchfu
	(ID_Pengaduan, Kantor, Status, Catatan_Rencana, Tanggal_Rencana, Petugas_Rencana,
	 Keterangan, Tanggal_Realisasi, Petugas_Realisasi)
	VALUES (?, ?, ?, ?, ?, ?, NULL, NULL, NULL)`
)

func (repository *MySQLRepository) CreateCallCenterPlan(ctx context.Context, command followups.CreatePlanCommand) (followups.Entry, error) {
	if repository == nil || repository.db == nil {
		return followups.Entry{}, ErrRepositoryNotConfigured
	}
	request, err := command.CreatePlanRequest.Normalize()
	if err != nil || command.OfficeCode == "" || len(command.OfficeCode) > 7 || strings.Contains(command.OfficeCode, ",") ||
		command.Actor == "" || len(command.Actor) > 20 {
		return followups.Entry{}, followups.ErrInvalidPlanRequest
	}
	command.CreatePlanRequest = request
	transaction, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return followups.Entry{}, fmt.Errorf("begin follow-up plan transaction: %w", err)
	}
	defer transaction.Rollback()

	var parentStatus string
	if err := transaction.QueryRowContext(ctx, lockFollowUpParentSQL, request.ComplaintID, command.OfficeCode, command.OfficeCode).Scan(&parentStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return followups.Entry{}, followups.ErrPlanNotAvailable
		}
		return followups.Entry{}, fmt.Errorf("lock complaint for follow-up plan: %w", err)
	}
	var statusCode string
	if err := transaction.QueryRowContext(ctx, validateFollowUpStatusSQL, request.StatusCode).Scan(&statusCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return followups.Entry{}, followups.ErrInvalidPlanRequest
		}
		return followups.Entry{}, fmt.Errorf("validate follow-up status: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, insertFollowUpPlanSQL,
		request.ComplaintID, command.OfficeCode, request.StatusCode, request.PlanNotes, request.PlannedAt, command.Actor,
	); err != nil {
		var mysqlError *mysql.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return followups.Entry{}, followups.ErrPlanAlreadyExists
		}
		return followups.Entry{}, fmt.Errorf("insert follow-up plan: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return followups.Entry{}, fmt.Errorf("commit follow-up plan: %w", err)
	}
	return followups.Entry{
		ComplaintID: request.ComplaintID,
		OfficeCode:  command.OfficeCode,
		StatusCode:  request.StatusCode,
		PlannedAt:   &request.PlannedAt,
		PlanNotes:   request.PlanNotes,
	}, nil
}

package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/followups"
)

func TestMySQLRepositoryRealizesOfficeFollowUpInTransaction(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := validRealizeCommand()
	plannedAt := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockCallCenterFollowUpSQL)).
		WithArgs(command.ComplaintID, command.OfficeCode, command.StatusCode).
		WillReturnRows(sqlmock.NewRows([]string{"Tanggal_Rencana", "Catatan_Rencana", "Tanggal_Realisasi", "Petugas_Realisasi", "Keterangan"}).
			AddRow(plannedAt, "Call customer", nil, "", ""))
	mock.ExpectExec(regexp.QuoteMeta(updateCallCenterFollowUpRealizationSQL)).
		WithArgs(sqlmock.AnyArg(), command.Actor, command.Outcome, command.ComplaintID, command.OfficeCode, command.StatusCode).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := NewMySQLRepository(database).RealizeCallCenterPlan(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if result.Replayed || result.Entry.ComplaintID != command.ComplaintID || result.Entry.OfficeCode != command.OfficeCode || result.Entry.StatusCode != command.StatusCode || result.Entry.PlanNotes != "Call customer" || result.Entry.Outcome != command.Outcome || result.Entry.RealizedAt == nil || result.Entry.PlannedAt == nil || !result.Entry.PlannedAt.Equal(plannedAt) {
		t.Fatalf("realization result = %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryReplaysSameActorAndOutcomeWithoutUpdating(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := validRealizeCommand()
	realizedAt := time.Date(2026, time.October, 2, 11, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockCallCenterFollowUpSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"Tanggal_Rencana", "Catatan_Rencana", "Tanggal_Realisasi", "Petugas_Realisasi", "Keterangan"}).
			AddRow(nil, "Call customer", realizedAt, command.Actor, command.Outcome))
	mock.ExpectCommit()

	result, err := NewMySQLRepository(database).RealizeCallCenterPlan(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.Entry.RealizedAt == nil || !result.Entry.RealizedAt.Equal(realizedAt) {
		t.Fatalf("realization replay = %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsDifferentReplayAndForeignOffice(t *testing.T) {
	for _, test := range []struct {
		name    string
		rows    *sqlmock.Rows
		wantErr error
	}{
		{
			name: "different realization",
			rows: sqlmock.NewRows([]string{"Tanggal_Rencana", "Catatan_Rencana", "Tanggal_Realisasi", "Petugas_Realisasi", "Keterangan"}).
				AddRow(nil, "Call customer", time.Now(), "other.agent", "Different outcome"),
			wantErr: followups.ErrRealizationConflict,
		},
		{
			name:    "row absent from current office",
			rows:    sqlmock.NewRows([]string{"Tanggal_Rencana", "Catatan_Rencana", "Tanggal_Realisasi", "Petugas_Realisasi", "Keterangan"}),
			wantErr: followups.ErrRealizationNotFound,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(lockCallCenterFollowUpSQL)).WillReturnRows(test.rows)
			mock.ExpectRollback()
			_, err = NewMySQLRepository(database).RealizeCallCenterPlan(context.Background(), validRealizeCommand())
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("RealizeCallCenterPlan() error = %v, want %v", err, test.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func validRealizeCommand() followups.RealizeCommand {
	return followups.RealizeCommand{
		RealizeRequest: followups.RealizeRequest{ComplaintID: "TKT-001", StatusCode: "FU1", Outcome: "Pelanggan telah dihubungi"},
		OfficeCode:     "40005CC",
		Actor:          "agent.test",
	}
}

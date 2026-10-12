package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/followups"
)

func TestMySQLRepositoryCreatesFollowUpPlanInTransaction(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := validCreatePlanCommand()
	plannedAt := command.PlannedAt

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockFollowUpParentSQL)).
		WithArgs(command.ComplaintID, command.OfficeCode, command.OfficeCode).
		WillReturnRows(sqlmock.NewRows([]string{"Status_Akhir"}).AddRow("102"))
	mock.ExpectQuery(regexp.QuoteMeta(validateFollowUpStatusSQL)).WithArgs(command.StatusCode).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_FollowUp"}).AddRow(command.StatusCode))
	mock.ExpectExec(regexp.QuoteMeta(insertFollowUpPlanSQL)).
		WithArgs(command.ComplaintID, command.OfficeCode, command.StatusCode, command.PlanNotes, plannedAt, command.Actor).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	entry, err := NewMySQLRepository(database).CreateCallCenterPlan(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if entry.ComplaintID != command.ComplaintID || entry.OfficeCode != command.OfficeCode || entry.StatusCode != command.StatusCode || entry.PlanNotes != command.PlanNotes || entry.PlannedAt == nil || !entry.PlannedAt.Equal(plannedAt) {
		t.Fatalf("created entry = %#v", entry)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsUnavailableParentAndUnknownPlanStatus(t *testing.T) {
	for _, test := range []struct {
		name       string
		parentRows *sqlmock.Rows
		statusRows *sqlmock.Rows
		wantErr    error
	}{
		{
			name:       "parent not visible or closed",
			parentRows: sqlmock.NewRows([]string{"Status_Akhir"}),
			wantErr:    followups.ErrPlanNotAvailable,
		},
		{
			name:       "unknown follow-up status",
			parentRows: sqlmock.NewRows([]string{"Status_Akhir"}).AddRow("102"),
			statusRows: sqlmock.NewRows([]string{"Kode_FollowUp"}),
			wantErr:    followups.ErrInvalidPlanRequest,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			command := validCreatePlanCommand()
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(lockFollowUpParentSQL)).
				WillReturnRows(test.parentRows)
			if test.statusRows != nil {
				mock.ExpectQuery(regexp.QuoteMeta(validateFollowUpStatusSQL)).WillReturnRows(test.statusRows)
			}
			mock.ExpectRollback()

			_, err = NewMySQLRepository(database).CreateCallCenterPlan(context.Background(), command)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("CreateCallCenterPlan() error = %v, want %v", err, test.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMySQLRepositoryRejectsDuplicateFollowUpPlan(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := validCreatePlanCommand()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockFollowUpParentSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"Status_Akhir"}).AddRow("102"))
	mock.ExpectQuery(regexp.QuoteMeta(validateFollowUpStatusSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_FollowUp"}).AddRow(command.StatusCode))
	mock.ExpectExec(regexp.QuoteMeta(insertFollowUpPlanSQL)).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate key"})
	mock.ExpectRollback()

	_, err = NewMySQLRepository(database).CreateCallCenterPlan(context.Background(), command)
	if !errors.Is(err, followups.ErrPlanAlreadyExists) {
		t.Fatalf("CreateCallCenterPlan() error = %v, want duplicate plan", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsInvalidPlanBeforeStartingTransaction(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := validCreatePlanCommand()
	command.OfficeCode = ""
	if _, err := NewMySQLRepository(database).CreateCallCenterPlan(context.Background(), command); !errors.Is(err, followups.ErrInvalidPlanRequest) {
		t.Fatalf("CreateCallCenterPlan() error = %v, want invalid plan", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryCancelsUnrealizedPlanTransactionally(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := followups.CancelPlanCommand{CancelPlanRequest: followups.CancelPlanRequest{ComplaintID: "TKT-001", StatusCode: "FU1"}, OfficeCode: "40005CC"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockCancelParentSQL)).WithArgs(command.ComplaintID, command.OfficeCode, command.OfficeCode).
		WillReturnRows(sqlmock.NewRows([]string{"Status_Akhir"}).AddRow("102"))
	mock.ExpectQuery(regexp.QuoteMeta(lockCancelPlanSQL)).WithArgs(command.ComplaintID, command.OfficeCode, command.StatusCode).
		WillReturnRows(sqlmock.NewRows([]string{"Tanggal_Realisasi"}).AddRow(nil))
	mock.ExpectExec(regexp.QuoteMeta(deleteCancelPlanSQL)).WithArgs(command.ComplaintID, command.OfficeCode, command.StatusCode).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := NewMySQLRepository(database).CancelCallCenterPlan(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRefusesRealizedPlanCancellation(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := followups.CancelPlanCommand{CancelPlanRequest: followups.CancelPlanRequest{ComplaintID: "TKT-001", StatusCode: "FU1"}, OfficeCode: "40005CC"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockCancelParentSQL)).WillReturnRows(sqlmock.NewRows([]string{"Status_Akhir"}).AddRow("102"))
	mock.ExpectQuery(regexp.QuoteMeta(lockCancelPlanSQL)).WillReturnRows(sqlmock.NewRows([]string{"Tanggal_Realisasi"}).AddRow(time.Date(2026, time.October, 2, 11, 0, 0, 0, time.UTC)))
	mock.ExpectRollback()
	if err := NewMySQLRepository(database).CancelCallCenterPlan(context.Background(), command); !errors.Is(err, followups.ErrPlanCancellationConflict) {
		t.Fatalf("CancelCallCenterPlan() error = %v, want realized conflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryEditsUnrealizedPlanTransactionally(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	plannedAt := time.Date(2026, time.October, 3, 12, 0, 0, 0, time.UTC)
	command := followups.EditPlanCommand{EditPlanRequest: followups.EditPlanRequest{ComplaintID: "TKT-001", StatusCode: "FU1", PlannedAt: plannedAt, PlanNotes: "Updated"}, OfficeCode: "40005CC"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockEditPlanSQL)).WithArgs(command.ComplaintID, command.OfficeCode, command.StatusCode).
		WillReturnRows(sqlmock.NewRows([]string{"Tanggal_Realisasi", "Catatan_Rencana"}).AddRow(nil, "Original"))
	mock.ExpectExec(regexp.QuoteMeta(updateEditPlanSQL)).WithArgs(plannedAt, command.PlanNotes, command.ComplaintID, command.OfficeCode, command.StatusCode).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	entry, err := NewMySQLRepository(database).EditCallCenterPlan(context.Background(), command)
	if err != nil || entry.PlanNotes != "Updated" || entry.PlannedAt == nil || !entry.PlannedAt.Equal(plannedAt) {
		t.Fatalf("entry/error = %#v/%v", entry, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRefusesRealizedPlanEdit(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := followups.EditPlanCommand{EditPlanRequest: followups.EditPlanRequest{ComplaintID: "TKT-001", StatusCode: "FU1", PlannedAt: time.Now().UTC(), PlanNotes: "Updated"}, OfficeCode: "40005CC"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockEditPlanSQL)).WillReturnRows(sqlmock.NewRows([]string{"Tanggal_Realisasi", "Catatan_Rencana"}).AddRow(time.Now(), "Original"))
	mock.ExpectRollback()
	if _, err := NewMySQLRepository(database).EditCallCenterPlan(context.Background(), command); !errors.Is(err, followups.ErrPlanEditConflict) {
		t.Fatalf("EditCallCenterPlan() error = %v, want realized conflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func validCreatePlanCommand() followups.CreatePlanCommand {
	return followups.CreatePlanCommand{
		CreatePlanRequest: followups.CreatePlanRequest{
			ComplaintID: "TKT-001",
			StatusCode:  "FU1",
			PlannedAt:   time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC),
			PlanNotes:   "Hubungi pelanggan",
		},
		OfficeCode: "40005CC",
		Actor:      "agent.test",
	}
}

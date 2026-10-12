package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/complaints"
)

func TestMySQLStatusTransitionCommitsHistoryDistributionSnapshotAndDurationTogether(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Date(2026, time.October, 1, 10, 20, 30, 450_000_000, time.UTC)
	updatedAt := now.Truncate(time.Second)
	command := validStatusTransitionCommand()
	repository := NewMySQLStatusTransitionRepository(database)
	repository.now = func() time.Time { return now }

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(claimStatusTransitionSQL)).
		WithArgs(command.IdempotencyKey, sqlmock.AnyArg(), command.ComplaintID, command.StatusCode, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(lockStatusParentSQL)).WithArgs(command.ComplaintID).
		WillReturnRows(sqlmock.NewRows([]string{"Status_Akhir", "Kantor_Asal", "Kantor_Tujuan", "Kantor_Tembusan"}).
			AddRow("102", "40005CC", "40006CC", "40007CC"))
	mock.ExpectQuery(regexp.QuoteMeta(activeStatusSQL)).WithArgs("103").
		WillReturnRows(sqlmock.NewRows([]string{"ID_Status"}).AddRow("103"))
	mock.ExpectQuery(regexp.QuoteMeta(latestStatusHistoryIDSQL)).WithArgs(command.ComplaintID).
		WillReturnRows(sqlmock.NewRows([]string{"ID_Histori", "Tanggal_Update"}).AddRow("2026100110195900", updatedAt.Add(-time.Second)))
	mock.ExpectExec(regexp.QuoteMeta(finalizeStatusTransitionSQL)).
		WithArgs("2026100110203045000000001", updatedAt, command.IdempotencyKey, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	regionalQuery := strings.Replace(statusOfficeRegionsSQL, "%s", "?,?,?", 1)
	mock.ExpectQuery(regexp.QuoteMeta(regionalQuery)).WithArgs("40005CC", "40006CC", "40007CC").
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor", "Regional"}).
			AddRow("40005CC", "40005").AddRow("40006CC", "40005").AddRow("40007CC", "40005"))
	mock.ExpectExec(regexp.QuoteMeta(statusCleanupEmptyDistributionSQL)).WithArgs(command.ComplaintID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(statusCleanupPlaceholderDistributionSQL)).WithArgs(command.ComplaintID, command.OfficeCode).
		WillReturnResult(sqlmock.NewResult(0, 0))
	for _, targetOffice := range []string{"40006CC", "40007CC", "40005"} {
		mock.ExpectExec(regexp.QuoteMeta(insertStatusDistributionSQL)).
			WithArgs(command.ComplaintID, sqlmock.AnyArg(), command.StatusCode, command.OfficeCode, targetOffice).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec(regexp.QuoteMeta(insertStatusHistorySQL)).
		WithArgs(command.ComplaintID, sqlmock.AnyArg(), command.StatusCode, command.OfficeCode,
			"40006CC,40007CC,40005", updatedAt, command.Description, command.Actor).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(updateStatusParentSQL)).
		WithArgs(command.StatusCode, command.OfficeCode, updatedAt, command.ComplaintID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	previousAt := updatedAt.Add(-time.Hour)
	mock.ExpectQuery(regexp.QuoteMeta(latestStatusHistorySQL)).WithArgs(command.ComplaintID).
		WillReturnRows(sqlmock.NewRows([]string{"ID_Histori", "Tanggal_Update"}).
			AddRow("new-history", updatedAt).AddRow("old-history", previousAt))
	mock.ExpectExec(regexp.QuoteMeta(updateStatusDurationSQL)).
		WithArgs(int64(3600), updatedAt, command.ComplaintID, sqlmock.AnyArg(), command.StatusCode, command.OfficeCode).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repository.ApplyCallCenterStatusTransition(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if result.ComplaintID != command.ComplaintID || result.StatusCode != command.StatusCode || result.Replayed {
		t.Fatalf("unexpected transition result: %#v", result)
	}
	if result.HistoryID != "2026100110203045000000001" {
		t.Fatalf("history ID = %q, want monotonic legacy-sortable identifier", result.HistoryID)
	}
	if result.UpdatedAt != "2026-10-01T10:20:30Z" {
		t.Fatalf("UpdatedAt = %q", result.UpdatedAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLStatusTransitionReplaysCommittedIdempotencyKey(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	command := validStatusTransitionCommand()
	requestHash, err := hashStatusTransitionRequest(command)
	if err != nil {
		t.Fatal(err)
	}
	appliedAt := time.Date(2026, time.October, 1, 10, 20, 30, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(claimStatusTransitionSQL)).
		WithArgs(command.IdempotencyKey, sqlmock.AnyArg(), command.ComplaintID, command.StatusCode, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(loadStatusTransitionSQL)).WithArgs(command.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{"request_hash", "complaint_id", "history_id", "status_code", "applied_at"}).
			AddRow(requestHash, command.ComplaintID, "2026100110203045ABCD2345", command.StatusCode, appliedAt))
	mock.ExpectCommit()

	result, err := NewMySQLStatusTransitionRepository(database).ApplyCallCenterStatusTransition(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Replayed || result.HistoryID != "2026100110203045ABCD2345" || result.UpdatedAt != "2026-10-01T10:20:30Z" {
		t.Fatalf("unexpected idempotent replay: %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLStatusTransitionRollsBackWhenSnapshotUpdateFails(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Date(2026, time.October, 1, 10, 20, 30, 450_000_000, time.UTC)
	updatedAt := now.Truncate(time.Second)
	command := validStatusTransitionCommand()
	repository := NewMySQLStatusTransitionRepository(database)
	repository.now = func() time.Time { return now }

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(claimStatusTransitionSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(lockStatusParentSQL)).WillReturnRows(sqlmock.NewRows([]string{"Status_Akhir", "Kantor_Asal", "Kantor_Tujuan", "Kantor_Tembusan"}).AddRow("102", "40005CC", "40006CC", ""))
	mock.ExpectQuery(regexp.QuoteMeta(activeStatusSQL)).WillReturnRows(sqlmock.NewRows([]string{"ID_Status"}).AddRow("103"))
	mock.ExpectQuery(regexp.QuoteMeta(latestStatusHistoryIDSQL)).WithArgs(command.ComplaintID).
		WillReturnRows(sqlmock.NewRows([]string{"ID_Histori", "Tanggal_Update"}).AddRow("2026100110195900", updatedAt.Add(-time.Second)))
	mock.ExpectExec(regexp.QuoteMeta(finalizeStatusTransitionSQL)).
		WithArgs("2026100110203045000000001", updatedAt, command.IdempotencyKey, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(strings.Replace(statusOfficeRegionsSQL, "%s", "?,?", 1))).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor", "Regional"}).AddRow("40005CC", "40005").AddRow("40006CC", "40005"))
	mock.ExpectExec(regexp.QuoteMeta(statusCleanupEmptyDistributionSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(statusCleanupPlaceholderDistributionSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(insertStatusDistributionSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(insertStatusDistributionSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(insertStatusHistorySQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(updateStatusParentSQL)).
		WillReturnError(errors.New("injected snapshot failure"))
	mock.ExpectRollback()

	_, err = repository.ApplyCallCenterStatusTransition(context.Background(), command)
	if err == nil || !strings.Contains(err.Error(), "update complaint status snapshot") {
		t.Fatalf("error = %v, want snapshot update failure", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLStatusTransitionRollsBackAtEveryWriteStage(t *testing.T) {
	for _, failureStage := range []string{"distribution", "history", "snapshot", "duration read", "duration update"} {
		t.Run(failureStage, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()

			now := time.Date(2026, time.October, 1, 10, 20, 30, 450_000_000, time.UTC)
			updatedAt := now.Truncate(time.Second)
			command := validStatusTransitionCommand()
			repository := NewMySQLStatusTransitionRepository(database)
			repository.now = func() time.Time { return now }
			expectStatusTransitionPreamble(mock, command)

			if failureStage == "distribution" {
				mock.ExpectExec(regexp.QuoteMeta(insertStatusDistributionSQL)).WillReturnError(errors.New("injected distribution failure"))
			} else {
				expectStatusDistributionRows(mock, command)
				if failureStage == "history" {
					mock.ExpectExec(regexp.QuoteMeta(insertStatusHistorySQL)).WillReturnError(errors.New("injected history failure"))
				} else {
					expectStatusHistoryRow(mock, command, updatedAt)
					if failureStage == "snapshot" {
						mock.ExpectExec(regexp.QuoteMeta(updateStatusParentSQL)).WillReturnError(errors.New("injected snapshot failure"))
					} else {
						mock.ExpectExec(regexp.QuoteMeta(updateStatusParentSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
						if failureStage == "duration read" {
							mock.ExpectQuery(regexp.QuoteMeta(latestStatusHistorySQL)).WillReturnError(errors.New("injected duration read failure"))
						} else {
							mock.ExpectQuery(regexp.QuoteMeta(latestStatusHistorySQL)).
								WillReturnRows(sqlmock.NewRows([]string{"ID_Histori", "Tanggal_Update"}).
									AddRow("new-history", updatedAt).AddRow("old-history", updatedAt.Add(-time.Minute)))
							mock.ExpectExec(regexp.QuoteMeta(updateStatusDurationSQL)).
								WillReturnError(errors.New("injected duration update failure"))
						}
					}
				}
			}
			mock.ExpectRollback()

			if _, err := repository.ApplyCallCenterStatusTransition(context.Background(), command); err == nil {
				t.Fatal("injected failure unexpectedly committed the transition")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func expectStatusTransitionPreamble(mock sqlmock.Sqlmock, command complaints.StatusTransitionCommand) {
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(claimStatusTransitionSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(lockStatusParentSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"Status_Akhir", "Kantor_Asal", "Kantor_Tujuan", "Kantor_Tembusan"}).
			AddRow("102", "40005CC", "40006CC", ""))
	mock.ExpectQuery(regexp.QuoteMeta(activeStatusSQL)).WillReturnRows(sqlmock.NewRows([]string{"ID_Status"}).AddRow("103"))
	updatedAt := time.Date(2026, time.October, 1, 10, 20, 30, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(latestStatusHistoryIDSQL)).WithArgs(command.ComplaintID).
		WillReturnRows(sqlmock.NewRows([]string{"ID_Histori", "Tanggal_Update"}).AddRow("2026100110195900", updatedAt.Add(-time.Second)))
	mock.ExpectExec(regexp.QuoteMeta(finalizeStatusTransitionSQL)).
		WithArgs("2026100110203045000000001", updatedAt, command.IdempotencyKey, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(strings.Replace(statusOfficeRegionsSQL, "%s", "?,?", 1))).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor", "Regional"}).AddRow("40005CC", "40005").AddRow("40006CC", "40005"))
	mock.ExpectExec(regexp.QuoteMeta(statusCleanupEmptyDistributionSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(statusCleanupPlaceholderDistributionSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
}

func TestNextStatusHistoryIDIsMonotonicWithinAndAcrossClockTicks(t *testing.T) {
	now := time.Date(2026, time.October, 1, 10, 20, 30, 450_000_000, time.UTC)
	for _, test := range []struct {
		name     string
		latest   string
		want     string
		wantFail bool
	}{
		{name: "first generated ID", latest: "2026100110195900", want: "2026100110203045000000001"},
		{name: "same centisecond", latest: "2026100110203045000000001", want: "2026100110203045000000002"},
		{name: "clock moved backwards", latest: "2026100110210045000000001", want: "2026100110210045000000002"},
		{name: "legacy ID without suffix", latest: "2026100110203045", want: "2026100110203045000000001"},
		{name: "invalid legacy ID", latest: "bad", wantFail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := nextStatusHistoryID(now, test.latest)
			if test.wantFail {
				if err == nil {
					t.Fatalf("nextStatusHistoryID() = %q, want format error", got)
				}
				return
			}
			if err != nil || got != test.want || got <= test.latest {
				t.Fatalf("nextStatusHistoryID() = %q, %v, want %q lexically after %q", got, err, test.want, test.latest)
			}
		})
	}
}

func expectStatusDistributionRows(mock sqlmock.Sqlmock, command complaints.StatusTransitionCommand) {
	for _, targetOffice := range []string{"40006CC", "40005"} {
		mock.ExpectExec(regexp.QuoteMeta(insertStatusDistributionSQL)).
			WithArgs(command.ComplaintID, sqlmock.AnyArg(), command.StatusCode, command.OfficeCode, targetOffice).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
}

func expectStatusHistoryRow(mock sqlmock.Sqlmock, command complaints.StatusTransitionCommand, updatedAt time.Time) {
	mock.ExpectExec(regexp.QuoteMeta(insertStatusHistorySQL)).
		WithArgs(command.ComplaintID, sqlmock.AnyArg(), command.StatusCode, command.OfficeCode,
			"40006CC,40005", updatedAt, command.Description, command.Actor).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestMySQLStatusTransitionRejectsStaleAndClosedParent(t *testing.T) {
	for _, test := range []struct {
		name         string
		status       string
		originOffice string
		wantErr      error
	}{
		{name: "stale status", status: "101", originOffice: "40005CC", wantErr: complaints.ErrStatusTransitionConflict},
		{name: "closed ticket", status: "109", originOffice: "40005CC", wantErr: complaints.ErrStatusTransitionNotAvailable},
		{name: "other origin office", status: "102", originOffice: "40006CC", wantErr: complaints.ErrStatusTransitionNotAvailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(claimStatusTransitionSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(regexp.QuoteMeta(lockStatusParentSQL)).
				WillReturnRows(sqlmock.NewRows([]string{"Status_Akhir", "Kantor_Asal", "Kantor_Tujuan", "Kantor_Tembusan"}).AddRow(test.status, test.originOffice, "40006CC", ""))
			mock.ExpectRollback()

			_, err = NewMySQLStatusTransitionRepository(database).ApplyCallCenterStatusTransition(context.Background(), validStatusTransitionCommand())
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ApplyCallCenterStatusTransition() error = %v, want %v", err, test.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMySQLStatusTransitionRejectsReusedIdempotencyKeyForDifferentPayload(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	command := validStatusTransitionCommand()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(claimStatusTransitionSQL)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(loadStatusTransitionSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"request_hash", "complaint_id", "history_id", "status_code", "applied_at"}).
			AddRow(strings.Repeat("a", 64), command.ComplaintID, "HIST-001", command.StatusCode, time.Now()))
	mock.ExpectRollback()

	_, err = NewMySQLStatusTransitionRepository(database).ApplyCallCenterStatusTransition(context.Background(), command)
	if !errors.Is(err, complaints.ErrIdempotencyConflict) {
		t.Fatalf("ApplyCallCenterStatusTransition() error = %v, want idempotency conflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func validStatusTransitionCommand() complaints.StatusTransitionCommand {
	return complaints.StatusTransitionCommand{
		StatusTransitionRequest: complaints.StatusTransitionRequest{
			ComplaintID:        "TKT-001",
			ExpectedStatusCode: "102",
			StatusCode:         "103",
			Description:        "Sedang dikonfirmasi",
			IdempotencyKey:     "request-key-0001",
		},
		OfficeCode: "40005CC",
		Actor:      "agent.test",
	}
}

func TestVerifyStatusTransitionSchemaRequiresInstalledMigration(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT idempotency_key FROM cch_be_status_transition_idempotency LIMIT 0")).
		WillReturnRows(sqlmock.NewRows([]string{"idempotency_key"}))
	if err := VerifyStatusTransitionSchema(context.Background(), database); err != nil {
		t.Fatalf("VerifyStatusTransitionSchema() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyStatusTransitionSchemaFailsClosedWhenMigrationIsMissing(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT idempotency_key FROM cch_be_status_transition_idempotency LIMIT 0")).
		WillReturnError(errors.New("table missing"))
	if err := VerifyStatusTransitionSchema(context.Background(), database); !errors.Is(err, ErrStatusTransitionSchemaNotReady) {
		t.Fatalf("VerifyStatusTransitionSchema() error = %v, want schema-not-ready", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

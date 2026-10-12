package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/timezones"
)

func TestMySQLRepositoryReadsWritesAndDeletesTimezones(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	columns := []string{"ID", "Timezone", "Default"}
	mock.ExpectQuery(regexp.QuoteMeta(listTimezonesSQL)).WillReturnRows(sqlmock.NewRows(columns).AddRow(1, "Asia/Jakarta", "Y").AddRow(2, "Asia/Makassar", "N"))
	mock.ExpectQuery(regexp.QuoteMeta(getTimezoneSQL)).WithArgs(1).WillReturnRows(sqlmock.NewRows(columns).AddRow(1, "Asia/Jakarta", "Y"))
	mock.ExpectQuery(regexp.QuoteMeta(defaultTimezoneExistsSQL)).WithArgs(int64(0)).WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	mock.ExpectExec(regexp.QuoteMeta(insertTimezoneSQL)).WithArgs("Asia/Jayapura", "N").WillReturnResult(sqlmock.NewResult(3, 1))
	// Update ID=8, no default elsewhere
	mock.ExpectQuery(regexp.QuoteMeta(defaultTimezoneExistsSQL)).WithArgs(int64(8)).WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	mock.ExpectExec(regexp.QuoteMeta(updateTimezoneSQL)).WithArgs("Asia/Singapore", "Y", int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	// Delete ID=9 (not default) directly
	mock.ExpectQuery(regexp.QuoteMeta(getDefaultTimezoneExistsSQL)).WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	mock.ExpectExec(regexp.QuoteMeta(deleteTimezoneSQL)).WithArgs(int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))

	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil || len(values) != 2 || !values[0].IsDefault || values[1].IsDefault {
		t.Fatalf("List() = %#v, error = %v", values, err)
	}
	value, err := repository.Get(context.Background(), 1)
	if err != nil || value.Timezone != "Asia/Jakarta" || !value.IsDefault {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	if err := repository.Insert(context.Background(), timezones.Timezone{Timezone: "Asia/Jayapura", IsDefault: false}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Update(context.Background(), timezones.Timezone{ID: 8, Timezone: "Asia/Singapore", IsDefault: true}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), 9); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsSecondDefaultAndDuplicateName(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(defaultTimezoneExistsSQL)).WithArgs(int64(0)).WillReturnRows(sqlmock.NewRows([]string{"ID"}).AddRow(5))
	repository := NewMySQLRepository(database)
	if err := repository.Insert(context.Background(), timezones.Timezone{Timezone: "Asia/Jakarta", IsDefault: true}); err != timezones.ErrOnlyOneDefault {
		t.Fatalf("second default insert error = %v, want only one default", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(defaultTimezoneExistsSQL)).WithArgs(int64(0)).WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	mock.ExpectExec(regexp.QuoteMeta(insertTimezoneSQL)).WithArgs("Asia/Jakarta", "N").WillReturnError(&mysqldriver.MySQLError{Number: 1062})
	if err := repository.Insert(context.Background(), timezones.Timezone{Timezone: "Asia/Jakarta", IsDefault: false}); err != timezones.ErrDuplicate {
		t.Fatalf("duplicate insert error = %v, want duplicate", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryDeleteForbidsDefaultAndMissing(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(getDefaultTimezoneExistsSQL)).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"ID"}).AddRow(1))
	repository := NewMySQLRepository(database)
	if err := repository.Delete(context.Background(), 1); err != timezones.ErrCannotDeleteDefault {
		t.Fatalf("default delete error = %v, want cannot delete default", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(getDefaultTimezoneExistsSQL)).WithArgs(int64(99)).WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	mock.ExpectExec(regexp.QuoteMeta(deleteTimezoneSQL)).WithArgs(int64(99)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(timezoneExistsSQL)).WithArgs(int64(99)).WillReturnRows(sqlmock.NewRows([]string{"ID"}))
	if err := repository.Delete(context.Background(), 99); !errors.Is(err, timezones.ErrNotFound) {
		t.Fatalf("missing delete error = %v, want not found", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

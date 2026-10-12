package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/holidaycalendar"
)

func TestMySQLRepositoryReadsWritesAndDeletesHolidays(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	createdAt := time.Date(2026, time.August, 1, 4, 0, 0, 0, time.UTC)
	date := time.Date(2026, time.August, 17, 0, 0, 0, 0, time.UTC)
	columns := []string{"Tanggal", "Keterangan", "Tanggal_Tambah", "Ditambah_Oleh"}
	mock.ExpectQuery(regexp.QuoteMeta(listHolidaysSQL)).WillReturnRows(sqlmock.NewRows(columns).AddRow(date, "Kemerdekaan", createdAt, "creator").AddRow(date.AddDate(0, 0, 1), "Cuti Bersama", nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta(getHolidaySQL)).WithArgs("2026-08-17").WillReturnRows(sqlmock.NewRows(columns).AddRow(date, "Kemerdekaan", createdAt, "creator"))
	mock.ExpectExec(regexp.QuoteMeta(insertHolidaySQL)).WithArgs("2026-08-17", "Kemerdekaan", &createdAt, "creator").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(updateHolidaySQL)).WithArgs("HUT RI", "2026-08-17").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(deleteHolidaySQL)).WithArgs("2026-08-17").WillReturnResult(sqlmock.NewResult(0, 1))

	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil || len(values) != 2 || values[0].Date != "2026-08-17" || values[0].CreatedBy != "creator" || values[1].CreatedAt != nil || values[1].CreatedBy != "" {
		t.Fatalf("List() = %#v, error = %v", values, err)
	}
	value, err := repository.Get(context.Background(), "2026-08-17")
	if err != nil || value.Date != "2026-08-17" || value.Description != "Kemerdekaan" {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	if err := repository.Insert(context.Background(), holidaycalendar.Holiday{Date: "2026-08-17", Description: "Kemerdekaan", CreatedAt: &createdAt, CreatedBy: "creator"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Update(context.Background(), holidaycalendar.Holiday{Date: "2026-08-17", Description: "HUT RI"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), "2026-08-17"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	// The DATE primary key must never be part of the UPDATE assignment list.
	if strings.Contains(updateHolidaySQL, "SET Tanggal") {
		t.Fatal("holiday update must not rewrite the date primary key")
	}
}

func TestMySQLRepositoryUpdateAndDeleteMissingDateAreNotFound(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectExec(regexp.QuoteMeta(updateHolidaySQL)).WithArgs("HUT RI", "2026-01-01").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(holidayExistsSQL)).WithArgs("2026-01-01").WillReturnRows(sqlmock.NewRows([]string{"Tanggal"}))
	mock.ExpectExec(regexp.QuoteMeta(deleteHolidaySQL)).WithArgs("2026-01-02").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(holidayExistsSQL)).WithArgs("2026-01-02").WillReturnRows(sqlmock.NewRows([]string{"Tanggal"}))
	repository := NewMySQLRepository(database)
	if err := repository.Update(context.Background(), holidaycalendar.Holiday{Date: "2026-01-01", Description: "HUT RI"}); err != holidaycalendar.ErrNotFound {
		t.Fatalf("missing update error = %v, want not found", err)
	}
	if err := repository.Delete(context.Background(), "2026-01-02"); err != holidaycalendar.ErrNotFound {
		t.Fatalf("missing delete error = %v, want not found", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryMapsDuplicateDateToAlreadyExists(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectExec(regexp.QuoteMeta(insertHolidaySQL)).WithArgs("2026-08-17", "Kemerdekaan", nil, "creator").WillReturnError(&mysqldriver.MySQLError{Number: 1062})
	repository := NewMySQLRepository(database)
	if err := repository.Insert(context.Background(), holidaycalendar.Holiday{Date: "2026-08-17", Description: "Kemerdekaan", CreatedBy: "creator"}); err != holidaycalendar.ErrAlreadyExists {
		t.Fatalf("duplicate insert error = %v, want already exists", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

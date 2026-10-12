package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/complainttypes"
)

func TestMySQLRepositoryListsAndGetsComplaintTypes(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	updatedAt := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(listComplaintTypesSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Jenis", "Deskripsi", "Status_Aktif", "Update_Terakhir"}).
			AddRow(1, "Pengaduan", "Y", updatedAt).
			AddRow(2, nil, "N", nil))
	mock.ExpectQuery(regexp.QuoteMeta(getComplaintTypeSQL)).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Jenis", "Deskripsi", "Status_Aktif", "Update_Terakhir"}).AddRow(1, "Pengaduan", "Y", updatedAt))

	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0].Code != 1 || !values[0].Active || values[0].Description == nil || *values[0].Description != "Pengaduan" || values[1].Active || values[1].Description != nil {
		t.Fatalf("unexpected complaint type list: %#v", values)
	}
	value, err := repository.Get(context.Background(), 1)
	if err != nil || value.Code != 1 || value.UpdatedAt == nil || !value.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryCreatesAndMapsDuplicateComplaintType(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	updatedAt := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	description := "Pengaduan"
	mock.ExpectExec(regexp.QuoteMeta(insertComplaintTypeSQL)).
		WithArgs(int64(7), description, "Y", updatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(insertComplaintTypeSQL)).
		WithArgs(int64(7), description, "Y", updatedAt).
		WillReturnError(&mysqldriver.MySQLError{Number: 1062})

	repository := NewMySQLRepository(database)
	if err := repository.Insert(context.Background(), complainttypes.ComplaintType{Code: 7, Description: &description, Active: true, UpdatedAt: &updatedAt}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Insert(context.Background(), complainttypes.ComplaintType{Code: 7, Description: &description, Active: true, UpdatedAt: &updatedAt}); !errors.Is(err, complainttypes.ErrAlreadyExists) {
		t.Fatalf("duplicate Insert() error = %v, want duplicate code", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpdatesAndDeactivatesWithoutDeletingCode(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	updatedAt := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	description := "Updated"
	mock.ExpectExec(regexp.QuoteMeta(updateComplaintTypeSQL)).
		WithArgs(description, "N", updatedAt, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(deactivateComplaintTypeSQL)).
		WithArgs(updatedAt, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(deactivateComplaintTypeSQL)).
		WithArgs(updatedAt, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(complaintTypeExistsSQL)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"Kode_Jenis"}).AddRow(7))

	repository := NewMySQLRepository(database)
	if err := repository.Update(context.Background(), complainttypes.ComplaintType{Code: 7, Description: &description, UpdatedAt: &updatedAt}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Deactivate(context.Background(), 7, updatedAt); err != nil {
		t.Fatal(err)
	}
	if err := repository.Deactivate(context.Background(), 7, updatedAt); err != nil {
		t.Fatalf("repeated deactivate should be idempotent: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryReturnsNotFoundForMissingComplaintType(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(getComplaintTypeSQL)).WithArgs(int64(99)).WillReturnError(sql.ErrNoRows)

	_, err = NewMySQLRepository(database).Get(context.Background(), 99)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Get() error = %v, want sql.ErrNoRows", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

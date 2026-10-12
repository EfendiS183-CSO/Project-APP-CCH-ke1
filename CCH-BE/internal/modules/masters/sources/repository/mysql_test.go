package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/masters/sources"
)

func TestMySQLRepositoryListsAndGetsSources(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	createdAt := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	mock.ExpectQuery(regexp.QuoteMeta(listSourcesSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"ID", "Sumber", "Apakah_Aktif", "Tanggal_Tambah", "Tanggal_Ubah"}).
			AddRow(1, "Web", "Y", createdAt, nil).
			AddRow(2, "Telepon", "N", createdAt, updatedAt))
	mock.ExpectQuery(regexp.QuoteMeta(getSourceSQL)).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"ID", "Sumber", "Apakah_Aktif", "Tanggal_Tambah", "Tanggal_Ubah"}).AddRow(1, "Web", "Y", createdAt, nil))

	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0].ID != 1 || !values[0].Active || values[0].CreatedAt == nil || values[0].UpdatedAt != nil || values[1].Active || values[1].UpdatedAt == nil {
		t.Fatalf("unexpected sources: %#v", values)
	}
	value, err := repository.Get(context.Background(), 1)
	if err != nil || value.ID != 1 || value.Name != "Web" || !value.Active {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryInsertsUpdatesAndDeactivatesSources(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	createdAt := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	mock.ExpectExec(regexp.QuoteMeta(insertSourceSQL)).WithArgs("Portal", "Y", &createdAt).
		WillReturnResult(sqlmock.NewResult(22, 1))
	mock.ExpectExec(regexp.QuoteMeta(updateSourceSQL)).WithArgs("Aplikasi", "N", &updatedAt, int64(22)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(deactivateSourceSQL)).WithArgs(updatedAt, int64(22)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(deactivateSourceSQL)).WithArgs(updatedAt, int64(22)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(sourceExistsSQL)).WithArgs(int64(22)).
		WillReturnRows(sqlmock.NewRows([]string{"ID"}).AddRow(22))

	repository := NewMySQLRepository(database)
	created, err := repository.Insert(context.Background(), sources.Source{Name: "Portal", Active: true, CreatedAt: &createdAt})
	if err != nil || created.ID != 22 {
		t.Fatalf("Insert() = %#v, error = %v", created, err)
	}
	if err := repository.Update(context.Background(), sources.Source{ID: 22, Name: "Aplikasi", UpdatedAt: &updatedAt}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Deactivate(context.Background(), 22, updatedAt); err != nil {
		t.Fatal(err)
	}
	if err := repository.Deactivate(context.Background(), 22, updatedAt); err != nil {
		t.Fatalf("repeated deactivation should be idempotent: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryReturnsNotFoundForMissingSource(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(getSourceSQL)).WithArgs(int64(99)).WillReturnError(sql.ErrNoRows)
	_, err = NewMySQLRepository(database).Get(context.Background(), 99)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Get() error = %v, want sql.ErrNoRows", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/masters/handlingstatuses"
)

func TestMySQLRepositoryUsesLegacyStatusFieldsAndSoftDelete(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	columns := []string{"ID_Status", "Deskripsi_Status", "Aktif"}
	mock.ExpectQuery(regexp.QuoteMeta(listStatusesSQL)).WillReturnRows(sqlmock.NewRows(columns).AddRow("103", "Konfirmasi", "Y").AddRow("105", "Legacy", "N"))
	mock.ExpectQuery(regexp.QuoteMeta(getStatusSQL)).WithArgs("103").WillReturnRows(sqlmock.NewRows(columns).AddRow("103", "Konfirmasi", "Y"))
	mock.ExpectExec(regexp.QuoteMeta(insertStatusSQL)).WithArgs("107", "Tinjau", "Y").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(updateStatusSQL)).WithArgs("Konfirmasi Baru", "N", "103").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(deactivateStatusSQL)).WithArgs("103").WillReturnResult(sqlmock.NewResult(0, 1))
	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil || len(values) != 2 || !values[0].Active || values[1].Active {
		t.Fatalf("List() = %#v, error = %v", values, err)
	}
	value, err := repository.Get(context.Background(), "103")
	if err != nil || value.Code != "103" || !value.Active {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	if err := repository.Insert(context.Background(), handlingstatuses.Status{Code: "107", Description: "Tinjau", Active: true}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Update(context.Background(), handlingstatuses.Status{Code: "103", Description: "Konfirmasi Baru", Active: false}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Deactivate(context.Background(), "103"); err != nil {
		t.Fatal(err)
	}
	if deactivateStatusSQL[:6] != "UPDATE" {
		t.Fatal("status deactivation must preserve historical status references")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRepeatedDeactivateIsIdempotent(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectExec(regexp.QuoteMeta(deactivateStatusSQL)).WithArgs("103").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(statusExistsSQL)).WithArgs("103").WillReturnRows(sqlmock.NewRows([]string{"ID_Status"}).AddRow("103"))
	mock.ExpectExec(regexp.QuoteMeta(deactivateStatusSQL)).WithArgs("999").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(statusExistsSQL)).WithArgs("999").WillReturnRows(sqlmock.NewRows([]string{"ID_Status"}))
	repository := NewMySQLRepository(database)
	if err := repository.Deactivate(context.Background(), "103"); err != nil {
		t.Fatalf("repeated deactivate: %v", err)
	}
	if err := repository.Deactivate(context.Background(), "999"); err != handlingstatuses.ErrNotFound {
		t.Fatalf("missing status error = %v, want not found", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/regionals"
)

func TestMySQLRepositoryListsGetsAndLookupOptions(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	columns := []string{"Kode_Kantor", "Nama_Kantor", "Singkatan", "Jenis", "Regional", "Apakah_Aktif"}
	mock.ExpectQuery(regexp.QuoteMeta(countRegionalOfficesSQL)).WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(listRegionalOfficesSQL)).WithArgs(25, 0).WillReturnRows(sqlmock.NewRows(columns).AddRow("40005", "Kantor Pusat", "KP", "KP", "40005", "Y"))
	mock.ExpectQuery(regexp.QuoteMeta(getRegionalOfficeSQL)).WithArgs("40005").WillReturnRows(sqlmock.NewRows(columns).AddRow("40005", "Kantor Pusat", "KP", "KP", "40005", "Y"))
	mock.ExpectQuery(regexp.QuoteMeta(listRegionalLookupSQL)).WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor", "Nama_Kantor", "Jenis"}).AddRow("40005", "Kantor Pusat", "KP"))

	repository := NewMySQLRepository(database)
	page, err := repository.List(context.Background(), regionals.ListQuery{})
	if err != nil || page.Total != 1 || len(page.Data) != 1 || page.Data[0].Code != "40005" || !page.Data[0].Active || page.Limit != 25 {
		t.Fatalf("List() = %#v, error = %v", page, err)
	}
	value, err := repository.Get(context.Background(), "40005")
	if err != nil || value.Name != "Kantor Pusat" {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	options, err := repository.LookupOptions(context.Background())
	if err != nil || len(options) != 1 || options[0].Code != "40005" {
		t.Fatalf("LookupOptions() = %#v, error = %v", options, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpdatesOnlyViewEligibleSourceAndLocksParent(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockRegionalSourceSQL)).WithArgs("40005").WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor"}).AddRow("40005"))
	mock.ExpectQuery(regexp.QuoteMeta(lockAssignedOfficeSQL)).WithArgs("40005").WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor"}).AddRow("40005"))
	mock.ExpectExec(regexp.QuoteMeta(updateRegionalSourceSQL)).WithArgs("Kantor Pusat Baru", "KP", "KP", "40005", "Y", "40005").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repository := NewMySQLRepository(database)
	value := regionals.Office{Code: "40005", Name: "Kantor Pusat Baru", Abbreviation: "KP", Type: "KP", RegionalCode: "40005", Active: true}
	if err := repository.Update(context.Background(), value); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsSourceOutsideViewAndUnknownRegionalReference(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockRegionalSourceSQL)).WithArgs("OFFICE1").WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor"}))
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockRegionalSourceSQL)).WithArgs("40005").WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor"}).AddRow("40005"))
	mock.ExpectQuery(regexp.QuoteMeta(lockAssignedOfficeSQL)).WithArgs("NOPE").WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor"}))
	mock.ExpectRollback()

	repository := NewMySQLRepository(database)
	if err := repository.Update(context.Background(), regionals.Office{Code: "OFFICE1", Name: "Office", Abbreviation: "O", Type: "KP", RegionalCode: "40005"}); err != regionals.ErrNotFound {
		t.Fatalf("out-of-view source update = %v, want not found", err)
	}
	if err := repository.Update(context.Background(), regionals.Office{Code: "40005", Name: "Office", Abbreviation: "KP", Type: "KP", RegionalCode: "NOPE"}); err != regionals.ErrInvalidRegional {
		t.Fatalf("unknown regional reference = %v, want invalid regional", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

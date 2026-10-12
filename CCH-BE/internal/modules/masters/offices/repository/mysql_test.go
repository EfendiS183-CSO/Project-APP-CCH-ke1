package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/masters/offices"
)

func TestMySQLRepositoryReadsOfficeAndLegacyLookupOptions(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	columns := []string{"Kode_Kantor", "Nama_Kantor", "Singkatan", "Jenis", "Regional", "Apakah_Aktif", "Sebagai_Tujuan", "Sebagai_Tembusan"}
	mock.ExpectQuery(regexp.QuoteMeta(listOfficesSQL)).WillReturnRows(sqlmock.NewRows(columns).AddRow("40005CC", "Kantor Pilot", "KP", "UPT", "40005", "Y", "Y", "N"))
	mock.ExpectQuery(regexp.QuoteMeta(getOfficeSQL)).WithArgs("40005CC").WillReturnRows(sqlmock.NewRows(columns).AddRow("40005CC", "Kantor Pilot", "KP", "UPT", "40005", "Y", "Y", "N"))
	mock.ExpectQuery(regexp.QuoteMeta(listOfficeTypesSQL)).WillReturnRows(sqlmock.NewRows([]string{"Jenis"}).AddRow("UPT").AddRow("Divre"))
	mock.ExpectQuery(regexp.QuoteMeta(listRegionalOfficesSQL)).WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor", "Nama_Kantor", "Jenis"}).AddRow("40005", "Kantor Pusat", "KP"))
	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil || len(values) != 1 || !values[0].Active || !values[0].AsDestination || values[0].AsCopy {
		t.Fatalf("List() = %#v, error = %v", values, err)
	}
	value, err := repository.Get(context.Background(), "40005CC")
	if err != nil || value.RegionalCode != "40005" {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	options, err := repository.LookupOptions(context.Background())
	if err != nil || len(options.Types) != 2 || len(options.RegionalOffices) != 1 || options.RegionalOffices[0].Type != "KP" {
		t.Fatalf("LookupOptions() = %#v, error = %v", options, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryWritesAndSoftDeactivatesOffice(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	value := offices.Office{Code: "40006CC", Name: "Kantor Baru", Abbreviation: "KB", Type: "UPT", RegionalCode: "40005", Active: true, AsDestination: true, AsCopy: false}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(officeTypeExistsSQL)).WithArgs("UPT").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(regionalOfficeExistsSQL)).WithArgs("40005").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(regexp.QuoteMeta(insertOfficeSQL)).WithArgs("40006CC", "Kantor Baru", "KB", "UPT", "40005", "Y", "Y", "N").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockOfficeSQL)).WithArgs("40006CC").WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor"}).AddRow("40006CC"))
	mock.ExpectQuery(regexp.QuoteMeta(officeTypeExistsSQL)).WithArgs("UPT").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(regionalOfficeExistsSQL)).WithArgs("40005").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(regexp.QuoteMeta(updateOfficeSQL)).WithArgs("Kantor Baru", "KB", "UPT", "40005", "Y", "Y", "N", "40006CC").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockOfficeSQL)).WithArgs("40006CC").WillReturnRows(sqlmock.NewRows([]string{"Kode_Kantor"}).AddRow("40006CC"))
	mock.ExpectExec(regexp.QuoteMeta(deactivateOfficeSQL)).WithArgs("40006CC").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	repository := NewMySQLRepository(database)
	if err := repository.Insert(context.Background(), value); err != nil {
		t.Fatal(err)
	}
	if err := repository.Update(context.Background(), value); err != nil {
		t.Fatal(err)
	}
	if err := repository.Deactivate(context.Background(), value.Code); err != nil {
		t.Fatal(err)
	}
	if deactivateOfficeSQL[:6] != "UPDATE" {
		t.Fatal("office deletion must retain existing references")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsInvalidRegionalAndOfficeTypeReferences(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(officeTypeExistsSQL)).WithArgs("UPT").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(regionalOfficeExistsSQL)).WithArgs("99999").WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(officeTypeExistsSQL)).WithArgs("NEW").WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectRollback()
	repository := NewMySQLRepository(database)
	err = repository.Insert(context.Background(), offices.Office{Code: "40006CC", Name: "Kantor Baru", Abbreviation: "KB", Type: "UPT", RegionalCode: "99999"})
	if err != offices.ErrInvalidRegional {
		t.Fatalf("Insert invalid regional error = %v", err)
	}
	err = repository.Insert(context.Background(), offices.Office{Code: "40006CC", Name: "Kantor Baru", Abbreviation: "KB", Type: "NEW", RegionalCode: "40005"})
	if err != offices.ErrInvalidType {
		t.Fatalf("Insert invalid office type error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

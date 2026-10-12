package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/masters/documenttypes"
)

func TestMySQLRepositoryReadsAndWritesDocumentTypes(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	createdAt := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	columns := []string{"Kode_Jenis_Dokumen", "Deskripsi_Jenis_Dokumen", "Status_Aktif", "Tanggal_Tambah", "Ditambah_Oleh", "Tanggal_Ubah", "Diubah_Oleh"}
	mock.ExpectQuery(regexp.QuoteMeta(listDocumentTypesSQL)).WillReturnRows(sqlmock.NewRows(columns).AddRow("PDF", "Dokumen", "Y", createdAt, "creator", updatedAt, "editor").AddRow("IMG", "Gambar", "N", nil, nil, nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta(getDocumentTypeSQL)).WithArgs("PDF").WillReturnRows(sqlmock.NewRows(columns).AddRow("PDF", "Dokumen", "Y", createdAt, "creator", updatedAt, "editor"))
	mock.ExpectExec(regexp.QuoteMeta(insertDocumentTypeSQL)).WithArgs("ZIP", "Arsip", "Y", &createdAt, "creator", &createdAt, "creator").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(updateDocumentTypeSQL)).WithArgs("Dokumen Revisi", "N", &updatedAt, "editor", "PDF").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(deactivateDocumentTypeSQL)).WithArgs(updatedAt, "editor", "PDF").WillReturnResult(sqlmock.NewResult(0, 1))

	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil || len(values) != 2 || values[0].CreatedBy != "creator" || values[0].UpdatedBy != "editor" || values[1].Active {
		t.Fatalf("List() = %#v, error = %v", values, err)
	}
	value, err := repository.Get(context.Background(), "PDF")
	if err != nil || value.Code != "PDF" || !value.Active {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	if err := repository.Insert(context.Background(), documenttypes.DocumentType{Code: "ZIP", Description: "Arsip", Active: true, CreatedAt: &createdAt, CreatedBy: "creator", UpdatedAt: &createdAt, UpdatedBy: "creator"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Update(context.Background(), documenttypes.DocumentType{Code: "PDF", Description: "Dokumen Revisi", UpdatedAt: &updatedAt, UpdatedBy: "editor"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Deactivate(context.Background(), "PDF", "editor", updatedAt); err != nil {
		t.Fatal(err)
	}
	if len(deactivateDocumentTypeSQL) < 6 || deactivateDocumentTypeSQL[:6] != "UPDATE" {
		t.Fatal("deactivation must update Status_Aktif and preserve historical document references")
	}
	if !strings.Contains(deactivateDocumentTypeSQL, "Status_Aktif IS NULL OR Status_Aktif <> 'N'") {
		t.Fatal("deactivation must also change nullable legacy active states")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryDeactivateIsIdempotentAndMissingCodeIsNotFound(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	updatedAt := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta(deactivateDocumentTypeSQL)).WithArgs(updatedAt, "editor", "PDF").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(documentTypeExistsSQL)).WithArgs("PDF").WillReturnRows(sqlmock.NewRows([]string{"Kode_Jenis_Dokumen"}).AddRow("PDF"))
	mock.ExpectExec(regexp.QuoteMeta(deactivateDocumentTypeSQL)).WithArgs(updatedAt, "editor", "MSS").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(documentTypeExistsSQL)).WithArgs("MSS").WillReturnRows(sqlmock.NewRows([]string{"Kode_Jenis_Dokumen"}))
	repository := NewMySQLRepository(database)
	if err := repository.Deactivate(context.Background(), "PDF", "editor", updatedAt); err != nil {
		t.Fatalf("repeated deactivation = %v", err)
	}
	if err := repository.Deactivate(context.Background(), "MSS", "editor", updatedAt); err != documenttypes.ErrNotFound {
		t.Fatalf("missing deactivation error = %v, want not found", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

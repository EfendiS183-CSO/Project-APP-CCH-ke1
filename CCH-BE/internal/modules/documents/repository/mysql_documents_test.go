package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/documents"
)

func TestMySQLRepositoryListsMetadataWithoutReturningFileNamesOrPaths(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	addedAt := time.Date(2026, time.September, 30, 10, 0, 0, 0, time.UTC)
	query := documents.Query{ComplaintID: "TKT-001", OfficeCode: "40005CC", Limit: 10, Offset: 20}
	mock.ExpectQuery(regexp.QuoteMeta(callCenterTicketDocumentsSQL)).
		WithArgs("TKT-001", "TKT-001", 10, 20, "TKT-001", "40005CC", "40005CC").
		WillReturnRows(sqlmock.NewRows([]string{
			"ID_Pengaduan", "ID_Histori", "Jenis_Dokumen", "Deskripsi_Jenis_Dokumen", "Kantor_Upload", "Tanggal_Tambah", "File_Count",
		}).AddRow("TKT-001", "HIST-001", "BA1", "Berita Acara", "40005CC", addedAt, 2).
			AddRow("TKT-001", nil, nil, nil, nil, nil, nil))

	items, err := NewMySQLRepository(database).ListForOpenCallCenterTicket(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("metadata count = %d, want 1", len(items))
	}
	item := items[0]
	if item.ComplaintID != "TKT-001" || item.HistoryID != "HIST-001" || item.DocumentTypeCode != "BA1" || item.DocumentType != "Berita Acara" || item.UploadOfficeCode != "40005CC" || item.FileCount != 2 || !item.AddedAt.Equal(addedAt) {
		t.Fatalf("metadata = %#v", item)
	}
	for _, forbiddenProjection := range []string{"d.Folder", "\n\t\td.Nama_File,\n", "d.Nama_File AS", "d.Folder AS"} {
		if strings.Contains(callCenterTicketDocumentsSQL, forbiddenProjection) {
			t.Errorf("document query exposes forbidden projection %q", forbiddenProjection)
		}
	}
	for _, requiredClause := range []string{
		"d.ID_Pengaduan = ?",
		"FROM cch_be_document_files AS d",
		"UNION ALL",
		"c.Status_Akhir <> '109'",
		"c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0",
		"INNER JOIN cchentrimas AS h",
		"LIMIT ? OFFSET ?",
	} {
		if !strings.Contains(callCenterTicketDocumentsSQL, requiredClause) {
			t.Errorf("document query missing required clause %q", requiredClause)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryReturnsEmptyForVisibleTicketWithoutDocuments(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(callCenterTicketDocumentsSQL)).
		WillReturnRows(sqlmock.NewRows([]string{
			"ID_Pengaduan", "ID_Histori", "Jenis_Dokumen", "Deskripsi_Jenis_Dokumen", "Kantor_Upload", "Tanggal_Tambah", "File_Count",
		}).AddRow("TKT-001", nil, nil, nil, nil, nil, nil))

	items, err := NewMySQLRepository(database).ListForOpenCallCenterTicket(context.Background(), documents.Query{
		ComplaintID: "TKT-001", OfficeCode: "40005CC",
	})
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("items/error = %#v/%v, want empty slice and no error", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryHidesUnknownOrOutOfScopeComplaint(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(callCenterTicketDocumentsSQL)).
		WillReturnRows(sqlmock.NewRows([]string{
			"ID_Pengaduan", "ID_Histori", "Jenis_Dokumen", "Deskripsi_Jenis_Dokumen", "Kantor_Upload", "Tanggal_Tambah", "File_Count",
		}))
	items, err := NewMySQLRepository(database).ListForOpenCallCenterTicket(context.Background(), documents.Query{
		ComplaintID: "TKT-OUTSIDE", OfficeCode: "40005CC",
	})
	if !errors.Is(err, sql.ErrNoRows) || items != nil {
		t.Fatalf("items/error = %#v/%v, want no rows", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsInvalidQueryBeforeDatabaseAccess(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = NewMySQLRepository(database).ListForOpenCallCenterTicket(context.Background(), documents.Query{
		ComplaintID: "TKT-001", OfficeCode: "", Limit: documents.MaxPageSize + 1,
	})
	if !errors.Is(err, documents.ErrInvalidQuery) {
		t.Fatalf("ListForOpenCallCenterTicket() error = %v, want invalid query", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

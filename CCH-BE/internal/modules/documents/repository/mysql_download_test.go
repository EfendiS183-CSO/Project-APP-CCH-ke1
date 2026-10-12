package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/documents"
)

func TestMySQLRepositoryGetsDownloadObjectOnlyThroughVisibleTicket(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(getCallCenterDocumentSQL)).
		WithArgs("0123456789abcdef0123456789abcdef", "TKT-001", "40005CC", "40005CC").
		WillReturnRows(sqlmock.NewRows([]string{"document_id", "storage_key", "media_type", "file_extension", "file_size", "sha256"}).
			AddRow("0123456789abcdef0123456789abcdef", strings.Repeat("a", 64), "application/pdf", ".pdf", int64(42), strings.Repeat("b", 64)))

	metadata, err := NewMySQLRepository(database).GetCallCenterDocument(context.Background(), "TKT-001", "0123456789abcdef0123456789abcdef", "40005CC")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.DocumentID != "0123456789abcdef0123456789abcdef" || metadata.MediaType != "application/pdf" || metadata.Extension != ".pdf" || metadata.FileSize != 42 || metadata.StorageKey == "" {
		t.Fatalf("download metadata = %#v", metadata)
	}
	for _, requiredClause := range []string{
		"d.document_id = ?",
		"d.complaint_id = ?",
		"c.Status_Akhir <> '109'",
		"c.Kantor_Asal = ? OR FIND_IN_SET(?, REPLACE(COALESCE(c.Semua_Tujuan, ''), ' ', '')) > 0",
		"INNER JOIN cchentrimas AS h",
	} {
		if !strings.Contains(getCallCenterDocumentSQL, requiredClause) {
			t.Errorf("download query missing %q", requiredClause)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryHidesMissingOrOutOfScopeDocument(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(getCallCenterDocumentSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"document_id", "storage_key", "media_type", "file_extension", "file_size", "sha256"}))
	_, err = NewMySQLRepository(database).GetCallCenterDocument(context.Background(), "TKT-OUTSIDE", "0123456789abcdef0123456789abcdef", "40005CC")
	if !errors.Is(err, sql.ErrNoRows) || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetCallCenterDocument() error = %v, want no rows", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

var _ documents.DownloadRepository = (*MySQLRepository)(nil)

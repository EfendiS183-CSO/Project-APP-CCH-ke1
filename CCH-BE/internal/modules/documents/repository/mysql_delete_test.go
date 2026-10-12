package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/documents"
)

func TestMySQLRepositoryDeletesOfficeOwnedDocumentTransactionally(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	documentID := "0123456789abcdef0123456789abcdef"
	storageKey := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(deleteDocumentContextSQL)).
		WithArgs(documentID, "TKT-001", "40005CC", "40005CC").
		WillReturnRows(sqlmock.NewRows([]string{"storage_key", "office_code"}).AddRow(storageKey, "40005CC"))
	mock.ExpectExec(regexp.QuoteMeta(deleteBEDocumentSQL)).
		WithArgs(documentID, "TKT-001").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := NewMySQLRepository(database).DeleteCallCenterDocument(context.Background(), "TKT-001", documentID, "40005CC")
	if err != nil {
		t.Fatal(err)
	}
	if result.ComplaintID != "TKT-001" || result.DocumentID != documentID || result.StorageKey != storageKey {
		t.Fatalf("unexpected delete result: %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryDeleteHidesMissingDocumentAndRejectsForeignOffice(t *testing.T) {
	documentID := "0123456789abcdef0123456789abcdef"
	for _, test := range []struct {
		name         string
		uploadOffice string
		queryOffice  string
		wantErr      error
	}{
		{name: "missing document", queryOffice: "40005CC", uploadOffice: "", wantErr: sql.ErrNoRows},
		{name: "foreign office document", queryOffice: "40005CC", uploadOffice: "40006CC", wantErr: documents.ErrDeleteUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			mock.ExpectBegin()
			if test.wantErr == sql.ErrNoRows {
				mock.ExpectQuery(regexp.QuoteMeta(deleteDocumentContextSQL)).
					WithArgs(documentID, "TKT-001", "40005CC", "40005CC").
					WillReturnRows(sqlmock.NewRows([]string{"storage_key", "office_code"}))
			} else {
				mock.ExpectQuery(regexp.QuoteMeta(deleteDocumentContextSQL)).
					WithArgs(documentID, "TKT-001", "40005CC", "40005CC").
					WillReturnRows(sqlmock.NewRows([]string{"storage_key", "office_code"}).AddRow("key-value", test.uploadOffice))
				mock.ExpectRollback()
			}
			_, err = NewMySQLRepository(database).DeleteCallCenterDocument(context.Background(), "TKT-001", documentID, test.queryOffice)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("DeleteCallCenterDocument() error = %v, want %v", err, test.wantErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

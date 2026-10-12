package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/masters/followupstatuses"
)

func TestMySQLRepositoryReadsAndWritesFollowUpStatuses(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	description := "Telepon"
	columns := []string{"Kode_FollowUp", "Deskripsi_FollowUp"}
	mock.ExpectQuery(regexp.QuoteMeta(listFollowUpStatusesSQL)).WillReturnRows(sqlmock.NewRows(columns).AddRow("001", description).AddRow("002", nil))
	mock.ExpectQuery(regexp.QuoteMeta(getFollowUpStatusSQL)).WithArgs("001").WillReturnRows(sqlmock.NewRows(columns).AddRow("001", description))
	mock.ExpectExec(regexp.QuoteMeta(insertFollowUpStatusSQL)).WithArgs("003", &description).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(updateFollowUpStatusSQL)).WithArgs(&description, "001").WillReturnResult(sqlmock.NewResult(0, 1))
	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil || len(values) != 2 || values[0].Description == nil || *values[0].Description != description || values[1].Description != nil {
		t.Fatalf("List() = %#v, error = %v", values, err)
	}
	value, err := repository.Get(context.Background(), "001")
	if err != nil || value.Description == nil || *value.Description != description {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	if err := repository.Insert(context.Background(), followupstatuses.Status{Code: "003", Description: &description}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Update(context.Background(), followupstatuses.Status{Code: "001", Description: &description}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryPreventsDeletingReferencedStatus(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockFollowUpStatusSQL)).WithArgs("001").WillReturnRows(sqlmock.NewRows([]string{"Kode_FollowUp"}).AddRow("001"))
	mock.ExpectQuery(regexp.QuoteMeta(followUpStatusInUseSQL)).WithArgs("001").WillReturnRows(sqlmock.NewRows([]string{"ID_Pengaduan"}).AddRow("TICKET-1"))
	mock.ExpectRollback()
	err = NewMySQLRepository(database).Delete(context.Background(), "001")
	if err != followupstatuses.ErrInUse {
		t.Fatalf("Delete referenced status error = %v, want ErrInUse", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryDeletesOnlyUnreferencedStatus(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockFollowUpStatusSQL)).WithArgs("001").WillReturnRows(sqlmock.NewRows([]string{"Kode_FollowUp"}).AddRow("001"))
	mock.ExpectQuery(regexp.QuoteMeta(followUpStatusInUseSQL)).WithArgs("001").WillReturnRows(sqlmock.NewRows([]string{"ID_Pengaduan"}))
	mock.ExpectExec(regexp.QuoteMeta(deleteFollowUpStatusSQL)).WithArgs("001").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := NewMySQLRepository(database).Delete(context.Background(), "001"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/breadcrumbs"
)

func TestMySQLRepositoryTransactionRunsNestedSetQueries(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewMySQLRepository(database)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE cch_breadcrumblinks SET Lft = Lft + 2 WHERE Lft >= ?")).
		WithArgs(3).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE cch_breadcrumblinks SET Rgt = Rgt + 2 WHERE Rgt > ?")).
		WithArgs(3).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO cch_breadcrumblinks (Page_Title, Page_URL, Lft, Rgt) VALUES (?, ?, ?, ?)")).
		WithArgs("Contact", "/contact", 3, 4).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repository.Transaction(context.Background(), func(ctx context.Context, q breadcrumbs.Querier) error {
		return q.Add(ctx, "Contact", "/contact", "Home", 3, 4)
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryAddRejectsMissingRoot(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewMySQLRepository(database)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO cch_breadcrumblinks (Page_Title, Page_URL, Lft, Rgt) VALUES (?, ?, ?, ?)")).
		WithArgs("Child", "/child", 1, 2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = repository.Transaction(context.Background(), func(ctx context.Context, q breadcrumbs.Querier) error {
		return q.Add(ctx, "Child", "/child", "Missing", 1, 2)
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

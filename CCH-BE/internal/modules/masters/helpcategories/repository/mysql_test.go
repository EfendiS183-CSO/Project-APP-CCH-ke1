package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/masters/helpcategories"
)

func TestMySQLRepositoryListsCategoriesAndLanguageOptions(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(listCategoriesSQL)).WillReturnRows(
		sqlmock.NewRows([]string{"Category_ID", "Language", "Category_Description"}).
			AddRow(1, "id", "Pendaftaran").AddRow(2, "en", "Login"))
	mock.ExpectQuery(regexp.QuoteMeta(listLanguagesSQL)).WillReturnRows(
		sqlmock.NewRows([]string{"Language_Code", "Language_Name"}).AddRow("en", "English").AddRow("id", "Indonesia"))
	mock.ExpectQuery(regexp.QuoteMeta(getCategorySQL)).WithArgs(int64(1)).WillReturnRows(
		sqlmock.NewRows([]string{"Category_ID", "Language", "Category_Description"}).AddRow(1, "id", "Pendaftaran"))

	repository := NewMySQLRepository(database)
	categories, err := repository.List(context.Background())
	if err != nil || len(categories) != 2 || categories[0] != (helpcategories.Category{ID: 1, Language: "id", Description: "Pendaftaran"}) {
		t.Fatalf("List() = %#v, error = %v", categories, err)
	}
	languages, err := repository.Languages(context.Background())
	if err != nil || len(languages) != 2 || languages[0] != (helpcategories.Language{Code: "en", Name: "English"}) {
		t.Fatalf("Languages() = %#v, error = %v", languages, err)
	}
	category, err := repository.Get(context.Background(), 1)
	if err != nil || category.ID != 1 || category.Language != "id" {
		t.Fatalf("Get() = %#v, error = %v", category, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryCreatesAndUpdatesOnlyForKnownLanguage(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(languageExistsSQL)).WithArgs("id").WillReturnRows(sqlmock.NewRows([]string{"Language_Code"}).AddRow("id"))
	mock.ExpectExec(regexp.QuoteMeta(insertCategorySQL)).WithArgs("id", "Masuk").WillReturnResult(sqlmock.NewResult(8, 1))
	mock.ExpectQuery(regexp.QuoteMeta(languageExistsSQL)).WithArgs("en").WillReturnRows(sqlmock.NewRows([]string{"Language_Code"}).AddRow("en"))
	mock.ExpectExec(regexp.QuoteMeta(updateCategorySQL)).WithArgs("en", "Sign in", int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(languageExistsSQL)).WithArgs("xx").WillReturnError(sql.ErrNoRows)

	repository := NewMySQLRepository(database)
	created, err := repository.Insert(context.Background(), helpcategories.Category{Language: "id", Description: "Masuk"})
	if err != nil || created.ID != 8 {
		t.Fatalf("Insert() = %#v, error = %v", created, err)
	}
	if err := repository.Update(context.Background(), helpcategories.Category{ID: 8, Language: "en", Description: "Sign in"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Insert(context.Background(), helpcategories.Category{Language: "xx", Description: "Unknown"}); !errors.Is(err, helpcategories.ErrInvalidLanguage) {
		t.Fatalf("Insert with unknown language error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryDeleteProtectsReferencedHelpEntries(t *testing.T) {
	for _, test := range []struct {
		name       string
		reference  error
		want       error
		deleteRows int64
	}{
		{name: "unused category is deleted", reference: sql.ErrNoRows, deleteRows: 1},
		{name: "referenced category is retained", reference: nil, want: helpcategories.ErrInUse},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(lockCategorySQL)).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"Category_ID"}).AddRow(5))
			referenceQuery := mock.ExpectQuery(regexp.QuoteMeta(categoryInUseSQL)).WithArgs(int64(5))
			if test.reference == sql.ErrNoRows {
				referenceQuery.WillReturnError(sql.ErrNoRows)
				mock.ExpectExec(regexp.QuoteMeta(deleteCategorySQL)).WithArgs(int64(5)).WillReturnResult(sqlmock.NewResult(0, test.deleteRows))
				mock.ExpectCommit()
			} else {
				referenceQuery.WillReturnRows(sqlmock.NewRows([]string{"Help_ID"}).AddRow(12))
				mock.ExpectRollback()
			}
			err = NewMySQLRepository(database).Delete(context.Background(), 5)
			if !errors.Is(err, test.want) {
				t.Fatalf("Delete() error = %v, want %v", err, test.want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMySQLRepositoryDeleteMissingCategoryReturnsNotFound(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockCategorySQL)).WithArgs(int64(404)).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	if err := NewMySQLRepository(database).Delete(context.Background(), 404); !errors.Is(err, helpcategories.ErrNotFound) {
		t.Fatalf("Delete missing category error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

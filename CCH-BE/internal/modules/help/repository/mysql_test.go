package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"cch.local/backend/internal/modules/help"
)

func TestMySQLRepositoryListsSummariesWithoutLongDescription(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	updated := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(listArticlesCountSQL)).WithArgs("id", "id", int64(3), int64(3), "login", "login").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(listArticlesSQL)).WithArgs("id", "id", int64(3), int64(3), "login", "login", 25, 0).
		WillReturnRows(sqlmock.NewRows([]string{"Help_ID", "Language", "Topic", "Category", "Category_Description", "Order", "Display_in_Page", "Updated_By", "Last_Updated"}).
			AddRow(7, "id", "Login", 3, "Akun", 1, "login", "help.manager", updated))

	page, err := NewMySQLRepository(database).List(context.Background(), help.ListQuery{Language: "id", CategoryID: 3, DisplayInPage: "login", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Data) != 1 || page.Data[0].ID != 7 || page.Data[0].CategoryDescription == nil || *page.Data[0].CategoryDescription != "Akun" || page.Data[0].LastUpdated == nil {
		t.Fatalf("unexpected help article page: %#v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryGetsFullArticleAndLookupOptions(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	updated := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(getArticleSQL)).WithArgs(int64(7)).WillReturnRows(
		sqlmock.NewRows([]string{"Help_ID", "Language", "Topic", "Description", "Category", "Category_Description", "Order", "Display_in_Page", "Updated_By", "Last_Updated"}).
			AddRow(7, "id", "Login", "<p>Help text</p>", 3, "Akun", 1, "login", "help.manager", updated))
	mock.ExpectQuery(regexp.QuoteMeta(listLanguagesSQL)).WillReturnRows(
		sqlmock.NewRows([]string{"Language_Code", "Language_Name"}).AddRow("id", "Indonesia"))
	mock.ExpectQuery(regexp.QuoteMeta(listCategoriesSQL)).WillReturnRows(
		sqlmock.NewRows([]string{"Category_ID", "Language", "Category_Description"}).AddRow(3, "id", "Akun"))

	repository := NewMySQLRepository(database)
	article, err := repository.Get(context.Background(), 7)
	if err != nil || article.ID != 7 || article.Description != "<p>Help text</p>" || article.CategoryID != 3 {
		t.Fatalf("Get() = %#v, error = %v", article, err)
	}
	options, err := repository.LookupOptions(context.Background())
	if err != nil || len(options.Languages) != 1 || len(options.Categories) != 1 {
		t.Fatalf("LookupOptions() = %#v, error = %v", options, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryInsertsUpdatesAndDeletesTransactionally(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	updated := time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockLanguageSQL)).WithArgs("id").WillReturnRows(sqlmock.NewRows([]string{"Language_Code"}).AddRow("id"))
	mock.ExpectQuery(regexp.QuoteMeta(lockCategorySQL)).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"Category_ID"}).AddRow(3))
	mock.ExpectExec(regexp.QuoteMeta(insertArticleSQL)).WithArgs("id", "Login", "Help text", int64(3), int32(1), "login", "help.manager").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectQuery(regexp.QuoteMeta(getArticleSQL)).WithArgs(int64(7)).WillReturnRows(articleRows(updated, "Help text"))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockArticleSQL)).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"Help_ID"}).AddRow(7))
	mock.ExpectQuery(regexp.QuoteMeta(lockLanguageSQL)).WithArgs("id").WillReturnRows(sqlmock.NewRows([]string{"Language_Code"}).AddRow("id"))
	mock.ExpectQuery(regexp.QuoteMeta(lockCategorySQL)).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"Category_ID"}).AddRow(3))
	mock.ExpectExec(regexp.QuoteMeta(updateArticleSQL)).WithArgs("id", "Login", "Updated help", int64(3), int32(2), "login", "help.manager", int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(getArticleSQL)).WithArgs(int64(7)).WillReturnRows(articleRows(updated, "Updated help"))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockArticleSQL)).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"Help_ID"}).AddRow(7))
	mock.ExpectExec(regexp.QuoteMeta(deleteArticleSQL)).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repository := NewMySQLRepository(database)
	created, err := repository.Insert(context.Background(), testArticle("Help text"), "help.manager")
	if err != nil || created.ID != 7 || created.UpdatedBy == nil || *created.UpdatedBy != "help.manager" {
		t.Fatalf("Insert() = %#v, error = %v", created, err)
	}
	updatedArticle := testArticle("Updated help")
	updatedArticle.ID = 7
	updatedArticle.Order = 2
	if _, err := repository.Update(context.Background(), updatedArticle, "help.manager"); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsMissingReferencesAndMissingDeleteTarget(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockLanguageSQL)).WithArgs("id").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockLanguageSQL)).WithArgs("id").WillReturnRows(sqlmock.NewRows([]string{"Language_Code"}).AddRow("id"))
	mock.ExpectQuery(regexp.QuoteMeta(lockCategorySQL)).WithArgs(int64(3)).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(lockArticleSQL)).WithArgs(int64(404)).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	repository := NewMySQLRepository(database)
	if _, err := repository.Insert(context.Background(), testArticle("Body"), "help.manager"); !errors.Is(err, help.ErrInvalidLanguage) {
		t.Fatalf("Insert with missing language error = %v", err)
	}
	if _, err := repository.Insert(context.Background(), testArticle("Body"), "help.manager"); !errors.Is(err, help.ErrInvalidCategory) {
		t.Fatalf("Insert with missing category error = %v", err)
	}
	if err := repository.Delete(context.Background(), 404); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Delete missing article error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func articleRows(updated time.Time, description string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"Help_ID", "Language", "Topic", "Description", "Category", "Category_Description", "Order", "Display_in_Page", "Updated_By", "Last_Updated"}).
		AddRow(7, "id", "Login", description, 3, "Akun", 1, "login", "help.manager", updated)
}

func testArticle(description string) help.Article {
	return help.Article{
		ArticleSummary: help.ArticleSummary{Language: "id", Topic: "Login", CategoryID: 3, Order: 1, DisplayInPage: "login"},
		Description:    description,
	}
}

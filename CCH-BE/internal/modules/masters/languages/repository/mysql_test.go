package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/languages"
)

var languageColumns = []string{
	"Language_Code", "Language_Name", "Default", "Site_Logo", "Site_Title",
	"Default_Thousands_Separator", "Default_Decimal_Point", "Default_Currency_Symbol",
	"Default_Money_Thousands_Separator", "Default_Money_Decimal_Point",
	"Terms_And_Condition_Text", "Announcement_Text", "About_Text",
}

func sampleLanguage() languages.Language {
	return languages.Language{
		Code: "ID", Name: "Indonesia", IsDefault: true, SiteLogo: "logo.png", SiteTitle: "CCH",
		TermsAndConditionText: "Syarat", AnnouncementText: "Pengumuman", AboutText: "Tentang",
	}
}

func TestMySQLRepositoryReadsWritesAndDeletesLanguages(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery(regexp.QuoteMeta(listLanguagesSQL)).WillReturnRows(sqlmock.NewRows(languageColumns).AddRow(
		"ID", "Indonesia", "Y", "logo.png", "CCH", ",", ".", "Rp", ",", ".",
		"Syarat", "Pengumuman", "Tentang"))
	mock.ExpectQuery(regexp.QuoteMeta(getLanguageSQL)).WithArgs("ID").WillReturnRows(sqlmock.NewRows(languageColumns).AddRow(
		"ID", "Indonesia", "Y", "logo.png", "CCH", ",", ".", "Rp", ",", ".",
		"Syarat", "Pengumuman", "Tentang"))
	// Insert: EN is non-default, so checkOtherDefault skips the query
	mock.ExpectExec(regexp.QuoteMeta(insertLanguageSQL)).WithArgs(
		"EN", "English", "N", "en.png", "CCH EN",
		sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		"Terms", "News", "About").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Update: ID already default, no other default
	mock.ExpectQuery(regexp.QuoteMeta(otherDefaultLanguageExistsSQL)).WithArgs("ID").WillReturnRows(sqlmock.NewRows([]string{"Language_Code"}))
	mock.ExpectExec(regexp.QuoteMeta(updateLanguageSQL)).WithArgs(
		"Indonesia", "Y", "logo2.png", "CCH2", nil, nil, "$", nil, nil, "Syarat2", "News2", "About2", "ID").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Delete: EN not default
	mock.ExpectQuery(regexp.QuoteMeta(getDefaultLanguageExistsSQL)).WithArgs("EN").WillReturnRows(sqlmock.NewRows([]string{"Language_Code"}))
	mock.ExpectExec(regexp.QuoteMeta(deleteLanguageSQL)).WithArgs("EN").WillReturnResult(sqlmock.NewResult(0, 1))

	repository := NewMySQLRepository(database)
	values, err := repository.List(context.Background())
	if err != nil || len(values) != 1 || values[0].Code != "ID" || !values[0].IsDefault || values[0].DefaultCurrencySymbol == nil || *values[0].DefaultCurrencySymbol != "Rp" {
		t.Fatalf("List() = %#v, error = %v", values, err)
	}
	value, err := repository.Get(context.Background(), "ID")
	if err != nil || value.Name != "Indonesia" {
		t.Fatalf("Get() = %#v, error = %v", value, err)
	}
	en := languages.Language{Code: "EN", Name: "English", SiteLogo: "en.png", SiteTitle: "CCH EN", TermsAndConditionText: "Terms", AnnouncementText: "News", AboutText: "About"}
	if err := repository.Insert(context.Background(), en); err != nil {
		t.Fatal(err)
	}
	updated := languages.Language{Code: "ID", Name: "Indonesia", IsDefault: true, SiteLogo: "logo2.png", SiteTitle: "CCH2", DefaultCurrencySymbol: strPtr("$"), TermsAndConditionText: "Syarat2", AnnouncementText: "News2", AboutText: "About2"}
	if err := repository.Update(context.Background(), updated); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(context.Background(), "EN"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRejectsSecondDefaultDuplicateAndDefaultDelete(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	// Second default: another row already has Default=Y
	mock.ExpectQuery(regexp.QuoteMeta(otherDefaultLanguageExistsSQL)).WithArgs("EN").WillReturnRows(sqlmock.NewRows([]string{"Language_Code"}).AddRow("ID"))
	repository := NewMySQLRepository(database)
	en := languages.Language{Code: "EN", Name: "English", IsDefault: true, SiteLogo: "en.png", SiteTitle: "CCH EN", TermsAndConditionText: "Terms", AnnouncementText: "News", AboutText: "About"}
	if err := repository.Insert(context.Background(), en); err != languages.ErrOnlyOneDefault {
		t.Fatalf("second default insert error = %v, want only one default", err)
	}
	// Duplicate code: no other default, then MySQL 1062
	duplicate := languages.Language{Code: "EN", Name: "English", SiteLogo: "en.png", SiteTitle: "CCH EN", TermsAndConditionText: "Terms", AnnouncementText: "News", AboutText: "About"}
	mock.ExpectExec(regexp.QuoteMeta(insertLanguageSQL)).WithArgs(
		"EN", "English", "N", "en.png", "CCH EN",
		sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		"Terms", "News", "About").
		WillReturnError(&mysqldriver.MySQLError{Number: 1062})
	if err := repository.Insert(context.Background(), duplicate); !errors.Is(err, languages.ErrDuplicate) {
		t.Fatalf("duplicate insert error = %v, want duplicate", err)
	}
	// Delete default forbidden
	mock.ExpectQuery(regexp.QuoteMeta(getDefaultLanguageExistsSQL)).WithArgs("ID").WillReturnRows(sqlmock.NewRows([]string{"Language_Code"}).AddRow("ID"))
	if err := repository.Delete(context.Background(), "ID"); err != languages.ErrCannotDeleteDefault {
		t.Fatalf("default delete error = %v, want cannot delete default", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func strPtr(value string) *string { return &value }

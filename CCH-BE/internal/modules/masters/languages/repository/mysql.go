package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"

	"cch.local/backend/internal/modules/masters/languages"
)

const listLanguagesSQL = `SELECT Language_Code, Language_Name, Default, Site_Logo, Site_Title,
Default_Thousands_Separator, Default_Decimal_Point, Default_Currency_Symbol,
Default_Money_Thousands_Separator, Default_Money_Decimal_Point,
Terms_And_Condition_Text, Announcement_Text, About_Text
FROM cch_languages ORDER BY Language_Code ASC`
const getLanguageSQL = `SELECT Language_Code, Language_Name, Default, Site_Logo, Site_Title,
Default_Thousands_Separator, Default_Decimal_Point, Default_Currency_Symbol,
Default_Money_Thousands_Separator, Default_Money_Decimal_Point,
Terms_And_Condition_Text, Announcement_Text, About_Text
FROM cch_languages WHERE Language_Code = ? LIMIT 1`
const insertLanguageSQL = `INSERT INTO cch_languages
(Language_Code, Language_Name, Default, Site_Logo, Site_Title,
Default_Thousands_Separator, Default_Decimal_Point, Default_Currency_Symbol,
Default_Money_Thousands_Separator, Default_Money_Decimal_Point,
Terms_And_Condition_Text, Announcement_Text, About_Text)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
const updateLanguageSQL = `UPDATE cch_languages SET
Language_Name = ?, Default = ?, Site_Logo = ?, Site_Title = ?,
Default_Thousands_Separator = ?, Default_Decimal_Point = ?, Default_Currency_Symbol = ?,
Default_Money_Thousands_Separator = ?, Default_Money_Decimal_Point = ?,
Terms_And_Condition_Text = ?, Announcement_Text = ?, About_Text = ?
WHERE Language_Code = ?`
const deleteLanguageSQL = `DELETE FROM cch_languages WHERE Language_Code = ? AND Default = 'N'`
const getDefaultLanguageExistsSQL = `SELECT Language_Code FROM cch_languages WHERE Language_Code = ? AND Default = 'Y' LIMIT 1`
const otherDefaultLanguageExistsSQL = `SELECT Language_Code FROM cch_languages WHERE Default = 'Y' AND Language_Code <> ? LIMIT 1`
const languageExistsSQL = `SELECT Language_Code FROM cch_languages WHERE Language_Code = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("language repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ languages.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context) ([]languages.Language, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listLanguagesSQL)
	if err != nil {
		return nil, fmt.Errorf("list languages: %w", err)
	}
	defer rows.Close()
	values := make([]languages.Language, 0)
	for rows.Next() {
		value, err := scanLanguage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan language: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read languages: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, code string) (languages.Language, error) {
	if repository == nil || repository.db == nil {
		return languages.Language{}, ErrRepositoryNotConfigured
	}
	value, err := scanLanguage(repository.db.QueryRowContext(ctx, getLanguageSQL, code))
	if err != nil {
		return languages.Language{}, fmt.Errorf("get language: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value languages.Language) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if err := repository.checkOtherDefault(ctx, value.Code, value.IsDefault); err != nil {
		return err
	}
	if _, err := repository.db.ExecContext(ctx, insertLanguageSQL,
		value.Code, value.Name, boolToEnum(value.IsDefault), value.SiteLogo, value.SiteTitle,
		value.DefaultThousandsSeparator, value.DefaultDecimalPoint, value.DefaultCurrencySymbol,
		value.DefaultMoneyThousandsSep, value.DefaultMoneyDecimalPoint,
		value.TermsAndConditionText, value.AnnouncementText, value.AboutText); err != nil {
		var mysqlError *mysqldriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			return languages.ErrDuplicate
		}
		return fmt.Errorf("insert language: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value languages.Language) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if err := repository.checkOtherDefault(ctx, value.Code, value.IsDefault); err != nil {
		return err
	}
	result, err := repository.db.ExecContext(ctx, updateLanguageSQL,
		value.Name, boolToEnum(value.IsDefault), value.SiteLogo, value.SiteTitle,
		value.DefaultThousandsSeparator, value.DefaultDecimalPoint, value.DefaultCurrencySymbol,
		value.DefaultMoneyThousandsSep, value.DefaultMoneyDecimalPoint,
		value.TermsAndConditionText, value.AnnouncementText, value.AboutText, value.Code)
	if err != nil {
		return fmt.Errorf("update language: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check language update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, value.Code)
}

func (repository *MySQLRepository) Delete(ctx context.Context, code string) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	var defaultCode string
	err := repository.db.QueryRowContext(ctx, getDefaultLanguageExistsSQL, code).Scan(&defaultCode)
	if err == nil {
		return languages.ErrCannotDeleteDefault
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check default language: %w", err)
	}
	result, err := repository.db.ExecContext(ctx, deleteLanguageSQL, code)
	if err != nil {
		return fmt.Errorf("delete language: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check language delete: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, code)
}

func (repository *MySQLRepository) checkExists(ctx context.Context, code string) error {
	var existing string
	err := repository.db.QueryRowContext(ctx, languageExistsSQL, code).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		return languages.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check language: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) checkOtherDefault(ctx context.Context, code string, isDefault bool) error {
	if !isDefault {
		return nil
	}
	var other string
	err := repository.db.QueryRowContext(ctx, otherDefaultLanguageExistsSQL, code).Scan(&other)
	if err == nil {
		return languages.ErrOnlyOneDefault
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check language default: %w", err)
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanLanguage(row rowScanner) (languages.Language, error) {
	var value languages.Language
	var isDefault string
	if err := row.Scan(
		&value.Code, &value.Name, &isDefault, &value.SiteLogo, &value.SiteTitle,
		&value.DefaultThousandsSeparator, &value.DefaultDecimalPoint, &value.DefaultCurrencySymbol,
		&value.DefaultMoneyThousandsSep, &value.DefaultMoneyDecimalPoint,
		&value.TermsAndConditionText, &value.AnnouncementText, &value.AboutText); err != nil {
		return languages.Language{}, err
	}
	value.IsDefault = isDefault == "Y"
	return value, nil
}

func boolToEnum(enabled bool) string {
	if enabled {
		return "Y"
	}
	return "N"
}

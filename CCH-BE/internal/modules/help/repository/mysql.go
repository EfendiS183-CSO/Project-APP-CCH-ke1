package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cch.local/backend/internal/modules/help"
)

const articleProjection = "h.Help_ID, h.Language, h.Topic, h.Category, c.Category_Description, h.\x60Order\x60, h.Display_in_Page, h.Updated_By, h.Last_Updated"
const articleFrom = `FROM cch_help h LEFT JOIN cch_help_categories c ON c.Category_ID = h.Category`
const articleFilter = `WHERE (? = '' OR h.Language = ?)
AND (? = 0 OR h.Category = ?)
AND (? = '' OR h.Display_in_Page = ?)`
const listArticlesCountSQL = `SELECT COUNT(*) FROM cch_help h ` + articleFilter
const listArticlesSQL = `SELECT ` + articleProjection + ` ` + articleFrom + ` ` + articleFilter + `
ORDER BY 6 ASC, 1 ASC
LIMIT ? OFFSET ?`
const getArticleSQL = "SELECT h.Help_ID, h.Language, h.Topic, h.Description, h.Category, c.Category_Description, h.\x60Order\x60, h.Display_in_Page, h.Updated_By, h.Last_Updated " + articleFrom + `
WHERE h.Help_ID = ?
LIMIT 1`
const listLanguagesSQL = `SELECT Language_Code, Language_Name FROM cch_languages ORDER BY Language_Code ASC`
const listCategoriesSQL = `SELECT Category_ID, Language, Category_Description FROM cch_help_categories ORDER BY Language ASC, Category_ID ASC`
const lockArticleSQL = `SELECT Help_ID FROM cch_help WHERE Help_ID = ? FOR UPDATE`
const lockLanguageSQL = `SELECT Language_Code FROM cch_languages WHERE Language_Code = ? FOR UPDATE`
const lockCategorySQL = `SELECT Category_ID FROM cch_help_categories WHERE Category_ID = ? FOR UPDATE`
const insertArticleSQL = "INSERT INTO cch_help (Language, Topic, Description, Category, \x60Order\x60, Display_in_Page, Updated_By, Last_Updated) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)"
const updateArticleSQL = "UPDATE cch_help SET Language = ?, Topic = ?, Description = ?, Category = ?, \x60Order\x60 = ?, Display_in_Page = ?, Updated_By = ?, Last_Updated = CURRENT_TIMESTAMP WHERE Help_ID = ?"
const deleteArticleSQL = `DELETE FROM cch_help WHERE Help_ID = ?`

var ErrRepositoryNotConfigured = errors.New("help article repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ help.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context, query help.ListQuery) (help.Page, error) {
	if repository == nil || repository.db == nil {
		return help.Page{}, ErrRepositoryNotConfigured
	}
	filterArgs := []any{query.Language, query.Language, query.CategoryID, query.CategoryID, query.DisplayInPage, query.DisplayInPage}
	var total int64
	if err := repository.db.QueryRowContext(ctx, listArticlesCountSQL, filterArgs...).Scan(&total); err != nil {
		return help.Page{}, fmt.Errorf("count help articles: %w", err)
	}
	args := append(filterArgs, query.Limit, query.Offset)
	rows, err := repository.db.QueryContext(ctx, listArticlesSQL, args...)
	if err != nil {
		return help.Page{}, fmt.Errorf("list help articles: %w", err)
	}
	defer rows.Close()
	values := make([]help.ArticleSummary, 0)
	for rows.Next() {
		value, err := scanSummary(rows)
		if err != nil {
			return help.Page{}, fmt.Errorf("scan help article: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return help.Page{}, fmt.Errorf("read help articles: %w", err)
	}
	return help.Page{Data: values, Total: total, Limit: query.Limit, Offset: query.Offset}, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, id int64) (help.Article, error) {
	if repository == nil || repository.db == nil {
		return help.Article{}, ErrRepositoryNotConfigured
	}
	article, err := scanArticle(repository.db.QueryRowContext(ctx, getArticleSQL, id))
	if err != nil {
		return help.Article{}, fmt.Errorf("get help article: %w", err)
	}
	return article, nil
}

func (repository *MySQLRepository) LookupOptions(ctx context.Context) (help.LookupOptions, error) {
	if repository == nil || repository.db == nil {
		return help.LookupOptions{}, ErrRepositoryNotConfigured
	}
	options := help.LookupOptions{
		Languages:  make([]help.LanguageOption, 0),
		Categories: make([]help.CategoryOption, 0),
	}
	languageRows, err := repository.db.QueryContext(ctx, listLanguagesSQL)
	if err != nil {
		return help.LookupOptions{}, fmt.Errorf("list help article languages: %w", err)
	}
	for languageRows.Next() {
		var option help.LanguageOption
		if err := languageRows.Scan(&option.Code, &option.Name); err != nil {
			languageRows.Close()
			return help.LookupOptions{}, fmt.Errorf("scan help article language: %w", err)
		}
		options.Languages = append(options.Languages, option)
	}
	if err := languageRows.Err(); err != nil {
		languageRows.Close()
		return help.LookupOptions{}, fmt.Errorf("read help article languages: %w", err)
	}
	if err := languageRows.Close(); err != nil {
		return help.LookupOptions{}, fmt.Errorf("close help article languages: %w", err)
	}
	categoryRows, err := repository.db.QueryContext(ctx, listCategoriesSQL)
	if err != nil {
		return help.LookupOptions{}, fmt.Errorf("list help article categories: %w", err)
	}
	defer categoryRows.Close()
	for categoryRows.Next() {
		var option help.CategoryOption
		if err := categoryRows.Scan(&option.ID, &option.Language, &option.Description); err != nil {
			return help.LookupOptions{}, fmt.Errorf("scan help article category: %w", err)
		}
		options.Categories = append(options.Categories, option)
	}
	if err := categoryRows.Err(); err != nil {
		return help.LookupOptions{}, fmt.Errorf("read help article categories: %w", err)
	}
	return options, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, article help.Article, actor string) (help.Article, error) {
	if repository == nil || repository.db == nil {
		return help.Article{}, ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return help.Article{}, fmt.Errorf("begin help article insert: %w", err)
	}
	defer transaction.Rollback()
	if err := lockReferences(ctx, transaction, article); err != nil {
		return help.Article{}, err
	}
	result, err := transaction.ExecContext(ctx, insertArticleSQL, article.Language, article.Topic, article.Description, article.CategoryID, article.Order, article.DisplayInPage, actor)
	if err != nil {
		return help.Article{}, fmt.Errorf("insert help article: %w", err)
	}
	if article.ID, err = result.LastInsertId(); err != nil {
		return help.Article{}, fmt.Errorf("read help article ID: %w", err)
	}
	article, err = scanArticle(transaction.QueryRowContext(ctx, getArticleSQL, article.ID))
	if err != nil {
		return help.Article{}, fmt.Errorf("read inserted help article: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return help.Article{}, fmt.Errorf("commit help article insert: %w", err)
	}
	return article, nil
}

func (repository *MySQLRepository) Update(ctx context.Context, article help.Article, actor string) (help.Article, error) {
	if repository == nil || repository.db == nil {
		return help.Article{}, ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return help.Article{}, fmt.Errorf("begin help article update: %w", err)
	}
	defer transaction.Rollback()
	var existingID int64
	if err := transaction.QueryRowContext(ctx, lockArticleSQL, article.ID).Scan(&existingID); err != nil {
		return help.Article{}, fmt.Errorf("lock help article: %w", err)
	}
	if err := lockReferences(ctx, transaction, article); err != nil {
		return help.Article{}, err
	}
	if _, err := transaction.ExecContext(ctx, updateArticleSQL, article.Language, article.Topic, article.Description, article.CategoryID, article.Order, article.DisplayInPage, actor, article.ID); err != nil {
		return help.Article{}, fmt.Errorf("update help article: %w", err)
	}
	article, err = scanArticle(transaction.QueryRowContext(ctx, getArticleSQL, article.ID))
	if err != nil {
		return help.Article{}, fmt.Errorf("read updated help article: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return help.Article{}, fmt.Errorf("commit help article update: %w", err)
	}
	return article, nil
}

func (repository *MySQLRepository) Delete(ctx context.Context, id int64) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin help article delete: %w", err)
	}
	defer transaction.Rollback()
	var existingID int64
	if err := transaction.QueryRowContext(ctx, lockArticleSQL, id).Scan(&existingID); err != nil {
		return fmt.Errorf("lock help article: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, deleteArticleSQL, id); err != nil {
		return fmt.Errorf("delete help article: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit help article delete: %w", err)
	}
	return nil
}

func lockReferences(ctx context.Context, transaction *sql.Tx, article help.Article) error {
	var language string
	err := transaction.QueryRowContext(ctx, lockLanguageSQL, article.Language).Scan(&language)
	if errors.Is(err, sql.ErrNoRows) {
		return help.ErrInvalidLanguage
	}
	if err != nil {
		return fmt.Errorf("validate help article language: %w", err)
	}
	var categoryID int64
	err = transaction.QueryRowContext(ctx, lockCategorySQL, article.CategoryID).Scan(&categoryID)
	if errors.Is(err, sql.ErrNoRows) {
		return help.ErrInvalidCategory
	}
	if err != nil {
		return fmt.Errorf("validate help article category: %w", err)
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func scanSummary(row rowScanner) (help.ArticleSummary, error) {
	var value help.ArticleSummary
	var categoryDescription sql.NullString
	var updatedBy sql.NullString
	var lastUpdated sql.NullTime
	err := row.Scan(&value.ID, &value.Language, &value.Topic, &value.CategoryID, &categoryDescription, &value.Order, &value.DisplayInPage, &updatedBy, &lastUpdated)
	if err != nil {
		return help.ArticleSummary{}, err
	}
	if categoryDescription.Valid {
		value.CategoryDescription = &categoryDescription.String
	}
	if updatedBy.Valid {
		value.UpdatedBy = &updatedBy.String
	}
	if lastUpdated.Valid {
		value.LastUpdated = &lastUpdated.Time
	}
	return value, nil
}

func scanArticle(row rowScanner) (help.Article, error) {
	var value help.Article
	var categoryDescription sql.NullString
	var updatedBy sql.NullString
	var lastUpdated sql.NullTime
	err := row.Scan(&value.ID, &value.Language, &value.Topic, &value.Description, &value.CategoryID, &categoryDescription, &value.Order, &value.DisplayInPage, &updatedBy, &lastUpdated)
	if err != nil {
		return help.Article{}, err
	}
	if categoryDescription.Valid {
		value.CategoryDescription = &categoryDescription.String
	}
	if updatedBy.Valid {
		value.UpdatedBy = &updatedBy.String
	}
	if lastUpdated.Valid {
		value.LastUpdated = &lastUpdated.Time
	}
	return value, nil
}

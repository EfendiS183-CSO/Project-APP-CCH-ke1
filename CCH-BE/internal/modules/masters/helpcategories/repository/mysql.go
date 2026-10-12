package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cch.local/backend/internal/modules/masters/helpcategories"
)

const listCategoriesSQL = `SELECT Category_ID, Language, Category_Description
FROM cch_help_categories
ORDER BY Category_ID ASC`
const getCategorySQL = `SELECT Category_ID, Language, Category_Description
FROM cch_help_categories
WHERE Category_ID = ?
LIMIT 1`
const listLanguagesSQL = `SELECT Language_Code, Language_Name
FROM cch_languages
ORDER BY Language_Code ASC`
const languageExistsSQL = `SELECT Language_Code FROM cch_languages WHERE Language_Code = ? LIMIT 1`
const insertCategorySQL = `INSERT INTO cch_help_categories (Language, Category_Description) VALUES (?, ?)`
const updateCategorySQL = `UPDATE cch_help_categories SET Language = ?, Category_Description = ? WHERE Category_ID = ?`
const lockCategorySQL = `SELECT Category_ID FROM cch_help_categories WHERE Category_ID = ? FOR UPDATE`
const categoryInUseSQL = `SELECT Help_ID FROM cch_help WHERE Category = ? LIMIT 1`
const deleteCategorySQL = `DELETE FROM cch_help_categories WHERE Category_ID = ?`
const categoryExistsSQL = `SELECT Category_ID FROM cch_help_categories WHERE Category_ID = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("help category repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ helpcategories.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context) ([]helpcategories.Category, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listCategoriesSQL)
	if err != nil {
		return nil, fmt.Errorf("list help categories: %w", err)
	}
	defer rows.Close()
	values := make([]helpcategories.Category, 0)
	for rows.Next() {
		var value helpcategories.Category
		if err := rows.Scan(&value.ID, &value.Language, &value.Description); err != nil {
			return nil, fmt.Errorf("scan help category: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read help categories: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, id int64) (helpcategories.Category, error) {
	if repository == nil || repository.db == nil {
		return helpcategories.Category{}, ErrRepositoryNotConfigured
	}
	var value helpcategories.Category
	err := repository.db.QueryRowContext(ctx, getCategorySQL, id).Scan(&value.ID, &value.Language, &value.Description)
	if err != nil {
		return helpcategories.Category{}, fmt.Errorf("get help category: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Languages(ctx context.Context) ([]helpcategories.Language, error) {
	if repository == nil || repository.db == nil {
		return nil, ErrRepositoryNotConfigured
	}
	rows, err := repository.db.QueryContext(ctx, listLanguagesSQL)
	if err != nil {
		return nil, fmt.Errorf("list help category languages: %w", err)
	}
	defer rows.Close()
	values := make([]helpcategories.Language, 0)
	for rows.Next() {
		var value helpcategories.Language
		if err := rows.Scan(&value.Code, &value.Name); err != nil {
			return nil, fmt.Errorf("scan help category language: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read help category languages: %w", err)
	}
	return values, nil
}

func (repository *MySQLRepository) Insert(ctx context.Context, value helpcategories.Category) (helpcategories.Category, error) {
	if repository == nil || repository.db == nil {
		return helpcategories.Category{}, ErrRepositoryNotConfigured
	}
	if err := repository.requireLanguage(ctx, value.Language); err != nil {
		return helpcategories.Category{}, err
	}
	result, err := repository.db.ExecContext(ctx, insertCategorySQL, value.Language, value.Description)
	if err != nil {
		return helpcategories.Category{}, fmt.Errorf("insert help category: %w", err)
	}
	if value.ID, err = result.LastInsertId(); err != nil {
		return helpcategories.Category{}, fmt.Errorf("read help category ID: %w", err)
	}
	return value, nil
}

func (repository *MySQLRepository) Update(ctx context.Context, value helpcategories.Category) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	if err := repository.requireLanguage(ctx, value.Language); err != nil {
		return err
	}
	result, err := repository.db.ExecContext(ctx, updateCategorySQL, value.Language, value.Description, value.ID)
	if err != nil {
		return fmt.Errorf("update help category: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check help category update: %w", err)
	}
	if rowsAffected > 0 {
		return nil
	}
	return repository.checkExists(ctx, value.ID)
}

func (repository *MySQLRepository) Delete(ctx context.Context, id int64) error {
	if repository == nil || repository.db == nil {
		return ErrRepositoryNotConfigured
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin help category delete: %w", err)
	}
	defer transaction.Rollback()
	var categoryID int64
	if err := transaction.QueryRowContext(ctx, lockCategorySQL, id).Scan(&categoryID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return helpcategories.ErrNotFound
		}
		return fmt.Errorf("lock help category: %w", err)
	}
	var helpID int64
	if err := transaction.QueryRowContext(ctx, categoryInUseSQL, id).Scan(&helpID); err == nil {
		return helpcategories.ErrInUse
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check help category references: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, deleteCategorySQL, id); err != nil {
		return fmt.Errorf("delete help category: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit help category delete: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) requireLanguage(ctx context.Context, code string) error {
	var existingCode string
	err := repository.db.QueryRowContext(ctx, languageExistsSQL, code).Scan(&existingCode)
	if errors.Is(err, sql.ErrNoRows) {
		return helpcategories.ErrInvalidLanguage
	}
	if err != nil {
		return fmt.Errorf("validate help category language: %w", err)
	}
	return nil
}

func (repository *MySQLRepository) checkExists(ctx context.Context, id int64) error {
	var existingID int64
	err := repository.db.QueryRowContext(ctx, categoryExistsSQL, id).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return helpcategories.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check help category: %w", err)
	}
	return nil
}

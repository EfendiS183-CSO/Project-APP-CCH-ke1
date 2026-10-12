package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cch.local/backend/internal/modules/userlevels"
)

const countUserLevelsSQL = `SELECT COUNT(*) FROM cch_userlevels`
const listUserLevelsSQL = `SELECT User_Level_ID, User_Level_Name FROM cch_userlevels
ORDER BY User_Level_ID ASC LIMIT ? OFFSET ?`
const getUserLevelSQL = `SELECT User_Level_ID, User_Level_Name FROM cch_userlevels WHERE User_Level_ID = ? LIMIT 1`

var ErrRepositoryNotConfigured = errors.New("user level repository is not configured")

type MySQLRepository struct{ db *sql.DB }

var _ userlevels.Repository = (*MySQLRepository)(nil)

func NewMySQLRepository(database *sql.DB) *MySQLRepository { return &MySQLRepository{db: database} }

func (repository *MySQLRepository) List(ctx context.Context, query userlevels.ListQuery) (userlevels.Page, error) {
	if repository == nil || repository.db == nil {
		return userlevels.Page{}, ErrRepositoryNotConfigured
	}
	query, err := query.Normalize()
	if err != nil {
		return userlevels.Page{}, err
	}
	var total int64
	if err := repository.db.QueryRowContext(ctx, countUserLevelsSQL).Scan(&total); err != nil {
		return userlevels.Page{}, fmt.Errorf("count user levels: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, listUserLevelsSQL, query.Limit, query.Offset)
	if err != nil {
		return userlevels.Page{}, fmt.Errorf("list user levels: %w", err)
	}
	defer rows.Close()
	values := make([]userlevels.UserLevel, 0, query.Limit)
	for rows.Next() {
		var value userlevels.UserLevel
		if err := rows.Scan(&value.ID, &value.Name); err != nil {
			return userlevels.Page{}, fmt.Errorf("scan user level: %w", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return userlevels.Page{}, fmt.Errorf("read user levels: %w", err)
	}
	return userlevels.Page{Data: values, Total: total, Limit: query.Limit, Offset: query.Offset}, nil
}

func (repository *MySQLRepository) Get(ctx context.Context, id int64) (userlevels.UserLevel, error) {
	if repository == nil || repository.db == nil {
		return userlevels.UserLevel{}, ErrRepositoryNotConfigured
	}
	var value userlevels.UserLevel
	if err := repository.db.QueryRowContext(ctx, getUserLevelSQL, id).Scan(&value.ID, &value.Name); err != nil {
		return userlevels.UserLevel{}, fmt.Errorf("get user level: %w", err)
	}
	return value, nil
}

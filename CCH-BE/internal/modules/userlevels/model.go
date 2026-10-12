package userlevels

import (
	"context"
	"errors"
	"math"
)

const TableVariable = "cch_userlevels"

const (
	DefaultPageSize = 25
	MaximumPageSize = 100
)

var (
	ErrInvalidInput    = errors.New("user level query is invalid")
	ErrNotFound        = errors.New("user level not found")
	ErrServiceNotReady = errors.New("user level catalog is not configured")
)

type UserLevel struct {
	ID   int64  `json:"user_level_id"`
	Name string `json:"user_level_name"`
}

type ListQuery struct {
	Limit  int
	Offset int
}

func (query ListQuery) Normalize() (ListQuery, error) {
	if query.Limit == 0 {
		query.Limit = DefaultPageSize
	}
	if query.Limit < 1 || query.Limit > MaximumPageSize || query.Offset < 0 {
		return ListQuery{}, ErrInvalidInput
	}
	return query, nil
}

type Page struct {
	Data   []UserLevel `json:"data"`
	Total  int64       `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type Repository interface {
	List(context.Context, ListQuery) (Page, error)
	Get(context.Context, int64) (UserLevel, error)
}

func validateID(id int64) error {
	if id < math.MinInt32 || id > math.MaxInt32 {
		return ErrInvalidInput
	}
	return nil
}

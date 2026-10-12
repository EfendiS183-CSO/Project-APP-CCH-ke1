package breadcrumbs

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const TableVariable = "cch_breadcrumblinks"

const (
	defaultPageSize = 25
	maximumPageSize = 100
	maximumTitleLen = 100
	maximumURLLen   = 100
)

var (
	ErrInvalidInput    = errors.New("invalid breadcrumb input")
	ErrNotFound        = errors.New("breadcrumb link not found")
	ErrDuplicateTitle  = errors.New("breadcrumb page title already exists")
	ErrInvalidParent   = errors.New("breadcrumb parent page title does not exist")
	ErrInvalidRoot     = errors.New("breadcrumb root page title does not exist")
	ErrSameRoot        = errors.New("breadcrumb move source and destination are identical")
	ErrNoData          = errors.New("breadcrumb tree is empty")
	ErrServiceNotReady = errors.New("breadcrumb service is not configured")
)

type Breadcrumb struct {
	PageTitle string `json:"page_title"`
	PageURL   string `json:"page_url"`
	Left      int    `json:"left"`
	Right     int    `json:"right"`
}

type Page struct {
	Data   []Breadcrumb `json:"data"`
	Total  int64        `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

type Ancestor struct {
	PageTitle string `json:"page_title"`
	PageURL   string `json:"page_url"`
}

type ListQuery struct {
	Limit  int
	Offset int
}

func (query ListQuery) Normalize() (ListQuery, error) {
	if query.Limit == 0 {
		query.Limit = defaultPageSize
	}
	if query.Limit < 1 || query.Limit > maximumPageSize || query.Offset < 0 {
		return ListQuery{}, ErrInvalidInput
	}
	return query, nil
}

func ValidateTitle(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maximumTitleLen {
		return "", ErrInvalidInput
	}
	return value, nil
}

func ValidateURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maximumURLLen {
		return "", ErrInvalidInput
	}
	return value, nil
}

type Repository interface {
	Transaction(ctx context.Context, fn func(context.Context, Querier) error) error
	List(context.Context, ListQuery) (Page, error)
	Get(context.Context, string) (Breadcrumb, error)
	Ancestors(context.Context, string) ([]Ancestor, error)
}

type Querier interface {
	Count(context.Context) (int64, error)
	Exists(context.Context, string) (bool, error)
	Get(context.Context, string) (Breadcrumb, error)
	Add(context.Context, string, string, string, int, int) error
	Move(context.Context, string, string) error
	Delete(context.Context, string) error
}

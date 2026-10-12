package regionals

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	TableVariable       = "cch_regional"
	OfficeTableVariable = "cch_kantor"
	DefaultPageSize     = 25
	MaximumPageSize     = 100
)

var (
	ErrInvalidInput    = errors.New("regional office input is invalid")
	ErrNotFound        = errors.New("regional office not found")
	ErrInvalidRegional = errors.New("regional assignment must reference an existing office")
	ErrServiceNotReady = errors.New("regional office service is not configured")
)

type Office struct {
	Code         string `json:"office_code"`
	Name         string `json:"office_name"`
	Abbreviation string `json:"abbreviation"`
	Type         string `json:"type"`
	RegionalCode string `json:"regional_code"`
	Active       bool   `json:"active"`
}

type OfficeOption struct {
	Code string `json:"office_code"`
	Name string `json:"office_name"`
	Type string `json:"type"`
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
	Data   []Office `json:"data"`
	Total  int64    `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

type UpdateCommand struct {
	Code         string
	Name         string
	Abbreviation string
	Type         string
	RegionalCode string
	Active       bool
}

type Repository interface {
	List(context.Context, ListQuery) (Page, error)
	Get(context.Context, string) (Office, error)
	LookupOptions(context.Context) ([]OfficeOption, error)
	Update(context.Context, Office) error
}

func normalizeCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len(value) == 0 || len(value) > 7 || !isASCII(value) {
		return "", ErrInvalidInput
	}
	return value, nil
}

func normalizeRegionalCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len(value) == 0 || len(value) > 5 || !isASCII(value) {
		return "", ErrInvalidInput
	}
	return value, nil
}

func normalizeText(value string, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum || !isLatin1(value) {
		return "", ErrInvalidInput
	}
	return value, nil
}

func isASCII(value string) bool {
	for _, character := range value {
		if character > 0x7f {
			return false
		}
	}
	return true
}

func isLatin1(value string) bool {
	for _, character := range value {
		if character > 0xff {
			return false
		}
	}
	return true
}

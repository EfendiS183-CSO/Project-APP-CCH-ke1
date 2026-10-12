package timezones

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const TableVariable = "cch_timezone"

var (
	ErrInvalidInput        = errors.New("invalid timezone input")
	ErrNotFound            = errors.New("timezone not found")
	ErrDuplicate           = errors.New("timezone name already exists")
	ErrOnlyOneDefault      = errors.New("only one timezone can be the default")
	ErrCannotDeleteDefault = errors.New("the default timezone cannot be deleted")
	ErrServiceNotReady     = errors.New("timezone master is not configured")
)

// Timezone mirrors one legacy cch_timezone row. ID is server-owned; Timezone is
// unique by the legacy UNIQUE KEY Index_Timezone; Default maps the enum Y/N
// with exactly one active default.
type Timezone struct {
	ID        int64  `json:"id"`
	Timezone  string `json:"timezone"`
	IsDefault bool   `json:"is_default"`
}

type CreateCommand struct {
	Timezone  string
	IsDefault bool
}

type UpdateCommand struct {
	ID        int64
	Timezone  string
	IsDefault bool
}

type Repository interface {
	List(context.Context) ([]Timezone, error)
	Get(context.Context, int64) (Timezone, error)
	Insert(context.Context, Timezone) error
	Update(context.Context, Timezone) error
	Delete(context.Context, int64) error
}

func normalizeValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 50 {
		return "", ErrInvalidInput
	}
	return value, nil
}

func validateID(id int64) error {
	if id < 1 {
		return ErrInvalidInput
	}
	return nil
}

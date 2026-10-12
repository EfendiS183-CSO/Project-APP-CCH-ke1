package followupstatuses

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const TableVariable = "cchfust"

var (
	ErrInvalidInput    = errors.New("invalid follow-up status input")
	ErrNotFound        = errors.New("follow-up status not found")
	ErrAlreadyExists   = errors.New("follow-up status code already exists")
	ErrInUse           = errors.New("follow-up status is referenced by follow-up records")
	ErrServiceNotReady = errors.New("follow-up status master is not configured")
)

type Status struct {
	Code        string  `json:"code"`
	Description *string `json:"description"`
}

type CreateCommand struct {
	Code        string
	Description *string
}

type UpdateCommand struct {
	Code        string
	Description *string
}

type Repository interface {
	List(context.Context) ([]Status, error)
	LookupOptions(context.Context) ([]Status, error)
	Get(context.Context, string) (Status, error)
	Insert(context.Context, Status) error
	Update(context.Context, Status) error
	Delete(context.Context, string) error
}

func normalizeCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if len(code) != 3 || !utf8.ValidString(code) {
		return "", ErrInvalidInput
	}
	for _, character := range code {
		if character > 0x7f {
			return "", ErrInvalidInput
		}
	}
	return code, nil
}

func normalizeDescription(description *string) (*string, error) {
	if description == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*description)
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 20 {
		return nil, ErrInvalidInput
	}
	for _, character := range value {
		if character > 0xff {
			return nil, ErrInvalidInput
		}
	}
	return &value, nil
}

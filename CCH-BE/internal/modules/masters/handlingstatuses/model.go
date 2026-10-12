package handlingstatuses

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const TableVariable = "cch_jenis_penanganan"

var (
	ErrInvalidInput    = errors.New("invalid handling status input")
	ErrNotFound        = errors.New("handling status not found")
	ErrAlreadyExists   = errors.New("handling status code already exists")
	ErrServiceNotReady = errors.New("handling status master is not configured")
)

type Status struct {
	Code        string `json:"status_code"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
}

type CreateCommand struct {
	Code        string
	Description string
	Active      bool
}

type UpdateCommand struct {
	Code        string
	Description string
	Active      bool
}

type Repository interface {
	List(context.Context) ([]Status, error)
	Get(context.Context, string) (Status, error)
	Insert(context.Context, Status) error
	Update(context.Context, Status) error
	Deactivate(context.Context, string) error
}

func normalizeCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if !utf8.ValidString(code) || utf8.RuneCountInString(code) != 3 {
		return "", ErrInvalidInput
	}
	for _, character := range code {
		if character > 0x7f {
			return "", ErrInvalidInput
		}
	}
	return code, nil
}

func normalizeDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if description == "" || !utf8.ValidString(description) || utf8.RuneCountInString(description) > 50 {
		return "", ErrInvalidInput
	}
	for _, character := range description {
		if character > 0xff {
			return "", ErrInvalidInput
		}
	}
	return description, nil
}

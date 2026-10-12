package documenttypes

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const TableVariable = "cch_jenis_dokumen"

var (
	ErrInvalidInput    = errors.New("invalid document type input")
	ErrNotFound        = errors.New("document type not found")
	ErrAlreadyExists   = errors.New("document type code already exists")
	ErrServiceNotReady = errors.New("document type master is not configured")
)

type DocumentType struct {
	Code        string     `json:"code"`
	Description string     `json:"description"`
	Active      bool       `json:"active"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	CreatedBy   string     `json:"created_by,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	UpdatedBy   string     `json:"updated_by,omitempty"`
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
	List(context.Context) ([]DocumentType, error)
	Get(context.Context, string) (DocumentType, error)
	Insert(context.Context, DocumentType) error
	Update(context.Context, DocumentType) error
	Deactivate(context.Context, string, string, time.Time) error
}

func normalizeCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if !utf8.ValidString(code) || utf8.RuneCountInString(code) != 3 {
		return "", ErrInvalidInput
	}
	return code, nil
}

func normalizeDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if description == "" || !utf8.ValidString(description) || utf8.RuneCountInString(description) > 100 {
		return "", ErrInvalidInput
	}
	return description, nil
}

func normalizeActor(actor string) (string, error) {
	actor = strings.TrimSpace(actor)
	if actor == "" || !utf8.ValidString(actor) || utf8.RuneCountInString(actor) > 20 {
		return "", ErrInvalidInput
	}
	return actor, nil
}

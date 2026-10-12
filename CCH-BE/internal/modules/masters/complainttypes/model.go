package complainttypes

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const TableVariable = "cch_jenis_pengaduan"

var (
	ErrInvalidInput    = errors.New("invalid complaint type input")
	ErrNotFound        = errors.New("complaint type not found")
	ErrAlreadyExists   = errors.New("complaint type code already exists")
	ErrServiceNotReady = errors.New("complaint type service is not configured")
)

type ComplaintType struct {
	Code        int64      `json:"code"`
	Description *string    `json:"description"`
	Active      bool       `json:"active"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

type CreateCommand struct {
	Code        int64
	Description *string
	Active      bool
}

type UpdateCommand struct {
	Code        int64
	Description *string
	Active      bool
}

type Repository interface {
	List(context.Context) ([]ComplaintType, error)
	Get(context.Context, int64) (ComplaintType, error)
	Insert(context.Context, ComplaintType) error
	Update(context.Context, ComplaintType) error
	Deactivate(context.Context, int64, time.Time) error
}

func normalizeCode(code int64) error {
	if code < -128 || code > 127 {
		return ErrInvalidInput
	}
	return nil
}

func normalizeDescription(description *string) (*string, error) {
	if description == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*description)
	if utf8.RuneCountInString(value) > 50 {
		return nil, ErrInvalidInput
	}
	if value == "" {
		return nil, nil
	}
	return &value, nil
}

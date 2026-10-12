package sources

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const TableVariable = "cch_sumber_pengaduan"

var (
	ErrInvalidInput    = errors.New("invalid complaint source input")
	ErrNotFound        = errors.New("complaint source not found")
	ErrServiceNotReady = errors.New("complaint source service is not configured")
)

type Source struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Active    bool       `json:"active"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type CreateCommand struct {
	Name   string
	Active bool
}

type UpdateCommand struct {
	ID     int64
	Name   string
	Active bool
}

type Repository interface {
	List(context.Context) ([]Source, error)
	Get(context.Context, int64) (Source, error)
	Insert(context.Context, Source) (Source, error)
	Update(context.Context, Source) error
	Deactivate(context.Context, int64, time.Time) error
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return "", ErrInvalidInput
	}
	return name, nil
}

func validateID(id int64) error {
	if id < 1 {
		return ErrInvalidInput
	}
	return nil
}

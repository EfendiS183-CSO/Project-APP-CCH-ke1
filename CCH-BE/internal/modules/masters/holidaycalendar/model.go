package holidaycalendar

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const TableVariable = "cch_kalender_libur"

// DateLayout is the transport representation of the legacy DATE primary key.
const DateLayout = "2006-01-02"

var (
	ErrInvalidInput    = errors.New("invalid holiday calendar input")
	ErrNotFound        = errors.New("holiday calendar entry not found")
	ErrAlreadyExists   = errors.New("holiday calendar date already exists")
	ErrServiceNotReady = errors.New("holiday calendar master is not configured")
)

// Holiday mirrors one legacy cch_kalender_libur row. The date is the business
// key and is immutable after creation, matching the legacy generated pages.
type Holiday struct {
	Date        string     `json:"date"`
	Description string     `json:"description"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	CreatedBy   string     `json:"created_by,omitempty"`
}

type CreateCommand struct {
	Date        string
	Description string
}

type UpdateCommand struct {
	Date        string
	Description string
}

type Repository interface {
	List(context.Context) ([]Holiday, error)
	Get(context.Context, string) (Holiday, error)
	Insert(context.Context, Holiday) error
	Update(context.Context, Holiday) error
	Delete(context.Context, string) error
}

// NormalizeDate validates the DATE key and normalizes it to the stored layout.
func NormalizeDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) {
		return "", ErrInvalidInput
	}
	parsed, err := time.Parse(DateLayout, value)
	if err != nil {
		return "", ErrInvalidInput
	}
	return parsed.Format(DateLayout), nil
}

func normalizeDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if description == "" || !utf8.ValidString(description) || utf8.RuneCountInString(description) > 50 {
		return "", ErrInvalidInput
	}
	return description, nil
}

// normalizeActor bounds the legacy VARCHAR(20) "Ditambah_Oleh" value.
func normalizeActor(actor string) (string, error) {
	actor = strings.TrimSpace(actor)
	if actor == "" || !utf8.ValidString(actor) || utf8.RuneCountInString(actor) > 20 {
		return "", ErrInvalidInput
	}
	return actor, nil
}

package shippingservices

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const TableVariable = "cch_jenis_layanan"

var (
	ErrInvalidInput    = errors.New("invalid shipping service input")
	ErrNotFound        = errors.New("shipping service not found")
	ErrAlreadyExists   = errors.New("shipping service code already exists")
	ErrServiceNotReady = errors.New("shipping service master is not configured")
)

type ShippingService struct {
	Code        string     `json:"code"`
	Description string     `json:"description"`
	TrackingURL string     `json:"tracking_url"`
	Active      bool       `json:"active"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

type CreateCommand struct {
	Code        string
	Description string
	TrackingURL string
	Active      bool
}

type UpdateCommand struct {
	Code        string
	Description string
	TrackingURL string
	Active      bool
}

type Repository interface {
	List(context.Context) ([]ShippingService, error)
	Get(context.Context, string) (ShippingService, error)
	Insert(context.Context, ShippingService) error
	Update(context.Context, ShippingService) error
	Deactivate(context.Context, string, time.Time) error
}

func normalizeCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" || utf8.RuneCountInString(code) > 3 {
		return "", ErrInvalidInput
	}
	return code, nil
}

func normalizeDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return "0", nil
	}
	if utf8.RuneCountInString(description) > 25 {
		return "", ErrInvalidInput
	}
	return description, nil
}

func normalizeTrackingURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > 255 {
		return "", ErrInvalidInput
	}
	return value, nil
}

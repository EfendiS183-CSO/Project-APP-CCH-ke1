package offices

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const TableVariable = "cch_kantor"

var (
	ErrInvalidInput    = errors.New("invalid office input")
	ErrNotFound        = errors.New("office not found")
	ErrAlreadyExists   = errors.New("office code already exists")
	ErrInvalidType     = errors.New("office type is not an existing legacy type")
	ErrInvalidRegional = errors.New("regional office must be active and have type KP or Divre")
	ErrServiceNotReady = errors.New("office master is not configured")
)

type Office struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Abbreviation  string `json:"abbreviation"`
	Type          string `json:"type"`
	RegionalCode  string `json:"regional_code"`
	Active        bool   `json:"active"`
	AsDestination bool   `json:"as_destination"`
	AsCopy        bool   `json:"as_copy"`
}

type OfficeOption struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type LookupOptions struct {
	Types           []string       `json:"types"`
	RegionalOffices []OfficeOption `json:"regional_offices"`
}

type CreateCommand struct {
	Code          string
	Name          string
	Abbreviation  string
	Type          string
	RegionalCode  string
	Active        bool
	AsDestination bool
	AsCopy        bool
}

type UpdateCommand struct {
	Code          string
	Name          string
	Abbreviation  string
	Type          string
	RegionalCode  string
	Active        bool
	AsDestination bool
	AsCopy        bool
}

type Repository interface {
	List(context.Context) ([]Office, error)
	Get(context.Context, string) (Office, error)
	LookupOptions(context.Context) (LookupOptions, error)
	Insert(context.Context, Office) error
	Update(context.Context, Office) error
	Deactivate(context.Context, string) error
}

func NormalizeCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len(value) == 0 || len(value) > 7 || !isASCII(value) {
		return "", ErrInvalidInput
	}
	return value, nil
}

func NormalizeRegionalCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len(value) == 0 || len(value) > 5 || !isASCII(value) {
		return "", ErrInvalidInput
	}
	return value, nil
}

func NormalizeText(value string, maximum int) (string, error) {
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

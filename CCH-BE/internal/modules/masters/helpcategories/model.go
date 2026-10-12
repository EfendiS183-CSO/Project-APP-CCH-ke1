package helpcategories

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const TableVariable = "cch_help_categories"

var (
	ErrInvalidInput    = errors.New("invalid help category input")
	ErrInvalidLanguage = errors.New("help category language does not exist")
	ErrNotFound        = errors.New("help category not found")
	ErrInUse           = errors.New("help category is referenced by help entries")
	ErrServiceNotReady = errors.New("help category service is not configured")
)

type Category struct {
	ID          int64  `json:"id"`
	Language    string `json:"language"`
	Description string `json:"description"`
}

type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type CreateCommand struct {
	Language    string
	Description string
}

type UpdateCommand struct {
	ID          int64
	Language    string
	Description string
}

type Repository interface {
	List(context.Context) ([]Category, error)
	Get(context.Context, int64) (Category, error)
	Languages(context.Context) ([]Language, error)
	Insert(context.Context, Category) (Category, error)
	Update(context.Context, Category) error
	Delete(context.Context, int64) error
}

func normalizeCategory(language, description string) (string, string, error) {
	language = strings.TrimSpace(language)
	description = strings.TrimSpace(description)
	if !utf8.ValidString(language) || utf8.RuneCountInString(language) != 2 ||
		!utf8.ValidString(description) || description == "" || utf8.RuneCountInString(description) > 100 {
		return "", "", ErrInvalidInput
	}
	return language, description, nil
}

func validateID(id int64) error {
	if id < 1 {
		return ErrInvalidInput
	}
	return nil
}

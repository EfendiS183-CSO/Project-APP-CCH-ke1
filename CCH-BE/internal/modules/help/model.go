package help

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const TableVariable = "cch_help"

const (
	defaultPageSize = 25
	maximumPageSize = 100
	maximumBodySize = 1 << 20
)

var (
	ErrInvalidInput    = errors.New("invalid help article input")
	ErrNotFound        = errors.New("help article not found")
	ErrInvalidLanguage = errors.New("help article language does not exist")
	ErrInvalidCategory = errors.New("help article category does not exist")
	ErrServiceNotReady = errors.New("help article service is not configured")
)

type ArticleSummary struct {
	ID                  int64      `json:"id"`
	Language            string     `json:"language"`
	Topic               string     `json:"topic"`
	CategoryID          int64      `json:"category_id"`
	CategoryDescription *string    `json:"category_description,omitempty"`
	Order               int32      `json:"order"`
	DisplayInPage       string     `json:"display_in_page"`
	UpdatedBy           *string    `json:"updated_by,omitempty"`
	LastUpdated         *time.Time `json:"last_updated,omitempty"`
}

type Article struct {
	ArticleSummary
	Description string `json:"description"`
}

type Page struct {
	Data   []ArticleSummary `json:"data"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type LanguageOption struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type CategoryOption struct {
	ID          int64  `json:"id"`
	Language    string `json:"language"`
	Description string `json:"description"`
}

type LookupOptions struct {
	Languages  []LanguageOption `json:"languages"`
	Categories []CategoryOption `json:"categories"`
}

type ListQuery struct {
	Language      string
	CategoryID    int64
	DisplayInPage string
	Limit         int
	Offset        int
}

type CreateCommand struct {
	Language      string
	Topic         string
	Description   string
	CategoryID    int64
	Order         int64
	DisplayInPage string
}

type UpdateCommand struct {
	ID int64
	CreateCommand
}

type Repository interface {
	List(ctx context.Context, query ListQuery) (Page, error)
	Get(ctx context.Context, id int64) (Article, error)
	LookupOptions(ctx context.Context) (LookupOptions, error)
	Insert(ctx context.Context, article Article, actor string) (Article, error)
	Update(ctx context.Context, article Article, actor string) (Article, error)
	Delete(ctx context.Context, id int64) error
}

func normalizeListQuery(query ListQuery) (ListQuery, error) {
	query.Language = strings.TrimSpace(query.Language)
	query.DisplayInPage = strings.TrimSpace(query.DisplayInPage)
	if query.Limit == 0 {
		query.Limit = defaultPageSize
	}
	if query.Limit < 1 || query.Limit > maximumPageSize || query.Offset < 0 || query.CategoryID < 0 {
		return ListQuery{}, ErrInvalidInput
	}
	if query.Language != "" && (!utf8.ValidString(query.Language) || utf8.RuneCountInString(query.Language) != 2 || !utf8MB3(query.Language)) {
		return ListQuery{}, ErrInvalidInput
	}
	if utf8.RuneCountInString(query.DisplayInPage) > 100 || !utf8MB3(query.DisplayInPage) {
		return ListQuery{}, ErrInvalidInput
	}
	return query, nil
}

func normalizeArticle(command CreateCommand, id int64, actor string) (Article, string, error) {
	language := strings.TrimSpace(command.Language)
	topic := strings.TrimSpace(command.Topic)
	displayInPage := strings.TrimSpace(command.DisplayInPage)
	actor = strings.TrimSpace(actor)
	if id < 0 || !validText(language) || utf8.RuneCountInString(language) != 2 ||
		!validText(topic) || topic == "" || utf8.RuneCountInString(topic) > 255 ||
		!validText(command.Description) || strings.TrimSpace(command.Description) == "" || len(command.Description) > maximumBodySize ||
		command.CategoryID < 1 || command.Order < -2147483648 || command.Order > 2147483647 ||
		!validText(displayInPage) || displayInPage == "" || utf8.RuneCountInString(displayInPage) > 100 ||
		!validText(actor) || utf8.RuneCountInString(actor) > 20 {
		return Article{}, "", ErrInvalidInput
	}
	return Article{
		ArticleSummary: ArticleSummary{
			ID:            id,
			Language:      language,
			Topic:         topic,
			CategoryID:    command.CategoryID,
			Order:         int32(command.Order),
			DisplayInPage: displayInPage,
		},
		Description: command.Description,
	}, actor, nil
}

func validText(value string) bool {
	return utf8.ValidString(value) && utf8MB3(value)
}

func utf8MB3(value string) bool {
	for _, character := range value {
		if character > 0xFFFF {
			return false
		}
	}
	return true
}

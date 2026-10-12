package announcements

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const TableVariable = "cch_announcement"

const (
	defaultPageSize = 25
	maximumPageSize = 100
	maximumBodySize = 1 << 20
)

var (
	ErrInvalidInput         = errors.New("invalid announcement input")
	ErrNotFound             = errors.New("announcement not found")
	ErrInUse                = errors.New("announcement has translated child rows")
	ErrInvalidLanguage      = errors.New("announcement language does not exist")
	ErrDateRange            = errors.New("announcement date start must be before or equal date end")
	ErrDateOverlaps         = errors.New("announcement dates overlap another announcement")
	ErrDuplicateTranslation = errors.New("announcement translation already exists for this language")
	ErrServiceNotReady      = errors.New("announcement service is not configured")
)

type Announcement struct {
	ID             int64      `json:"id"`
	IsActive       bool       `json:"is_active"`
	Topic          string     `json:"topic"`
	Message        string     `json:"message"`
	DateLastUpdate time.Time  `json:"date_last_update"`
	Language       string     `json:"language"`
	AutoPublish    bool       `json:"auto_publish"`
	DateStart      *time.Time `json:"date_start,omitempty"`
	DateEnd        *time.Time `json:"date_end,omitempty"`
	DateCreated    *time.Time `json:"date_created,omitempty"`
	CreatedBy      *string    `json:"created_by,omitempty"`
	TranslatedID   *int64     `json:"translated_id,omitempty"`
}

type Page struct {
	Data   []Announcement `json:"data"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type LanguageOption struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type Options struct {
	Languages []LanguageOption `json:"languages"`
}

type ListQuery struct {
	Language string
	Limit    int
	Offset   int
}

func (query ListQuery) Normalize() (ListQuery, error) {
	if query.Limit == 0 {
		query.Limit = defaultPageSize
	}
	if query.Limit < 1 || query.Limit > maximumPageSize || query.Offset < 0 {
		return ListQuery{}, ErrInvalidInput
	}
	query.Language = strings.TrimSpace(query.Language)
	if query.Language != "" && !isLanguageCode(query.Language) {
		return ListQuery{}, ErrInvalidInput
	}
	return query, nil
}

type CreateCommand struct {
	IsActive     bool
	Topic        string
	Message      string
	Language     string
	AutoPublish  bool
	DateStart    *time.Time
	DateEnd      *time.Time
	TranslatedID *int64
}

type UpdateCommand struct {
	ID           int64
	IsActive     bool
	Topic        string
	Message      string
	Language     string
	AutoPublish  bool
	DateStart    *time.Time
	DateEnd      *time.Time
	TranslatedID *int64
}

func (command CreateCommand) Normalize() (CreateCommand, error) {
	topic, message, language, err := normalizeFields(command.Topic, command.Message, command.Language)
	if err != nil {
		return CreateCommand{}, err
	}
	command.Topic = topic
	command.Message = message
	command.Language = language
	if err := validateDates(command.DateStart, command.DateEnd); err != nil {
		return CreateCommand{}, err
	}
	return command, nil
}

func (command UpdateCommand) Normalize() (UpdateCommand, error) {
	if command.ID <= 0 {
		return UpdateCommand{}, ErrInvalidInput
	}
	topic, message, language, err := normalizeFields(command.Topic, command.Message, command.Language)
	if err != nil {
		return UpdateCommand{}, err
	}
	command.Topic = topic
	command.Message = message
	command.Language = language
	if err := validateDates(command.DateStart, command.DateEnd); err != nil {
		return UpdateCommand{}, err
	}
	return command, nil
}

func normalizeFields(topic, message, language string) (string, string, string, error) {
	topic = strings.TrimSpace(topic)
	message = strings.TrimSpace(message)
	language = strings.ToLower(strings.TrimSpace(language))
	if topic == "" || utf8.RuneCountInString(topic) > 50 ||
		message == "" || utf8.RuneCountInString(message) > maximumBodySize ||
		!isLanguageCode(language) {
		return "", "", "", ErrInvalidInput
	}
	return topic, message, language, nil
}

func validateDates(start, end *time.Time) error {
	if start == nil && end == nil {
		return nil
	}
	if start == nil || end == nil {
		return ErrDateRange
	}
	if end.Before(*start) {
		return ErrDateRange
	}
	return nil
}

func isLanguageCode(value string) bool {
	if len(value) != 2 {
		return false
	}
	for _, character := range value {
		if character < 'a' || character > 'z' {
			return false
		}
	}
	return true
}

type Repository interface {
	List(ctx context.Context, query ListQuery) (Page, error)
	Options(ctx context.Context) (Options, error)
	LanguageExists(ctx context.Context, language string) (bool, error)
	CountByLanguage(ctx context.Context, language string, translatedID int64) (int64, error)
	Get(ctx context.Context, id int64) (Announcement, error)
	FindOverlap(ctx context.Context, language string, start, end time.Time, excludeID int64) (bool, error)
	Create(ctx context.Context, value Announcement, actor string) (Announcement, error)
	Update(ctx context.Context, value Announcement, actor string) (Announcement, error)
	Delete(ctx context.Context, id int64) error
}

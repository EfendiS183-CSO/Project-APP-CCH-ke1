package languages

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const TableVariable = "cch_languages"

var (
	ErrInvalidInput        = errors.New("invalid language input")
	ErrNotFound            = errors.New("language not found")
	ErrDuplicate           = errors.New("language code already exists")
	ErrOnlyOneDefault      = errors.New("only one language can be the default")
	ErrCannotDeleteDefault = errors.New("the default language cannot be deleted")
	ErrServiceNotReady     = errors.New("language master is not configured")
)

// Language mirrors one legacy cch_languages row. Language_Code is the char(2)
// primary key and is immutable on update. Only one row may carry IsDefault.
type Language struct {
	Code                      string  `json:"language_code"`
	Name                      string  `json:"language_name"`
	IsDefault                 bool    `json:"is_default"`
	SiteLogo                  string  `json:"site_logo"`
	SiteTitle                 string  `json:"site_title"`
	DefaultThousandsSeparator *string `json:"default_thousands_separator,omitempty"`
	DefaultDecimalPoint       *string `json:"default_decimal_point,omitempty"`
	DefaultCurrencySymbol     *string `json:"default_currency_symbol,omitempty"`
	DefaultMoneyThousandsSep  *string `json:"default_money_thousands_separator,omitempty"`
	DefaultMoneyDecimalPoint  *string `json:"default_money_decimal_point,omitempty"`
	TermsAndConditionText     string  `json:"terms_and_condition_text"`
	AnnouncementText          string  `json:"announcement_text"`
	AboutText                 string  `json:"about_text"`
}

type CreateCommand struct {
	Code                      string
	Name                      string
	IsDefault                 bool
	SiteLogo                  string
	SiteTitle                 string
	DefaultThousandsSeparator *string
	DefaultDecimalPoint       *string
	DefaultCurrencySymbol     *string
	DefaultMoneyThousandsSep  *string
	DefaultMoneyDecimalPoint  *string
	TermsAndConditionText     string
	AnnouncementText          string
	AboutText                 string
}

type UpdateCommand struct {
	Code                      string
	Name                      string
	IsDefault                 bool
	SiteLogo                  string
	SiteTitle                 string
	DefaultThousandsSeparator *string
	DefaultDecimalPoint       *string
	DefaultCurrencySymbol     *string
	DefaultMoneyThousandsSep  *string
	DefaultMoneyDecimalPoint  *string
	TermsAndConditionText     string
	AnnouncementText          string
	AboutText                 string
}

type Repository interface {
	List(context.Context) ([]Language, error)
	Get(context.Context, string) (Language, error)
	Insert(context.Context, Language) error
	Update(context.Context, Language) error
	Delete(context.Context, string) error
}

func normalizeCode(code string) (string, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if !utf8.ValidString(code) || utf8.RuneCountInString(code) != 2 {
		return "", ErrInvalidInput
	}
	return code, nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 20 {
		return "", ErrInvalidInput
	}
	return name, nil
}

func normalizeSiteText(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 100 {
		return "", ErrInvalidInput
	}
	return value, nil
}

func normalizeOptionalText(value *string, maxRunes int) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	if !utf8.ValidString(trimmed) || utf8.RuneCountInString(trimmed) > maxRunes {
		return nil, ErrInvalidInput
	}
	return &trimmed, nil
}

func normalizeBodyText(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) {
		return "", ErrInvalidInput
	}
	return value, nil
}

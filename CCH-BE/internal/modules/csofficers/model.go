package csofficers

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	TableVariable   = "cch_cs_officer"
	DefaultPageSize = 25
	MaximumPageSize = 100
)

var (
	ErrInvalidInput    = errors.New("CS officer query is invalid")
	ErrNotFound        = errors.New("CS officer not found")
	ErrServiceNotReady = errors.New("CS officer service is not configured")
)

type Officer struct {
	Username   string     `json:"username"`
	Regional   *string    `json:"regional,omitempty"`
	Office     *string    `json:"office,omitempty"`
	FullName   *string    `json:"full_name,omitempty"`
	Status     *string    `json:"status,omitempty"`
	Email      *string    `json:"email,omitempty"`
	Phone      *string    `json:"phone,omitempty"`
	LastLogin  *time.Time `json:"last_login,omitempty"`
	LastLogout *time.Time `json:"last_logout,omitempty"`
}

type ListQuery struct {
	Regional string
	Office   string
	Status   string
	Search   string
	Limit    int
	Offset   int
}

type Page struct {
	Data   []Officer `json:"data"`
	Total  int64     `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

type Repository interface {
	List(context.Context, ListQuery) (Page, error)
	Get(context.Context, string) (Officer, error)
}

func (query ListQuery) Normalize() (ListQuery, error) {
	query.Regional = strings.TrimSpace(query.Regional)
	query.Office = strings.TrimSpace(query.Office)
	query.Status = strings.TrimSpace(query.Status)
	query.Search = strings.TrimSpace(query.Search)
	if query.Limit == 0 {
		query.Limit = DefaultPageSize
	}
	if query.Limit < 1 || query.Limit > MaximumPageSize || query.Offset < 0 ||
		!utf8.ValidString(query.Regional) || utf8.RuneCountInString(query.Regional) > 50 ||
		!utf8.ValidString(query.Office) || utf8.RuneCountInString(query.Office) > 7 ||
		!utf8.ValidString(query.Search) || utf8.RuneCountInString(query.Search) > 50 {
		return ListQuery{}, ErrInvalidInput
	}
	if query.Status != "" && query.Status != "Login" && query.Status != "Logout" {
		return ListQuery{}, ErrInvalidInput
	}
	return query, nil
}

func NormalizeUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || !utf8.ValidString(username) || utf8.RuneCountInString(username) > 50 {
		return "", ErrInvalidInput
	}
	return username, nil
}

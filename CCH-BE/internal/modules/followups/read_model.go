package followups

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

var (
	ErrInvalidQuery             = errors.New("invalid follow-up query")
	ErrNotFoundOrNotVisible     = errors.New("follow-ups not found or not visible")
	ErrServiceNotConfigured     = errors.New("follow-up service is not configured")
	ErrCallCenterRoleRequired   = errors.New("Call Center role is required")
	ErrCallCenterOfficeRequired = errors.New("current CCH office is required")
)

type Query struct {
	ComplaintID string
	OfficeCode  string
	Limit       int
	Offset      int
}

type OfficeQuery struct {
	OfficeCode string
	Limit      int
	Offset     int
}

func (query OfficeQuery) NormalizeRequest() (OfficeQuery, error) {
	if query.Limit == 0 {
		query.Limit = DefaultPageSize
	}
	if query.Limit < 1 || query.Limit > MaxPageSize || query.Offset < 0 {
		return OfficeQuery{}, ErrInvalidQuery
	}
	return query, nil
}

func (query OfficeQuery) Normalize() (OfficeQuery, error) {
	query, err := query.NormalizeRequest()
	if err != nil {
		return OfficeQuery{}, err
	}
	query.OfficeCode = strings.TrimSpace(query.OfficeCode)
	if query.OfficeCode == "" || len(query.OfficeCode) > 7 || strings.Contains(query.OfficeCode, ",") {
		return OfficeQuery{}, ErrInvalidQuery
	}
	return query, nil
}

func (query Query) NormalizeRequest() (Query, error) {
	query.ComplaintID = strings.TrimSpace(query.ComplaintID)
	if query.ComplaintID == "" || len(query.ComplaintID) > 25 || strings.Contains(query.ComplaintID, ",") {
		return Query{}, ErrInvalidQuery
	}
	if query.Limit == 0 {
		query.Limit = DefaultPageSize
	}
	if query.Limit < 1 || query.Limit > MaxPageSize || query.Offset < 0 {
		return Query{}, ErrInvalidQuery
	}
	return query, nil
}

func (query Query) Normalize() (Query, error) {
	query, err := query.NormalizeRequest()
	if err != nil {
		return Query{}, err
	}
	query.OfficeCode = strings.TrimSpace(query.OfficeCode)
	if query.OfficeCode == "" || len(query.OfficeCode) > 7 || strings.Contains(query.OfficeCode, ",") {
		return Query{}, ErrInvalidQuery
	}
	return query, nil
}

type Entry struct {
	ComplaintID string
	OfficeCode  string
	StatusCode  string
	PlannedAt   *time.Time
	PlanNotes   string
	RealizedAt  *time.Time
	Outcome     string
}

type Repository interface {
	ListForOpenCallCenterTicket(context.Context, Query) ([]Entry, error)
	ListForCallCenterOffice(context.Context, OfficeQuery) ([]Entry, error)
	ListDueTodayForCallCenterOffice(context.Context, OfficeQuery) ([]Entry, error)
}

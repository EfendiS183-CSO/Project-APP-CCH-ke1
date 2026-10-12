package review

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/intake/submission"
	"cch.local/backend/internal/platform/authentication"
)

const (
	customerProfileTable = "cchentricus"
	complaintTable       = "cchentri"
	callCenterRoleID     = 3
	defaultPageSize      = 25
	maximumPageSize      = 100
	maximumPageOffset    = 100000
)

var (
	ErrServiceNotConfigured = errors.New("intake review service is not configured")
	ErrReviewerForbidden    = errors.New("intake review requires an authorized Call Center reviewer")
	ErrInvalidQuery         = errors.New("invalid intake review query")
	ErrDraftNotFound        = errors.New("pending intake draft not found")
)

type Query struct {
	Limit  int
	Offset int
}

func (query Query) Normalize() (Query, error) {
	if query.Limit == 0 {
		query.Limit = defaultPageSize
	}
	if query.Limit < 1 || query.Limit > maximumPageSize || query.Offset < 0 || query.Offset > maximumPageOffset {
		return Query{}, ErrInvalidQuery
	}
	return query, nil
}

type DraftSummary struct {
	ID           int64
	Summary      string
	ReviewReason string
	CreatedAt    time.Time
}

type PendingDraft struct {
	ID    int64
	Draft submission.Draft
}

type CustomerCandidate struct {
	ID      int64
	Name    string
	Email   string
	Phone   string
	Address string
}

type Repository interface {
	ListPending(context.Context, Query) ([]DraftSummary, error)
	GetPending(context.Context, int64) (PendingDraft, bool, error)
	SearchCustomers(context.Context, string) ([]CustomerCandidate, error)
}

type AccessAuthorizer interface {
	Authorize(context.Context, authentication.VerifiedIdentity, string, authorization.Action) (authorization.AccountAccess, error)
}

type Service struct {
	authorizer   AccessAuthorizer
	repository   Repository
	intakeOffice string
}

func NewService(authorizer AccessAuthorizer, repository Repository, intakeOffice string) (*Service, error) {
	intakeOffice = strings.TrimSpace(intakeOffice)
	if authorizer == nil || repository == nil || intakeOffice == "" || len(intakeOffice) > 7 || strings.Contains(intakeOffice, ",") {
		return nil, ErrServiceNotConfigured
	}
	return &Service{authorizer: authorizer, repository: repository, intakeOffice: intakeOffice}, nil
}

func (service *Service) ListPending(ctx context.Context, identity authentication.VerifiedIdentity, query Query) ([]DraftSummary, error) {
	if err := service.authorizeReviewer(ctx, identity); err != nil {
		return nil, err
	}
	normalized, err := query.Normalize()
	if err != nil {
		return nil, err
	}
	return service.repository.ListPending(ctx, normalized)
}

func (service *Service) GetPending(ctx context.Context, identity authentication.VerifiedIdentity, draftID int64) (PendingDraft, error) {
	if err := service.authorizeReviewer(ctx, identity); err != nil {
		return PendingDraft{}, err
	}
	if draftID <= 0 {
		return PendingDraft{}, ErrInvalidQuery
	}
	draft, found, err := service.repository.GetPending(ctx, draftID)
	if err != nil {
		return PendingDraft{}, err
	}
	if !found {
		return PendingDraft{}, ErrDraftNotFound
	}
	return draft, nil
}

func (service *Service) SearchCustomers(ctx context.Context, identity authentication.VerifiedIdentity, query string) ([]CustomerCandidate, error) {
	if err := service.authorizeReviewer(ctx, identity); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) < 3 || utf8.RuneCountInString(query) > 100 {
		return nil, ErrInvalidQuery
	}
	return service.repository.SearchCustomers(ctx, query)
}

func (service *Service) authorizeReviewer(ctx context.Context, identity authentication.VerifiedIdentity) error {
	if service == nil || service.authorizer == nil || service.repository == nil {
		return ErrServiceNotConfigured
	}
	if strings.TrimSpace(identity.Username) == "" {
		return ErrReviewerForbidden
	}
	customerAccess, err := service.authorizer.Authorize(ctx, identity, customerProfileTable, authorization.ActionView)
	if err != nil {
		return ErrReviewerForbidden
	}
	ticketAccess, err := service.authorizer.Authorize(ctx, identity, complaintTable, authorization.ActionAdd)
	if err != nil {
		return ErrReviewerForbidden
	}
	if !isIntakeCallCenter(customerAccess, service.intakeOffice) || !isIntakeCallCenter(ticketAccess, service.intakeOffice) ||
		customerAccess.OfficeCode != ticketAccess.OfficeCode {
		return ErrReviewerForbidden
	}
	return nil
}

func isIntakeCallCenter(access authorization.AccountAccess, intakeOffice string) bool {
	return access.OfficeCode == intakeOffice && len(access.UserLevelIDs) == 1 && access.UserLevelIDs[0] == callCenterRoleID
}

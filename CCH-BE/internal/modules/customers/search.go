package customers

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

const (
	profileTableName    = "cchentricus"
	callCenterRoleID    = 3
	defaultSearchLimit  = 20
	maxSearchLimit      = 20
	maxSearchQueryRunes = 100
)

var (
	ErrInvalidSearch        = errors.New("invalid customer search")
	ErrServiceNotConfigured = errors.New("customer search service is not configured")
	ErrCallCenterRequired   = errors.New("Call Center role is required for customer search")
	ErrOfficeRequired       = errors.New("current CCH office is required for customer search")
	ErrInvalidProfileID     = errors.New("invalid customer profile ID")
	ErrProfileNotFound      = errors.New("customer profile not found or not visible")
)

type SearchQuery struct {
	Term       string
	OfficeCode string
	Limit      int
}

func (query SearchQuery) NormalizeRequest() (SearchQuery, error) {
	query.Term = strings.TrimSpace(query.Term)
	termLength := utf8.RuneCountInString(query.Term)
	if termLength < 3 || termLength > maxSearchQueryRunes {
		return SearchQuery{}, ErrInvalidSearch
	}
	if query.Limit == 0 {
		query.Limit = defaultSearchLimit
	}
	if query.Limit < 1 || query.Limit > maxSearchLimit {
		return SearchQuery{}, ErrInvalidSearch
	}
	return query, nil
}

func (query SearchQuery) Normalize() (SearchQuery, error) {
	query, err := query.NormalizeRequest()
	if err != nil {
		return SearchQuery{}, err
	}
	query.OfficeCode = strings.TrimSpace(query.OfficeCode)
	if query.OfficeCode == "" || len(query.OfficeCode) > 7 || strings.Contains(query.OfficeCode, ",") {
		return SearchQuery{}, ErrInvalidSearch
	}
	return query, nil
}

type Profile struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Address  string `json:"address"`
}

type SearchRepository interface {
	SearchCallCenterProfiles(context.Context, SearchQuery) ([]Profile, error)
	GetCallCenterProfileForOffice(context.Context, int64, string) (Profile, error)
}

type CallCenterSearchService struct {
	accessService *authorization.AccessService
	repository    SearchRepository
}

func NewCallCenterSearchService(accessService *authorization.AccessService, repository SearchRepository) *CallCenterSearchService {
	return &CallCenterSearchService{accessService: accessService, repository: repository}
}

func (service *CallCenterSearchService) Search(ctx context.Context, identity authentication.VerifiedIdentity, query SearchQuery) ([]Profile, error) {
	if service == nil || service.accessService == nil || service.repository == nil {
		return nil, ErrServiceNotConfigured
	}
	query, err := query.NormalizeRequest()
	if err != nil {
		return nil, err
	}
	access, err := service.accessService.Authorize(ctx, identity, profileTableName, authorization.ActionLookup)
	if err != nil {
		return nil, err
	}
	if len(access.UserLevelIDs) != 1 || access.UserLevelIDs[0] != callCenterRoleID {
		return nil, ErrCallCenterRequired
	}
	query.OfficeCode = access.OfficeCode
	query, err = query.Normalize()
	if err != nil {
		return nil, ErrOfficeRequired
	}
	return service.repository.SearchCallCenterProfiles(ctx, query)
}

func (service *CallCenterSearchService) GetProfileForTicket(ctx context.Context, identity authentication.VerifiedIdentity, profileID int64) (Profile, error) {
	if service == nil || service.accessService == nil || service.repository == nil {
		return Profile{}, ErrServiceNotConfigured
	}
	if profileID <= 0 {
		return Profile{}, ErrInvalidProfileID
	}
	access, err := service.accessService.Authorize(ctx, identity, profileTableName, authorization.ActionLookup)
	if err != nil {
		return Profile{}, err
	}
	if len(access.UserLevelIDs) != 1 || access.UserLevelIDs[0] != callCenterRoleID {
		return Profile{}, ErrCallCenterRequired
	}
	officeCode := strings.TrimSpace(access.OfficeCode)
	if officeCode == "" || len(officeCode) > 7 || strings.Contains(officeCode, ",") {
		return Profile{}, ErrOfficeRequired
	}
	profile, err := service.repository.GetCallCenterProfileForOffice(ctx, profileID, officeCode)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrProfileNotFound
	}
	return profile, err
}

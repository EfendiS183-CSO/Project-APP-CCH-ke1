package review

import (
	"context"
	"errors"
	"testing"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type authorizerStub struct {
	accessByTable map[string]authorization.AccountAccess
	errByTable    map[string]error
	tables        []string
	actions       []authorization.Action
}

func (authorizer *authorizerStub) Authorize(_ context.Context, _ authentication.VerifiedIdentity, table string, action authorization.Action) (authorization.AccountAccess, error) {
	authorizer.tables = append(authorizer.tables, table)
	authorizer.actions = append(authorizer.actions, action)
	if err := authorizer.errByTable[table]; err != nil {
		return authorization.AccountAccess{}, err
	}
	return authorizer.accessByTable[table], nil
}

type repositoryStub struct {
	listCalled   bool
	getCalled    bool
	searchCalled bool
	query        Query
	searchQuery  string
	drafts       []DraftSummary
	candidates   []CustomerCandidate
	draft        PendingDraft
	found        bool
}

func (repository *repositoryStub) ListPending(_ context.Context, query Query) ([]DraftSummary, error) {
	repository.listCalled = true
	repository.query = query
	return repository.drafts, nil
}

func (repository *repositoryStub) GetPending(context.Context, int64) (PendingDraft, bool, error) {
	repository.getCalled = true
	return repository.draft, repository.found, nil
}

func (repository *repositoryStub) SearchCustomers(_ context.Context, query string) ([]CustomerCandidate, error) {
	repository.searchCalled = true
	repository.searchQuery = query
	return repository.candidates, nil
}

func TestListPendingRequiresCallCenterViewAndAddAtIntakeOffice(t *testing.T) {
	authorizer := validAuthorizer()
	repository := &repositoryStub{drafts: []DraftSummary{{ID: 10, Summary: "SUMMARY"}}}
	service := newTestService(t, authorizer, repository)
	results, err := service.ListPending(context.Background(), reviewerIdentity(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !repository.listCalled || repository.query != (Query{Limit: defaultPageSize}) {
		t.Fatalf("unexpected queue result/query: results=%#v query=%#v", results, repository.query)
	}
	if len(authorizer.tables) != 2 || authorizer.tables[0] != customerProfileTable || authorizer.actions[0] != authorization.ActionView ||
		authorizer.tables[1] != complaintTable || authorizer.actions[1] != authorization.ActionAdd {
		t.Fatalf("reviewer ACL checks = %#v/%#v", authorizer.tables, authorizer.actions)
	}
}

func TestReviewerDenialStopsRepositoryAccess(t *testing.T) {
	for _, test := range []struct {
		name        string
		accessTable string
		access      authorization.AccountAccess
		accessError error
	}{
		{name: "role", accessTable: customerProfileTable, access: authorization.AccountAccess{OfficeCode: "40005CC", UserLevelIDs: []int64{1}}},
		{name: "office", accessTable: complaintTable, access: authorization.AccountAccess{OfficeCode: "40006CC", UserLevelIDs: []int64{callCenterRoleID}}},
		{name: "permission", accessTable: complaintTable, accessError: authorization.ErrPermissionDenied},
	} {
		t.Run(test.name, func(t *testing.T) {
			authorizer := validAuthorizer()
			if test.accessTable != "" {
				authorizer.accessByTable[test.accessTable] = test.access
			}
			if test.accessError != nil {
				authorizer.errByTable[test.accessTable] = test.accessError
			}
			repository := &repositoryStub{}
			service := newTestService(t, authorizer, repository)
			if _, err := service.ListPending(context.Background(), reviewerIdentity(), Query{}); !errors.Is(err, ErrReviewerForbidden) {
				t.Fatalf("ListPending() error = %v, want forbidden", err)
			}
			if repository.listCalled {
				t.Fatal("repository was called for a denied reviewer")
			}
		})
	}
}

func TestGetPendingValidatesIDAndMapsMissingDraft(t *testing.T) {
	service := newTestService(t, validAuthorizer(), &repositoryStub{})
	if _, err := service.GetPending(context.Background(), reviewerIdentity(), 0); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("invalid draft ID error = %v", err)
	}
	missingRepository := &repositoryStub{}
	service = newTestService(t, validAuthorizer(), missingRepository)
	if _, err := service.GetPending(context.Background(), reviewerIdentity(), 42); !errors.Is(err, ErrDraftNotFound) {
		t.Fatalf("missing draft error = %v", err)
	}
	if !missingRepository.getCalled {
		t.Fatal("repository was not queried for valid draft ID")
	}
}

func TestReviewerWithoutUsernameIsDeniedBeforeRepositoryAccess(t *testing.T) {
	repository := &repositoryStub{}
	service := newTestService(t, validAuthorizer(), repository)
	if _, err := service.ListPending(context.Background(), authentication.VerifiedIdentity{}, Query{}); !errors.Is(err, ErrReviewerForbidden) {
		t.Fatalf("ListPending() error = %v, want forbidden", err)
	}
	if repository.listCalled {
		t.Fatal("pending draft query ran without an authenticated CCH username")
	}
}

func TestSearchCustomersRequiresReviewerAndNormalizesQuery(t *testing.T) {
	authorizer := validAuthorizer()
	repository := &repositoryStub{candidates: []CustomerCandidate{{ID: 21, Name: "CUSTOMER"}}}
	service := newTestService(t, authorizer, repository)
	candidates, err := service.SearchCustomers(context.Background(), reviewerIdentity(), "  CUSTOMER  ")
	if err != nil {
		t.Fatal(err)
	}
	if !repository.searchCalled || repository.searchQuery != "CUSTOMER" || len(candidates) != 1 || candidates[0].ID != 21 {
		t.Fatalf("unexpected search result/query: %#v, %q", candidates, repository.searchQuery)
	}
	denied := validAuthorizer()
	denied.errByTable[customerProfileTable] = authorization.ErrPermissionDenied
	blockedRepository := &repositoryStub{}
	if _, err := newTestService(t, denied, blockedRepository).SearchCustomers(context.Background(), reviewerIdentity(), "customer"); !errors.Is(err, ErrReviewerForbidden) {
		t.Fatalf("unauthorized customer search error = %v", err)
	}
	if blockedRepository.searchCalled {
		t.Fatal("customer repository was called without profile View permission")
	}
}

func TestQueryBoundsPagination(t *testing.T) {
	for _, query := range []Query{{Limit: maximumPageSize + 1}, {Offset: -1}, {Offset: maximumPageOffset + 1}} {
		if _, err := query.Normalize(); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("Normalize(%#v) error = %v, want invalid query", query, err)
		}
	}
}

func TestSearchQueryBoundsLength(t *testing.T) {
	for _, query := range []string{"ab", string(make([]byte, 101))} {
		if _, err := newTestService(t, validAuthorizer(), &repositoryStub{}).SearchCustomers(context.Background(), reviewerIdentity(), query); !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("SearchCustomers(%d chars) error = %v", len(query), err)
		}
	}
}

func newTestService(t *testing.T, authorizer AccessAuthorizer, repository Repository) *Service {
	t.Helper()
	service, err := NewService(authorizer, repository, "40005CC")
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func validAuthorizer() *authorizerStub {
	return &authorizerStub{
		accessByTable: map[string]authorization.AccountAccess{
			customerProfileTable: {OfficeCode: "40005CC", UserLevelIDs: []int64{callCenterRoleID}},
			complaintTable:       {OfficeCode: "40005CC", UserLevelIDs: []int64{callCenterRoleID}},
		},
		errByTable: make(map[string]error),
	}
}

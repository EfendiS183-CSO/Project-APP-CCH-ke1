package followups

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type stubAccessLoader struct {
	defaultAccess authorization.AccountAccess
	byTable       map[string]authorization.AccountAccess
	err           error
	loadedTables  []string
}

func (loader *stubAccessLoader) Load(_ context.Context, _ authentication.VerifiedIdentity, tableVariable string) (authorization.AccountAccess, error) {
	loader.loadedTables = append(loader.loadedTables, tableVariable)
	if access, exists := loader.byTable[tableVariable]; exists {
		return access, loader.err
	}
	return loader.defaultAccess, loader.err
}

type stubRepository struct {
	query          Query
	officeQuery    OfficeQuery
	dueTodayQuery  OfficeQuery
	entries        []Entry
	err            error
	called         bool
	officeCalled   bool
	dueTodayCalled bool
}

func (repository *stubRepository) ListForOpenCallCenterTicket(_ context.Context, query Query) ([]Entry, error) {
	repository.query = query
	repository.called = true
	return repository.entries, repository.err
}

func (repository *stubRepository) ListForCallCenterOffice(_ context.Context, query OfficeQuery) ([]Entry, error) {
	repository.officeQuery = query
	repository.officeCalled = true
	return repository.entries, repository.err
}

func (repository *stubRepository) ListDueTodayForCallCenterOffice(_ context.Context, query OfficeQuery) ([]Entry, error) {
	repository.dueTodayQuery = query
	repository.dueTodayCalled = true
	return repository.entries, repository.err
}

func TestServiceListsRelatedOfficeFollowUpsUsingTrustedParentScope(t *testing.T) {
	loader := newCallCenterLoader()
	repository := &stubRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)

	_, err := service.ListForCallCenterTicket(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, Query{
		ComplaintID: " TKT-001 ",
		OfficeCode:  "ATTACKER",
		Limit:       10,
		Offset:      20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.called || repository.query.ComplaintID != "TKT-001" || repository.query.OfficeCode != "40005CC" {
		t.Fatalf("follow-up query has untrusted scope: %#v", repository.query)
	}
	if repository.query.Limit != 10 || repository.query.Offset != 20 {
		t.Fatalf("pagination not preserved: %#v", repository.query)
	}
	if len(loader.loadedTables) != 2 || loader.loadedTables[0] != complaintTableName || loader.loadedTables[1] != followUpViewName {
		t.Fatalf("authorized tables = %#v", loader.loadedTables)
	}
}

func TestServiceListsFollowUpsForTrustedCallCenterOffice(t *testing.T) {
	loader := newCallCenterLoader()
	loader.byTable[followUpTableName] = authorization.AccountAccess{
		Username:       "agent.test",
		OfficeCode:     "40005CC",
		ReportTo:       "40005",
		UserLevelIDs:   []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionList),
	}
	repository := &stubRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)

	_, err := service.ListForCallCenterOffice(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, OfficeQuery{
		OfficeCode: "ATTACKER",
		Limit:      10,
		Offset:     20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.officeCalled || repository.officeQuery.OfficeCode != "40005CC" {
		t.Fatalf("follow-up query has untrusted office scope: %#v", repository.officeQuery)
	}
	if repository.officeQuery.Limit != 10 || repository.officeQuery.Offset != 20 {
		t.Fatalf("pagination not preserved: %#v", repository.officeQuery)
	}
	if len(loader.loadedTables) != 1 || loader.loadedTables[0] != followUpTableName {
		t.Fatalf("authorized tables = %#v", loader.loadedTables)
	}
}

func TestServiceListsDueTodayUsingTrustedOfficeAndViewPermission(t *testing.T) {
	loader := newCallCenterLoader()
	loader.byTable[followUpDueTodayViewName] = authorization.AccountAccess{
		Username:       "agent.test",
		OfficeCode:     "40005CC",
		ReportTo:       "40005",
		UserLevelIDs:   []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionList),
	}
	repository := &stubRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.ListDueTodayForCallCenterOffice(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, OfficeQuery{
		OfficeCode: "ATTACKER",
		Limit:      10,
		Offset:     20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.dueTodayCalled || repository.dueTodayQuery.OfficeCode != "40005CC" || repository.dueTodayQuery.Limit != 10 || repository.dueTodayQuery.Offset != 20 {
		t.Fatalf("due-today query has untrusted scope or pagination: %#v", repository.dueTodayQuery)
	}
	if len(loader.loadedTables) != 1 || loader.loadedTables[0] != followUpDueTodayViewName {
		t.Fatalf("authorized tables = %#v", loader.loadedTables)
	}
}

func TestServiceRequiresDueTodayListPermissionAndCallCenterRole(t *testing.T) {
	for _, test := range []struct {
		name    string
		access  authorization.AccountAccess
		wantErr error
	}{
		{
			name: "missing list permission",
			access: authorization.AccountAccess{
				OfficeCode:   "40005CC",
				UserLevelIDs: []int64{callCenterRoleID},
			},
			wantErr: authorization.ErrPermissionDenied,
		},
		{
			name: "unsupported role",
			access: authorization.AccountAccess{
				OfficeCode:     "40005CC",
				UserLevelIDs:   []int64{1},
				PermissionMask: int64(authorization.ActionList),
			},
			wantErr: ErrCallCenterRoleRequired,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := newCallCenterLoader()
			loader.byTable[followUpDueTodayViewName] = test.access
			repository := &stubRepository{}
			service := NewService(authorization.NewAccessService(loader), repository)
			_, err := service.ListDueTodayForCallCenterOffice(context.Background(), authentication.VerifiedIdentity{}, OfficeQuery{})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ListDueTodayForCallCenterOffice() error = %v, want %v", err, test.wantErr)
			}
			if repository.dueTodayCalled {
				t.Fatal("due-today repository called without authorization")
			}
		})
	}
}

func TestServiceRejectsDueTodayWithoutTrustedOffice(t *testing.T) {
	loader := &stubAccessLoader{defaultAccess: authorization.AccountAccess{
		UserLevelIDs:   []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionList),
	}}
	repository := &stubRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.ListDueTodayForCallCenterOffice(context.Background(), authentication.VerifiedIdentity{}, OfficeQuery{})
	if !errors.Is(err, ErrCallCenterOfficeRequired) {
		t.Fatalf("ListDueTodayForCallCenterOffice() error = %v, want trusted office required", err)
	}
	if repository.dueTodayCalled {
		t.Fatal("due-today repository called without a trusted office")
	}
}

func TestServiceRejectsCallCenterOfficeListWithoutPermissionOrSingleCallCenterRole(t *testing.T) {
	for _, test := range []struct {
		name    string
		access  authorization.AccountAccess
		wantErr error
	}{
		{
			name: "missing list permission",
			access: authorization.AccountAccess{
				Username:     "agent.test",
				OfficeCode:   "40005CC",
				UserLevelIDs: []int64{callCenterRoleID},
			},
			wantErr: authorization.ErrPermissionDenied,
		},
		{
			name: "non-call-center role",
			access: authorization.AccountAccess{
				Username:       "agent.test",
				OfficeCode:     "40005CC",
				UserLevelIDs:   []int64{1},
				PermissionMask: int64(authorization.ActionList),
			},
			wantErr: ErrCallCenterRoleRequired,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := newCallCenterLoader()
			loader.byTable[followUpTableName] = test.access
			repository := &stubRepository{}
			service := NewService(authorization.NewAccessService(loader), repository)
			_, err := service.ListForCallCenterOffice(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, OfficeQuery{})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ListForCallCenterOffice() error = %v, want %v", err, test.wantErr)
			}
			if repository.officeCalled {
				t.Fatal("follow-up repository called without authorization")
			}
		})
	}
}

func TestServiceRequiresBothParentViewAndFollowUpListPermissions(t *testing.T) {
	loader := newCallCenterLoader()
	loader.byTable = map[string]authorization.AccountAccess{
		followUpViewName: {
			Username:       "agent.test",
			OfficeCode:     "40005CC",
			ReportTo:       "40005",
			UserLevelIDs:   []int64{callCenterRoleID},
			PermissionMask: int64(authorization.ActionView),
		},
	}
	repository := &stubRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.ListForCallCenterTicket(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, Query{ComplaintID: "TKT-001"})
	if !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("ListForCallCenterTicket() error = %v, want cchfual List denial", err)
	}
	if repository.called {
		t.Fatal("follow-up repository called without permission")
	}
}

func TestServiceRejectsOtherOrMultiRoleAccounts(t *testing.T) {
	for _, roles := range [][]int64{{1}, {callCenterRoleID, 1}} {
		loader := newCallCenterLoader()
		loader.defaultAccess.UserLevelIDs = roles
		loader.byTable = map[string]authorization.AccountAccess{
			followUpViewName: {
				Username:       "agent.test",
				OfficeCode:     "40005CC",
				ReportTo:       "40005",
				UserLevelIDs:   roles,
				PermissionMask: int64(authorization.ActionList),
			},
		}
		repository := &stubRepository{}
		service := NewService(authorization.NewAccessService(loader), repository)
		_, err := service.ListForCallCenterTicket(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, Query{ComplaintID: "TKT-001"})
		if !errors.Is(err, ErrCallCenterRoleRequired) {
			t.Fatalf("role set %v error = %v, want Call Center denial", roles, err)
		}
		if repository.called {
			t.Fatalf("role set %v reached repository", roles)
		}
	}
}

func TestServiceMapsMissingAndOutOfScopeTicketToSameError(t *testing.T) {
	loader := newCallCenterLoader()
	repository := &stubRepository{err: sql.ErrNoRows}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.ListForCallCenterTicket(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, Query{ComplaintID: "TKT-UNKNOWN"})
	if !errors.Is(err, ErrNotFoundOrNotVisible) {
		t.Fatalf("ListForCallCenterTicket() error = %v, want not found or not visible", err)
	}
}

func TestServiceRequiresDependenciesAndRejectsInvalidQueryBeforeAuthorization(t *testing.T) {
	var service *Service
	_, err := service.ListForCallCenterTicket(context.Background(), authentication.VerifiedIdentity{}, Query{ComplaintID: "TKT-001"})
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Fatalf("service error = %v, want service not configured", err)
	}
	loader := newCallCenterLoader()
	service = NewService(authorization.NewAccessService(loader), &stubRepository{})
	_, err = service.ListForCallCenterTicket(context.Background(), authentication.VerifiedIdentity{}, Query{})
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("query error = %v, want invalid follow-up query", err)
	}
	if len(loader.loadedTables) != 0 {
		t.Fatal("invalid query reached authorization loader")
	}
}

func newCallCenterLoader() *stubAccessLoader {
	access := authorization.AccountAccess{
		Username:       "agent.test",
		OfficeCode:     "40005CC",
		ReportTo:       "40005",
		UserLevelIDs:   []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionView | authorization.ActionList),
	}
	return &stubAccessLoader{
		defaultAccess: access,
		byTable:       map[string]authorization.AccountAccess{followUpViewName: access},
	}
}

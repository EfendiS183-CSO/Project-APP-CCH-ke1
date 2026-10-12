package customers

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type searchAccessLoader struct {
	access authorization.AccountAccess
	err    error
	loaded string
}

func (loader *searchAccessLoader) Load(_ context.Context, _ authentication.VerifiedIdentity, tableVariable string) (authorization.AccountAccess, error) {
	loader.loaded = tableVariable
	return loader.access, loader.err
}

type searchRepositoryStub struct {
	query         SearchQuery
	profileID     int64
	profileOffice string
	profile       Profile
	results       []Profile
	err           error
	called        bool
	profileCalled bool
}

func (repository *searchRepositoryStub) SearchCallCenterProfiles(_ context.Context, query SearchQuery) ([]Profile, error) {
	repository.called = true
	repository.query = query
	return repository.results, repository.err
}

func (repository *searchRepositoryStub) GetCallCenterProfileForOffice(_ context.Context, profileID int64, officeCode string) (Profile, error) {
	repository.profileCalled = true
	repository.profileID = profileID
	repository.profileOffice = officeCode
	return repository.profile, repository.err
}

func TestCallCenterSearchUsesTrustedOfficeAndBoundedPrefix(t *testing.T) {
	loader := &searchAccessLoader{access: authorization.AccountAccess{
		Username:       "agent.test",
		OfficeCode:     "40005CC",
		UserLevelIDs:   []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionLookup),
	}}
	repository := &searchRepositoryStub{results: []Profile{{ID: 10, FullName: "CUSTOMER TEST", Phone: "0812345678"}}}
	service := NewCallCenterSearchService(authorization.NewAccessService(loader), repository)
	results, err := service.Search(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, SearchQuery{
		Term:       "  cus  ",
		OfficeCode: "ATTACKER",
	})
	if err != nil {
		t.Fatal(err)
	}
	if loader.loaded != profileTableName || !repository.called || repository.query.OfficeCode != "40005CC" || repository.query.Term != "cus" || repository.query.Limit != defaultSearchLimit {
		t.Fatalf("search did not use trusted, bounded query: loader=%q query=%#v", loader.loaded, repository.query)
	}
	if len(results) != 1 || results[0].ID != 10 {
		t.Fatalf("unexpected search results: %#v", results)
	}
}

func TestCallCenterSearchRejectsShortQueryBeforeAuthorization(t *testing.T) {
	loader := &searchAccessLoader{}
	repository := &searchRepositoryStub{}
	service := NewCallCenterSearchService(authorization.NewAccessService(loader), repository)
	_, err := service.Search(context.Background(), authentication.VerifiedIdentity{}, SearchQuery{Term: "ab"})
	if !errors.Is(err, ErrInvalidSearch) {
		t.Fatalf("Search() error = %v, want invalid search", err)
	}
	if loader.loaded != "" || repository.called {
		t.Fatal("short query reached authorization or customer repository")
	}
}

func TestCallCenterSearchRequiresLookupPermissionAndSingleCallCenterRole(t *testing.T) {
	for _, test := range []struct {
		name    string
		access  authorization.AccountAccess
		wantErr error
	}{
		{
			name: "missing lookup permission",
			access: authorization.AccountAccess{
				UserLevelIDs: []int64{callCenterRoleID},
			},
			wantErr: authorization.ErrPermissionDenied,
		},
		{
			name: "unsupported role",
			access: authorization.AccountAccess{
				OfficeCode:     "40005CC",
				UserLevelIDs:   []int64{1},
				PermissionMask: int64(authorization.ActionLookup),
			},
			wantErr: ErrCallCenterRequired,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &searchAccessLoader{access: test.access}
			repository := &searchRepositoryStub{}
			service := NewCallCenterSearchService(authorization.NewAccessService(loader), repository)
			_, err := service.Search(context.Background(), authentication.VerifiedIdentity{}, SearchQuery{Term: "cus"})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Search() error = %v, want %v", err, test.wantErr)
			}
			if repository.called {
				t.Fatal("customer repository called without authorization")
			}
		})
	}
}

func TestCallCenterSearchRequiresTrustedOffice(t *testing.T) {
	loader := &searchAccessLoader{access: authorization.AccountAccess{
		UserLevelIDs:   []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionLookup),
	}}
	repository := &searchRepositoryStub{}
	service := NewCallCenterSearchService(authorization.NewAccessService(loader), repository)
	_, err := service.Search(context.Background(), authentication.VerifiedIdentity{}, SearchQuery{Term: "cus"})
	if !errors.Is(err, ErrOfficeRequired) {
		t.Fatalf("Search() error = %v, want trusted office required", err)
	}
	if repository.called {
		t.Fatal("customer repository called without trusted office")
	}
}

func TestGetProfileForTicketUsesTrustedOfficeAndCanonicalProfile(t *testing.T) {
	loader := &searchAccessLoader{access: authorization.AccountAccess{
		Username:       "agent.test",
		OfficeCode:     "40005CC",
		UserLevelIDs:   []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionLookup),
	}}
	repository := &searchRepositoryStub{profile: Profile{
		ID:       42,
		FullName: "CUSTOMER CANONICAL",
		Email:    "canonical@example.test",
		Phone:    "0812345678",
		Address:  "CANONICAL ADDRESS",
	}}
	service := NewCallCenterSearchService(authorization.NewAccessService(loader), repository)
	profile, err := service.GetProfileForTicket(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, 42)
	if err != nil {
		t.Fatal(err)
	}
	if loader.loaded != profileTableName || !repository.profileCalled || repository.profileID != 42 || repository.profileOffice != "40005CC" {
		t.Fatalf("profile lookup did not use trusted office: loader=%q repository=%#v", loader.loaded, repository)
	}
	if profile.FullName != "CUSTOMER CANONICAL" || profile.Email != "canonical@example.test" || profile.Address != "CANONICAL ADDRESS" {
		t.Fatalf("unexpected canonical profile: %#v", profile)
	}
}

func TestGetProfileForTicketRejectsInvalidIDBeforeAuthorization(t *testing.T) {
	loader := &searchAccessLoader{}
	repository := &searchRepositoryStub{}
	service := NewCallCenterSearchService(authorization.NewAccessService(loader), repository)
	if _, err := service.GetProfileForTicket(context.Background(), authentication.VerifiedIdentity{}, 0); !errors.Is(err, ErrInvalidProfileID) {
		t.Fatalf("GetProfileForTicket() error = %v, want invalid profile ID", err)
	}
	if loader.loaded != "" || repository.profileCalled {
		t.Fatal("invalid profile ID reached authorization or repository")
	}
}

func TestGetProfileForTicketHidesProfilesOutsideOffice(t *testing.T) {
	loader := &searchAccessLoader{access: authorization.AccountAccess{
		OfficeCode:     "40005CC",
		UserLevelIDs:   []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionLookup),
	}}
	repository := &searchRepositoryStub{err: sql.ErrNoRows}
	service := NewCallCenterSearchService(authorization.NewAccessService(loader), repository)
	if _, err := service.GetProfileForTicket(context.Background(), authentication.VerifiedIdentity{}, 42); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("GetProfileForTicket() error = %v, want not found or not visible", err)
	}
	if !repository.profileCalled || repository.profileOffice != "40005CC" {
		t.Fatalf("profile repository was not scoped to the trusted office: %#v", repository)
	}
}

package complaints

import (
	"context"
	"errors"
	"testing"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type lookupAccessLoader struct {
	access authorization.AccountAccess
	err    error
	loaded string
}

func (loader *lookupAccessLoader) Load(_ context.Context, _ authentication.VerifiedIdentity, tableVariable string) (authorization.AccountAccess, error) {
	loader.loaded = tableVariable
	return loader.access, loader.err
}

type lookupRepositoryStub struct {
	typesCalled           bool
	sourcesCalled         bool
	destinationsCalled    bool
	copiesCalled          bool
	servicesCalled        bool
	shippingOfficesCalled bool
	officeCode            string
	types                 []ComplaintTypeOption
	sources               []ComplaintSourceOption
	destinations          []OfficeOption
	copies                []OfficeOption
	services              []ShippingServiceOption
	shippingOffices       []OfficeOption
	err                   error
}

func (repository *lookupRepositoryStub) ListActiveComplaintTypes(context.Context) ([]ComplaintTypeOption, error) {
	repository.typesCalled = true
	return repository.types, repository.err
}

func (repository *lookupRepositoryStub) ListActiveComplaintSources(context.Context) ([]ComplaintSourceOption, error) {
	repository.sourcesCalled = true
	return repository.sources, repository.err
}

func (repository *lookupRepositoryStub) ListDestinationOffices(_ context.Context, officeCode string) ([]OfficeOption, error) {
	repository.destinationsCalled = true
	repository.officeCode = officeCode
	return repository.destinations, repository.err
}

func (repository *lookupRepositoryStub) ListCopyOffices(_ context.Context, officeCode string) ([]OfficeOption, error) {
	repository.copiesCalled = true
	repository.officeCode = officeCode
	return repository.copies, repository.err
}

func (repository *lookupRepositoryStub) ListShippingServices(context.Context) ([]ShippingServiceOption, error) {
	repository.servicesCalled = true
	return repository.services, repository.err
}

func (repository *lookupRepositoryStub) ListShippingOffices(context.Context) ([]OfficeOption, error) {
	repository.shippingOfficesCalled = true
	return repository.shippingOffices, repository.err
}

func TestCallCenterLookupServiceReturnsActiveReferencesAfterLookupAuthorization(t *testing.T) {
	loader := &lookupAccessLoader{access: authorization.AccountAccess{
		Username:       "agent.test",
		OfficeCode:     "40005CC",
		UserLevelIDs:   []int64{callCenterLookupRoleID},
		PermissionMask: int64(authorization.ActionLookup),
	}}
	repository := &lookupRepositoryStub{
		types:           []ComplaintTypeOption{{Code: 1, Description: "Pengaduan"}},
		sources:         []ComplaintSourceOption{{ID: 2, Name: "Live Chat"}},
		destinations:    []OfficeOption{{Code: "40006CC", Name: "Kantor Tujuan"}},
		copies:          []OfficeOption{{Code: "40007CC", Name: "Kantor Tembusan"}},
		services:        []ShippingServiceOption{{Code: "REG", Description: "Reguler"}},
		shippingOffices: []OfficeOption{{Code: "40008CC", Name: "Kantor Kiriman"}},
	}
	service := NewCallCenterLookupService(authorization.NewAccessService(loader), repository)

	lookups, err := service.Get(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"})
	if err != nil {
		t.Fatal(err)
	}
	if loader.loaded != cchComplaintTable || !repository.typesCalled || !repository.sourcesCalled || !repository.destinationsCalled || !repository.copiesCalled || !repository.servicesCalled || !repository.shippingOfficesCalled {
		t.Fatalf("lookup authorization/repository calls unexpected: loader=%q repo=%#v", loader.loaded, repository)
	}
	if repository.officeCode != "40005CC" || lookups.OriginOfficeCode != "40005CC" {
		t.Fatalf("lookup office scope did not come from current account: repository=%q response=%q", repository.officeCode, lookups.OriginOfficeCode)
	}
	if len(lookups.ComplaintTypes) != 1 || lookups.ComplaintTypes[0].Code != 1 || len(lookups.Sources) != 1 || lookups.Sources[0].ID != 2 || len(lookups.DestinationOffices) != 1 || len(lookups.CopyOffices) != 1 || len(lookups.ShippingServices) != 1 || len(lookups.ShippingOffices) != 1 {
		t.Fatalf("unexpected lookups: %#v", lookups)
	}
}

func TestCallCenterLookupServiceRejectsMissingPermissionAndWrongRole(t *testing.T) {
	for _, test := range []struct {
		name    string
		access  authorization.AccountAccess
		wantErr error
	}{
		{
			name: "missing lookup permission",
			access: authorization.AccountAccess{
				UserLevelIDs: []int64{callCenterLookupRoleID},
			},
			wantErr: authorization.ErrPermissionDenied,
		},
		{
			name: "wrong role",
			access: authorization.AccountAccess{
				UserLevelIDs:   []int64{1},
				PermissionMask: int64(authorization.ActionLookup),
			},
			wantErr: ErrLookupCallCenterRoleRequired,
		},
		{
			name: "multi role",
			access: authorization.AccountAccess{
				UserLevelIDs:   []int64{callCenterLookupRoleID, 1},
				PermissionMask: int64(authorization.ActionLookup),
			},
			wantErr: ErrLookupCallCenterRoleRequired,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &lookupAccessLoader{access: test.access}
			repository := &lookupRepositoryStub{}
			service := NewCallCenterLookupService(authorization.NewAccessService(loader), repository)
			_, err := service.Get(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Get() error = %v, want %v", err, test.wantErr)
			}
			if repository.typesCalled || repository.sourcesCalled {
				t.Fatal("lookup repository called without permission and supported role")
			}
		})
	}
}

func TestCallCenterLookupServiceRequiresDependencies(t *testing.T) {
	var service *CallCenterLookupService
	if _, err := service.Get(context.Background(), authentication.VerifiedIdentity{}); !errors.Is(err, ErrLookupServiceNotConfigured) {
		t.Fatalf("Get() error = %v, want unconfigured service", err)
	}
}

func TestCallCenterLookupServiceRequiresTrustedOffice(t *testing.T) {
	loader := &lookupAccessLoader{access: authorization.AccountAccess{
		UserLevelIDs:   []int64{callCenterLookupRoleID},
		PermissionMask: int64(authorization.ActionLookup),
	}}
	repository := &lookupRepositoryStub{}
	service := NewCallCenterLookupService(authorization.NewAccessService(loader), repository)
	if _, err := service.Get(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}); !errors.Is(err, ErrLookupOfficeRequired) {
		t.Fatalf("Get() error = %v, want trusted office required", err)
	}
	if repository.typesCalled || repository.destinationsCalled || repository.copiesCalled || repository.servicesCalled || repository.shippingOfficesCalled {
		t.Fatal("lookup repository called without a trusted office")
	}
}

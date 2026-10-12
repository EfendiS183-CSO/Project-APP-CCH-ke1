package customers

import (
	"context"
	"errors"
	"testing"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type profileAccessLoaderStub struct {
	access authorization.AccountAccess
	table  string
}

func (loader *profileAccessLoaderStub) Load(_ context.Context, _ authentication.VerifiedIdentity, table string) (authorization.AccountAccess, error) {
	loader.table = table
	return loader.access, nil
}

type profileRepositoryStub struct {
	called  string
	office  string
	profile Profile
	err     error
}

func (repository *profileRepositoryStub) ListProfiles(_ context.Context, query ProfileListQuery) (ProfilePage, error) {
	repository.called = "list"
	repository.office = query.OfficeCode
	return ProfilePage{Data: []Profile{repository.profile}, Total: 1, Limit: query.Limit, Offset: query.Offset}, repository.err
}
func (repository *profileRepositoryStub) GetProfile(_ context.Context, _ int64, office string) (Profile, error) {
	repository.called = "get"
	repository.office = office
	return repository.profile, repository.err
}
func (repository *profileRepositoryStub) CreateProfile(_ context.Context, profile Profile, office string, actor string) (Profile, error) {
	repository.called = "create"
	repository.office = office
	profile.ID = 13
	return profile, repository.err
}
func (repository *profileRepositoryStub) UpdateProfile(_ context.Context, _ int64, office string, profile Profile, _ string, _ string) (Profile, error) {
	repository.called = "update"
	repository.office = office
	return profile, repository.err
}
func (repository *profileRepositoryStub) DeleteProfile(_ context.Context, _ int64, office string) error {
	repository.called = "delete"
	repository.office = office
	return repository.err
}

func TestProfileServiceUsesMatchingActionAndTrustedOffice(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "office.manager"}
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*ProfileService) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *ProfileService) error {
			_, err := service.List(context.Background(), identity, ProfileListQuery{Limit: 10, OfficeCode: "SPOOF"})
			return err
		}},
		{name: "view", action: authorization.ActionView, call: func(service *ProfileService) error {
			_, err := service.Get(context.Background(), identity, 13)
			return err
		}},
		{name: "add", action: authorization.ActionAdd, call: func(service *ProfileService) error {
			_, err := service.Create(context.Background(), identity, validCreateProfile())
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *ProfileService) error {
			_, err := service.Update(context.Background(), identity, validUpdateProfile())
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *ProfileService) error { return service.Delete(context.Background(), identity, 13) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &profileAccessLoaderStub{access: authorization.AccountAccess{OfficeCode: "40005CC", UserLevelIDs: []int64{29}, PermissionMask: int64(test.action)}}
			repository := &profileRepositoryStub{}
			service := NewProfileService(authorization.NewAccessService(loader), repository)
			if err := test.call(service); err != nil {
				t.Fatalf("authorized action failed: %v", err)
			}
			if loader.table != profileTableName || repository.office != "40005CC" {
				t.Fatalf("table/office = %q/%q", loader.table, repository.office)
			}
		})
	}
}

func TestProfileServiceDeniesMissingActionBeforeRepository(t *testing.T) {
	loader := &profileAccessLoaderStub{access: authorization.AccountAccess{OfficeCode: "40005CC", UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionView)}}
	repository := &profileRepositoryStub{}
	service := NewProfileService(authorization.NewAccessService(loader), repository)
	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, validCreateProfile()); !errors.Is(err, authorization.ErrPermissionDenied) || repository.called != "" {
		t.Fatal("customer create must require Add ACL")
	}
}

func TestNormalizeProfileMatchesLegacyCasingAndFieldLengths(t *testing.T) {
	profile, err := normalizeProfile("  Ada Lovelace ", " ada@example.test ", "+62123", " jalan utama ")
	if err != nil || profile.FullName != "ADA LOVELACE" || profile.Email != "ada@example.test" || profile.Address != "JALAN UTAMA" {
		t.Fatalf("normalized profile = %#v, error = %v", profile, err)
	}
	for _, command := range []CreateProfileCommand{
		{FullName: "", Email: "e", Phone: "1", Address: "a"},
		{FullName: "Name", Email: "", Phone: "1", Address: "a"},
		{FullName: "Name", Email: "e", Phone: "", Address: "a"},
		{FullName: "Name", Email: "e", Phone: "1", Address: ""},
		{FullName: string(make([]byte, 51)), Email: "e", Phone: "1", Address: "a"},
	} {
		if _, err := normalizeProfile(command.FullName, command.Email, command.Phone, command.Address); !errors.Is(err, ErrInvalidProfile) {
			t.Errorf("normalize invalid profile returned %v", err)
		}
	}
}

func validCreateProfile() CreateProfileCommand {
	return CreateProfileCommand{FullName: "New Name", Email: "new@example.test", Phone: "+62123", Address: "Somewhere"}
}
func validUpdateProfile() UpdateProfileCommand {
	return UpdateProfileCommand{ID: 13, FullName: "New Name", Email: "new@example.test", Phone: "+62123", Address: "Somewhere"}
}

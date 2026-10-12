package languages

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type accessLoaderStub struct {
	access authorization.AccountAccess
	table  string
}

func (loader *accessLoaderStub) Load(_ context.Context, _ authentication.VerifiedIdentity, table string) (authorization.AccountAccess, error) {
	loader.table = table
	return loader.access, nil
}

type repositoryStub struct {
	called string
	value  Language
}

func (repository *repositoryStub) List(context.Context) ([]Language, error) {
	repository.called = "list"
	return nil, nil
}

func (repository *repositoryStub) Get(context.Context, string) (Language, error) {
	repository.called = "view"
	return Language{}, nil
}

func (repository *repositoryStub) Insert(_ context.Context, value Language) error {
	repository.called = "add"
	repository.value = value
	return nil
}

func (repository *repositoryStub) Update(_ context.Context, value Language) error {
	repository.called = "edit"
	repository.value = value
	return nil
}

func (repository *repositoryStub) Delete(context.Context, string) error {
	repository.called = "delete"
	return nil
}

func validCreate() CreateCommand {
	return CreateCommand{
		Code: "id", Name: "Indonesia", SiteLogo: "logo.png", SiteTitle: "CCH",
		TermsAndConditionText: "Syarat", AnnouncementText: "Pengumuman", AboutText: "Tentang",
	}
}

func TestServiceAuthorizesEveryActionOnLanguages(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "admin"}
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error { _, err := service.List(context.Background(), identity); return err }},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error { _, err := service.Get(context.Background(), identity, "id"); return err }},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), identity, validCreate())
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), identity, UpdateCommand{Code: "id", Name: "Indonesia", SiteLogo: "logo.png", SiteTitle: "CCH", TermsAndConditionText: "Syarat", AnnouncementText: "Pengumuman", AboutText: "Tentang"})
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error { return service.Delete(context.Background(), identity, "id") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(test.action)}}
			repository := &repositoryStub{}
			service := NewService(authorization.NewAccessService(loader), repository)
			if err := test.call(service); err != nil {
				t.Fatalf("authorized operation failed: %v", err)
			}
			if loader.table != TableVariable || repository.called == "" {
				t.Fatalf("wrong ACL/repository call: %q/%q", loader.table, repository.called)
			}
		})
	}
}

func TestServiceRejectsMissingActionAndInvalidInput(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{1}, PermissionMask: int64(authorization.ActionList)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	identity := authentication.VerifiedIdentity{Username: "viewer"}

	if _, err := service.Create(context.Background(), identity, validCreate()); !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("Create() without Add permission = %v", err)
	}
	if repository.called != "" {
		t.Fatal("unauthorized create reached repository")
	}

	loader.access.PermissionMask = int64(authorization.ActionAdd)
	for _, command := range []CreateCommand{
		{Code: "i", Name: "x", SiteLogo: "l", SiteTitle: "t", TermsAndConditionText: "a", AnnouncementText: "b", AboutText: "c"},
		{Code: "IND", Name: "x", SiteLogo: "l", SiteTitle: "t", TermsAndConditionText: "a", AnnouncementText: "b", AboutText: "c"},
		{Code: "id", Name: "", SiteLogo: "l", SiteTitle: "t", TermsAndConditionText: "a", AnnouncementText: "b", AboutText: "c"},
		{Code: "id", Name: strings.Repeat("x", 21), SiteLogo: "l", SiteTitle: "t", TermsAndConditionText: "a", AnnouncementText: "b", AboutText: "c"},
		{Code: "id", Name: "x", SiteLogo: strings.Repeat("x", 101), SiteTitle: "t", TermsAndConditionText: "a", AnnouncementText: "b", AboutText: "c"},
		{Code: "id", Name: "x", SiteLogo: "l", SiteTitle: "t", TermsAndConditionText: "", AnnouncementText: "b", AboutText: "c"},
	} {
		if _, err := service.Create(context.Background(), identity, command); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Create(%#v) error = %v, want invalid input", command, err)
		}
	}
	if repository.called != "" {
		t.Fatalf("invalid request reached repository: %q", repository.called)
	}
	if _, err := service.Get(context.Background(), identity, "x"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Get(bad code) error = %v, want invalid input", err)
	}
}

func TestServiceDelegatesMutationCommandsToRepository(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionAdd | authorization.ActionEdit | authorization.ActionDelete)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)

	command := validCreate()
	command.Code = " ID "
	command.Name = "  Indonesia  "
	command.IsDefault = true
	created, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "admin"}, command)
	if err != nil {
		t.Fatal(err)
	}
	if created.Code != "ID" || created.Name != "Indonesia" || !created.IsDefault || repository.value.Code != "ID" || !repository.value.IsDefault {
		t.Fatalf("created language = %#v, repository insert = %#v", created, repository.value)
	}

	updated, err := service.Update(context.Background(), authentication.VerifiedIdentity{Username: "admin"}, UpdateCommand{Code: "ID", Name: "Bahasa Indonesia", SiteLogo: "new.png", SiteTitle: "CCH-New", TermsAndConditionText: "Syarat baru", AnnouncementText: "Pengumuman baru", AboutText: "Tentang baru"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Code != "ID" || updated.Name != "Bahasa Indonesia" || repository.value.SiteTitle != "CCH-New" {
		t.Fatalf("updated language = %#v, repository update = %#v", updated, repository.value)
	}

	if err := service.Delete(context.Background(), authentication.VerifiedIdentity{Username: "admin"}, "ID"); err != nil {
		t.Fatal(err)
	}
	if repository.called != "delete" {
		t.Fatalf("repository call = %q, want delete", repository.called)
	}
}

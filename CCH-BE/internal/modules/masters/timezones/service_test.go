package timezones

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
	value  Timezone
}

func (repository *repositoryStub) List(context.Context) ([]Timezone, error) {
	repository.called = "list"
	return nil, nil
}
func (repository *repositoryStub) Get(context.Context, int64) (Timezone, error) {
	repository.called = "view"
	return Timezone{}, nil
}
func (repository *repositoryStub) Insert(_ context.Context, value Timezone) error {
	repository.called = "add"
	repository.value = value
	return nil
}
func (repository *repositoryStub) Update(_ context.Context, value Timezone) error {
	repository.called = "edit"
	repository.value = value
	return nil
}
func (repository *repositoryStub) Delete(context.Context, int64) error {
	repository.called = "delete"
	return nil
}

func TestServiceAuthorizesEveryActionOnTimezones(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "admin"}
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error { _, err := service.List(context.Background(), identity); return err }},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error { _, err := service.Get(context.Background(), identity, 1); return err }},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), identity, CreateCommand{Timezone: "Asia/Jakarta"})
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), identity, UpdateCommand{ID: 1, Timezone: "Asia/Jakarta"})
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error { return service.Delete(context.Background(), identity, 1) }},
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

	if _, err := service.Create(context.Background(), identity, CreateCommand{Timezone: "Asia/Jakarta"}); !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("Create() without Add permission = %v", err)
	}
	if repository.called != "" {
		t.Fatal("unauthorized create reached repository")
	}

	loader.access.PermissionMask = int64(authorization.ActionAdd)
	tooLongTimezone := strings.Repeat("x", 51)
	for _, command := range []CreateCommand{
		{Timezone: ""},
		{Timezone: "   "},
		{Timezone: tooLongTimezone},
	} {
		if _, err := service.Create(context.Background(), identity, command); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Create(%#v) error = %v, want invalid input", command, err)
		}
	}
	if repository.called != "" {
		t.Fatalf("invalid request reached repository: %q", repository.called)
	}
	if _, err := service.Get(context.Background(), identity, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Get(0) error = %v, want invalid input", err)
	}
}

func TestServiceDelegatesMutationCommandsToRepository(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionAdd | authorization.ActionEdit | authorization.ActionDelete)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)

	created, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "admin"}, CreateCommand{Timezone: "  Asia/Jakarta  ", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	if created.Timezone != "Asia/Jakarta" || !created.IsDefault || repository.value.Timezone != "Asia/Jakarta" || !repository.value.IsDefault {
		t.Fatalf("created timezone = %#v, repository insert = %#v", created, repository.value)
	}

	updated, err := service.Update(context.Background(), authentication.VerifiedIdentity{Username: "admin"}, UpdateCommand{ID: 7, Timezone: "Asia/Makassar", IsDefault: false})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != 7 || updated.Timezone != "Asia/Makassar" || updated.IsDefault || repository.value.ID != 7 {
		t.Fatalf("updated timezone = %#v, repository update = %#v", updated, repository.value)
	}

	if err := service.Delete(context.Background(), authentication.VerifiedIdentity{Username: "admin"}, 7); err != nil {
		t.Fatal(err)
	}
	if repository.called != "delete" {
		t.Fatalf("repository call = %q, want delete", repository.called)
	}
}

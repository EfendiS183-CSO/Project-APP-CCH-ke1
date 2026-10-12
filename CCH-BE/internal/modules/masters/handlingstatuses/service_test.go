package handlingstatuses

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

type repositoryStub struct{ called string }

func (repository *repositoryStub) List(context.Context) ([]Status, error) {
	repository.called = "list"
	return nil, nil
}
func (repository *repositoryStub) Get(context.Context, string) (Status, error) {
	repository.called = "view"
	return Status{}, nil
}
func (repository *repositoryStub) Insert(context.Context, Status) error {
	repository.called = "add"
	return nil
}
func (repository *repositoryStub) Update(context.Context, Status) error {
	repository.called = "edit"
	return nil
}
func (repository *repositoryStub) Deactivate(context.Context, string) error {
	repository.called = "delete"
	return nil
}

func TestServiceAuthorizesEveryOperationAgainstStatusTable(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "manager"}
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error { _, err := service.List(context.Background(), identity); return err }},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error { _, err := service.Get(context.Background(), identity, "105"); return err }},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), identity, CreateCommand{Code: "105", Description: "Status baru", Active: true})
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), identity, UpdateCommand{Code: "105", Description: "Status baru", Active: true})
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error { return service.Deactivate(context.Background(), identity, "105") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(test.action)}}
			repository := &repositoryStub{}
			service := NewService(authorization.NewAccessService(loader), repository)
			if err := test.call(service); err != nil {
				t.Fatalf("authorized operation failed: %v", err)
			}
			if loader.table != TableVariable || repository.called == "" {
				t.Fatalf("wrong ACL/repository call: table=%q repository=%q", loader.table, repository.called)
			}
		})
	}
}

func TestServiceDeniesMissingActionBeforeRepositoryAccess(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionView)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, CreateCommand{Code: "105", Description: "Status baru"})
	if !errors.Is(err, authorization.ErrPermissionDenied) || repository.called != "" {
		t.Fatalf("Create without Add permission = %v, repository=%q", err, repository.called)
	}
}

func TestServiceValidatesLegacyCodeAndDescriptionLimits(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionAdd)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	for _, command := range []CreateCommand{
		{Code: "10", Description: "Valid"},
		{Code: "1000", Description: "Valid"},
		{Code: "ééé", Description: "Valid"},
		{Code: "105", Description: ""},
		{Code: "105", Description: "012345678901234567890123456789012345678901234567890"},
		{Code: "105", Description: strings.Repeat("🧪", 5)},
	} {
		if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, command); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Create(%#v) error = %v, want invalid input", command, err)
		}
	}
	if repository.called != "" {
		t.Fatalf("invalid input reached repository: %q", repository.called)
	}
}

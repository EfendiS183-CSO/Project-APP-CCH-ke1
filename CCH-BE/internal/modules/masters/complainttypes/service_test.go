package complainttypes

import (
	"context"
	"errors"
	"testing"
	"time"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type accessLoaderStub struct {
	access authorization.AccountAccess
	err    error
	tables []string
}

func (loader *accessLoaderStub) Load(_ context.Context, _ authentication.VerifiedIdentity, table string) (authorization.AccountAccess, error) {
	loader.tables = append(loader.tables, table)
	return loader.access, loader.err
}

type repositoryStub struct {
	values          []ComplaintType
	value           ComplaintType
	err             error
	listCalls       int
	getCalls        int
	insertCalls     int
	updateCalls     int
	deactivateCalls int
}

func (repository *repositoryStub) List(context.Context) ([]ComplaintType, error) {
	repository.listCalls++
	return repository.values, repository.err
}

func (repository *repositoryStub) Get(context.Context, int64) (ComplaintType, error) {
	repository.getCalls++
	return repository.value, repository.err
}

func (repository *repositoryStub) Insert(_ context.Context, value ComplaintType) error {
	repository.insertCalls++
	repository.value = value
	return repository.err
}

func (repository *repositoryStub) Update(_ context.Context, value ComplaintType) error {
	repository.updateCalls++
	repository.value = value
	return repository.err
}

func (repository *repositoryStub) Deactivate(context.Context, int64, time.Time) error {
	repository.deactivateCalls++
	return repository.err
}

func TestServiceUsesActionSpecificPermissionsForCRUD(t *testing.T) {
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error {
			_, err := service.List(context.Background(), authentication.VerifiedIdentity{Username: "master.admin"})
			return err
		}},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error {
			_, err := service.Get(context.Background(), authentication.VerifiedIdentity{Username: "master.admin"}, 7)
			return err
		}},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "master.admin"}, CreateCommand{Code: 7})
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), authentication.VerifiedIdentity{Username: "master.admin"}, UpdateCommand{Code: 7})
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error {
			return service.Deactivate(context.Background(), authentication.VerifiedIdentity{Username: "master.admin"}, 7)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{
				OfficeCode: "40005CC", UserLevelIDs: []int64{29}, PermissionMask: int64(test.action),
			}}
			repository := &repositoryStub{}
			service := NewService(authorization.NewAccessService(loader), repository)
			if err := test.call(service); err != nil {
				t.Fatalf("CRUD action rejected with its matching permission: %v", err)
			}
			if len(loader.tables) != 1 || loader.tables[0] != TableVariable {
				t.Fatalf("authorized table checks = %#v", loader.tables)
			}
		})
	}
}

func TestServiceRejectsEachCRUDActionWithoutItsPermission(t *testing.T) {
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionView, call: func(service *Service) error {
			_, err := service.List(context.Background(), authentication.VerifiedIdentity{Username: "viewer"})
			return err
		}},
		{name: "view", action: authorization.ActionList, call: func(service *Service) error {
			_, err := service.Get(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, 7)
			return err
		}},
		{name: "add", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, CreateCommand{Code: 7})
			return err
		}},
		{name: "edit", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Update(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, UpdateCommand{Code: 7})
			return err
		}},
		{name: "delete", action: authorization.ActionEdit, call: func(service *Service) error {
			return service.Deactivate(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, 7)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{1}, PermissionMask: int64(test.action)}}
			repository := &repositoryStub{}
			service := NewService(authorization.NewAccessService(loader), repository)
			if err := test.call(service); !errors.Is(err, authorization.ErrPermissionDenied) {
				t.Fatalf("action error = %v, want permission denied", err)
			}
			if repository.listCalls+repository.getCalls+repository.insertCalls+repository.updateCalls+repository.deactivateCalls != 0 {
				t.Fatal("permission denial reached repository")
			}
		})
	}
}

func TestServiceValidatesPrimaryKeyAndDescriptionLength(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{PermissionMask: int64(authorization.ActionAdd)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	tooLong := "123456789012345678901234567890123456789012345678901"
	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "master.admin"}, CreateCommand{Code: 128}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("out-of-range code error = %v, want invalid input", err)
	}
	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "master.admin"}, CreateCommand{Code: 1, Description: &tooLong}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("overlong description error = %v, want invalid input", err)
	}
	if repository.insertCalls != 0 {
		t.Fatal("invalid master input reached repository")
	}
}

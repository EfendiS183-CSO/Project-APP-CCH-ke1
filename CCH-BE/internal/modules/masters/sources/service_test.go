package sources

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
	called string
	value  Source
	values []Source
	err    error
}

func (repository *repositoryStub) List(context.Context) ([]Source, error) {
	repository.called = "list"
	return repository.values, repository.err
}

func (repository *repositoryStub) Get(_ context.Context, id int64) (Source, error) {
	repository.called = "get"
	repository.value.ID = id
	return repository.value, repository.err
}

func (repository *repositoryStub) Insert(_ context.Context, source Source) (Source, error) {
	repository.called = "add"
	repository.value = source
	return source, repository.err
}

func (repository *repositoryStub) Update(_ context.Context, source Source) error {
	repository.called = "edit"
	repository.value = source
	return repository.err
}

func (repository *repositoryStub) Deactivate(context.Context, int64, time.Time) error {
	repository.called = "delete"
	return repository.err
}

func TestServiceRequiresMatchingCCHActionForEachCRUDOperation(t *testing.T) {
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error {
			_, err := service.List(context.Background(), authentication.VerifiedIdentity{Username: "manager"})
			return err
		}},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error {
			_, err := service.Get(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, 4)
			return err
		}},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, CreateCommand{Name: "Web"})
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, UpdateCommand{ID: 4, Name: "Web"})
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error {
			return service.Deactivate(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, 4)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(test.action)}}
			repository := &repositoryStub{}
			service := NewService(authorization.NewAccessService(loader), repository)
			if err := test.call(service); err != nil {
				t.Fatalf("operation with matching permission failed: %v", err)
			}
			if repository.called == "" || len(loader.tables) != 1 || loader.tables[0] != TableVariable {
				t.Fatalf("operation was not scoped to master ACL: repository=%q table=%v", repository.called, loader.tables)
			}
		})
	}
}

func TestServiceRejectsMissingCRUDActionAndInvalidSourceName(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{1}, PermissionMask: int64(authorization.ActionList)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, CreateCommand{Name: "Web"})
	if !errors.Is(err, authorization.ErrPermissionDenied) || repository.called != "" {
		t.Fatalf("Create() without Add permission = %v, repository call=%q", err, repository.called)
	}
	loader.access.PermissionMask = int64(authorization.ActionAdd)
	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, CreateCommand{Name: "  "}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank source name error = %v, want invalid input", err)
	}
	if repository.called != "" {
		t.Fatal("invalid source name reached repository")
	}
}

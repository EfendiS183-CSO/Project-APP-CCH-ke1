package helpcategories

import (
	"context"
	"errors"
	"testing"

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

type repositoryStub struct{ called string }

func (repository *repositoryStub) List(context.Context) ([]Category, error) {
	repository.called = "list"
	return nil, nil
}
func (repository *repositoryStub) Get(context.Context, int64) (Category, error) {
	repository.called = "view"
	return Category{}, nil
}
func (repository *repositoryStub) Languages(context.Context) ([]Language, error) {
	repository.called = "lookup"
	return nil, nil
}
func (repository *repositoryStub) Insert(_ context.Context, value Category) (Category, error) {
	repository.called = "add"
	return value, nil
}
func (repository *repositoryStub) Update(context.Context, Category) error {
	repository.called = "edit"
	return nil
}
func (repository *repositoryStub) Delete(context.Context, int64) error {
	repository.called = "delete"
	return nil
}

func TestServiceRequiresMatchingActionForEachHelpCategoryOperation(t *testing.T) {
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
		{name: "lookup", action: authorization.ActionLookup, call: func(service *Service) error {
			_, err := service.Languages(context.Background(), authentication.VerifiedIdentity{Username: "manager"})
			return err
		}},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, CreateCommand{Language: "id", Description: "Login"})
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, UpdateCommand{ID: 4, Language: "id", Description: "Login"})
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error {
			return service.Delete(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, 4)
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
				t.Fatalf("operation did not use matching master ACL: repository=%q tables=%v", repository.called, loader.tables)
			}
		})
	}
}

func TestServiceDeniesMissingActionAndRejectsInvalidCategory(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{1}, PermissionMask: int64(authorization.ActionList)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, CreateCommand{Language: "id", Description: "Masuk"}); !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("Create without Add permission error = %v", err)
	}
	loader.access.PermissionMask = int64(authorization.ActionAdd)
	for _, command := range []CreateCommand{
		{Language: "i", Description: "Masuk"},
		{Language: "id", Description: " "},
		{Language: "id", Description: string(make([]rune, 101))},
	} {
		if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, command); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Create(%#v) error = %v, want invalid input", command, err)
		}
	}
	if repository.called != "" {
		t.Fatalf("invalid or denied operations reached repository: %q", repository.called)
	}
}

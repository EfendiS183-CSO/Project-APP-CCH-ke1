package offices

import (
	"context"
	"errors"
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

func (repository *repositoryStub) List(context.Context) ([]Office, error) {
	repository.called = "list"
	return nil, nil
}
func (repository *repositoryStub) Get(context.Context, string) (Office, error) {
	repository.called = "view"
	return Office{}, nil
}
func (repository *repositoryStub) LookupOptions(context.Context) (LookupOptions, error) {
	repository.called = "lookup"
	return LookupOptions{}, nil
}
func (repository *repositoryStub) Insert(context.Context, Office) error {
	repository.called = "add"
	return nil
}
func (repository *repositoryStub) Update(context.Context, Office) error {
	repository.called = "edit"
	return nil
}
func (repository *repositoryStub) Deactivate(context.Context, string) error {
	repository.called = "delete"
	return nil
}

func TestServiceAuthorizesEachOfficeOperationAgainstMatchingAction(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "office.manager"}
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error { _, err := service.List(context.Background(), identity); return err }},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error {
			_, err := service.Get(context.Background(), identity, "40005CC")
			return err
		}},
		{name: "lookup", action: authorization.ActionLookup, call: func(service *Service) error {
			_, err := service.LookupOptions(context.Background(), identity)
			return err
		}},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), identity, validCreateCommand())
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), identity, validUpdateCommand())
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error { return service.Deactivate(context.Background(), identity, "40005CC") }},
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

func TestServiceRejectsMissingActionAndInvalidSchemaValuesBeforeRepository(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionView)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, validCreateCommand()); !errors.Is(err, authorization.ErrPermissionDenied) || repository.called != "" {
		t.Fatal("create should require Add before repository work")
	}
	loader.access.PermissionMask = int64(authorization.ActionAdd)
	invalid := validCreateCommand()
	invalid.Name = "office 🧪"
	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, invalid); !errors.Is(err, ErrInvalidInput) || repository.called != "" {
		t.Fatalf("unrepresentable latin1 text should be rejected: %v", err)
	}
}

func validCreateCommand() CreateCommand {
	return CreateCommand{Code: "40006CC", Name: "Kantor Baru", Abbreviation: "KB", Type: "UPT", RegionalCode: "40005", Active: true, AsDestination: true, AsCopy: true}
}
func validUpdateCommand() UpdateCommand {
	return UpdateCommand{Code: "40005CC", Name: "Kantor Lama", Abbreviation: "KL", Type: "UPT", RegionalCode: "40005", Active: true, AsDestination: true, AsCopy: true}
}

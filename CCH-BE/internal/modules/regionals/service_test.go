package regionals

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
	tables []string
}

func (loader *accessLoaderStub) Load(_ context.Context, _ authentication.VerifiedIdentity, table string) (authorization.AccountAccess, error) {
	loader.tables = append(loader.tables, table)
	return loader.access, nil
}

type repositoryStub struct {
	called string
	page   Page
	office Office
	update Office
}

func (repository *repositoryStub) List(_ context.Context, query ListQuery) (Page, error) {
	repository.called = "list"
	repository.page.Limit = query.Limit
	repository.page.Offset = query.Offset
	return repository.page, nil
}
func (repository *repositoryStub) Get(_ context.Context, _ string) (Office, error) {
	repository.called = "get"
	return repository.office, nil
}
func (repository *repositoryStub) LookupOptions(context.Context) ([]OfficeOption, error) {
	repository.called = "lookup"
	return []OfficeOption{{Code: "40005", Name: "Kantor Pusat", Type: "KP"}}, nil
}
func (repository *repositoryStub) Update(_ context.Context, value Office) error {
	repository.called = "update"
	repository.update = value
	return nil
}

func TestServiceRequiresMatchingACLForRegionalWorkflow(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "regional.admin"}
	for _, test := range []struct {
		name   string
		table  string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", table: TableVariable, action: authorization.ActionList, call: func(service *Service) error {
			_, err := service.List(context.Background(), identity, ListQuery{})
			return err
		}},
		{name: "view", table: TableVariable, action: authorization.ActionView, call: func(service *Service) error {
			_, err := service.Get(context.Background(), identity, "40005")
			return err
		}},
		{name: "lookup parent offices", table: OfficeTableVariable, action: authorization.ActionLookup, call: func(service *Service) error {
			_, err := service.LookupOptions(context.Background(), identity)
			return err
		}},
		{name: "edit", table: TableVariable, action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), identity, UpdateCommand{Code: "40005", Name: "Kantor", Abbreviation: "KP", Type: "KP", RegionalCode: "40005", Active: true})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(test.action)}}
			repository := &repositoryStub{}
			service := NewService(authorization.NewAccessService(loader), repository)
			if err := test.call(service); err != nil {
				t.Fatalf("authorized operation failed: %v", err)
			}
			if len(loader.tables) != 1 || loader.tables[0] != test.table || repository.called == "" {
				t.Fatalf("wrong authorization/repository call: tables=%v repo=%q", loader.tables, repository.called)
			}
		})
	}
}

func TestServiceRejectsMissingPermissionAndInvalidRegionalInput(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionList)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	identity := authentication.VerifiedIdentity{Username: "viewer"}
	if _, err := service.Update(context.Background(), identity, UpdateCommand{Code: "40005", Name: "Pusat", Abbreviation: "KP", Type: "KP", RegionalCode: "40005"}); !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("Update without Edit permission = %v", err)
	}
	if repository.called != "" {
		t.Fatal("unauthorized update reached repository")
	}
	loader.access.PermissionMask = int64(authorization.ActionEdit)
	badCommands := []UpdateCommand{
		{Code: "40005", Name: "", Abbreviation: "KP", Type: "KP", RegionalCode: "40005"},
		{Code: "40005", Name: "Pusat", Abbreviation: strings.Repeat("K", 11), Type: "KP", RegionalCode: "40005"},
		{Code: "40005", Name: "Pusat", Abbreviation: "KP", Type: strings.Repeat("K", 11), RegionalCode: "40005"},
		{Code: "40005", Name: "Pusat", Abbreviation: "KP", Type: "KP", RegionalCode: "400056"},
		{Code: "40005", Name: "Pusat\U0001F642", Abbreviation: "KP", Type: "KP", RegionalCode: "40005"},
	}
	for _, command := range badCommands {
		if _, err := service.Update(context.Background(), identity, command); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Update(%#v) error = %v, want invalid input", command, err)
		}
	}
	if repository.called != "" {
		t.Fatalf("invalid regional update reached repository: %q", repository.called)
	}
}

func TestListQueryBoundsPaginationAndUpdatePreservesViewKey(t *testing.T) {
	for _, query := range []ListQuery{{Limit: -1}, {Limit: MaximumPageSize + 1}, {Offset: -1}} {
		if _, err := query.Normalize(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Normalize(%#v) error = %v, want invalid input", query, err)
		}
	}
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionEdit)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	value, err := service.Update(context.Background(), authentication.VerifiedIdentity{Username: "admin"}, UpdateCommand{
		Code: "40005", Name: "Kantor Pusat", Abbreviation: "KP", Type: "KP", RegionalCode: "40005", Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.Code != "40005" || repository.update.Code != "40005" || repository.called != "update" {
		t.Fatalf("regional key was not preserved: result=%#v repository=%#v call=%s", value, repository.update, repository.called)
	}
}

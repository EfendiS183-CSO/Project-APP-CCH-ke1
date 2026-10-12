package csofficers

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
	query  ListQuery
}

func (repository *repositoryStub) List(_ context.Context, query ListQuery) (Page, error) {
	repository.called = "list"
	repository.query = query
	return Page{}, nil
}
func (repository *repositoryStub) Get(context.Context, string) (Officer, error) {
	repository.called = "get"
	return Officer{}, nil
}

func TestServiceAuthorizesCSOfficerListAndViewSeparately(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "regional.admin"}
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error {
			_, err := service.List(context.Background(), identity, ListQuery{})
			return err
		}},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error {
			_, err := service.Get(context.Background(), identity, "cs.user")
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{10}, PermissionMask: int64(test.action)}}
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

func TestServiceRejectsMissingActionAndInvalidFilters(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{10}, PermissionMask: int64(authorization.ActionView)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	identity := authentication.VerifiedIdentity{Username: "viewer"}
	if _, err := service.List(context.Background(), identity, ListQuery{}); !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("List without List ACL = %v", err)
	}
	if repository.called != "" {
		t.Fatal("unauthorized list reached repository")
	}

	loader.access.PermissionMask = int64(authorization.ActionList)
	for _, query := range []ListQuery{
		{Limit: MaximumPageSize + 1}, {Limit: -1}, {Offset: -1},
		{Status: "Away"}, {Office: strings.Repeat("O", 8)},
		{Regional: strings.Repeat("R", 51)}, {Search: strings.Repeat("x", 51)},
	} {
		if _, err := service.List(context.Background(), identity, query); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("List(%#v) error = %v, want invalid input", query, err)
		}
	}
	if repository.called != "" {
		t.Fatalf("invalid query reached repository: %q", repository.called)
	}
	if _, err := service.Get(context.Background(), identity, ""); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Get(empty username) error = %v, want invalid input", err)
	}
}

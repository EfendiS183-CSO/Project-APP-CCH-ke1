package help

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

type repositoryStub struct {
	called  string
	actor   string
	query   ListQuery
	article Article
}

func (repository *repositoryStub) List(_ context.Context, query ListQuery) (Page, error) {
	repository.called = "list"
	repository.query = query
	return Page{}, nil
}
func (repository *repositoryStub) Get(context.Context, int64) (Article, error) {
	repository.called = "view"
	return Article{}, nil
}
func (repository *repositoryStub) LookupOptions(context.Context) (LookupOptions, error) {
	repository.called = "lookup"
	return LookupOptions{}, nil
}
func (repository *repositoryStub) Insert(_ context.Context, article Article, actor string) (Article, error) {
	repository.called = "add"
	repository.actor = actor
	repository.article = article
	return article, nil
}
func (repository *repositoryStub) Update(_ context.Context, article Article, actor string) (Article, error) {
	repository.called = "edit"
	repository.actor = actor
	repository.article = article
	return article, nil
}
func (repository *repositoryStub) Delete(context.Context, int64) error {
	repository.called = "delete"
	return nil
}

func TestServiceRequiresMatchingActionForEachHelpArticleOperation(t *testing.T) {
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error {
			_, err := service.List(context.Background(), testIdentity(), ListQuery{})
			return err
		}},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error {
			_, err := service.Get(context.Background(), testIdentity(), 8)
			return err
		}},
		{name: "lookup", action: authorization.ActionLookup, call: func(service *Service) error {
			_, err := service.LookupOptions(context.Background(), testIdentity())
			return err
		}},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), testIdentity(), validCreateCommand())
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), testIdentity(), UpdateCommand{ID: 8, CreateCommand: validCreateCommand()})
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error {
			return service.Delete(context.Background(), testIdentity(), 8)
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
				t.Fatalf("operation did not use matching CCH table ACL: repository=%q tables=%v", repository.called, loader.tables)
			}
		})
	}
}

func TestServiceUsesVerifiedActorAndRejectsInvalidArticleInput(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionAdd)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	article, err := service.Create(context.Background(), testIdentity(), validCreateCommand())
	if err != nil {
		t.Fatal(err)
	}
	if repository.actor != "help.manager" || article.Description != validCreateCommand().Description {
		t.Fatalf("insert actor/article = %q/%#v", repository.actor, article)
	}
	repository.called = ""
	for _, command := range []CreateCommand{
		{Language: "id", Topic: "", Description: "body", CategoryID: 1, Order: 1, DisplayInPage: "home"},
		{Language: "id", Topic: "topic", Description: " ", CategoryID: 1, Order: 1, DisplayInPage: "home"},
		{Language: "id", Topic: "topic", Description: "body", CategoryID: 0, Order: 1, DisplayInPage: "home"},
		{Language: "id", Topic: "topic", Description: "body", CategoryID: 1, Order: 1, DisplayInPage: ""},
	} {
		if _, err := service.Create(context.Background(), testIdentity(), command); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Create(%#v) error = %v, want invalid input", command, err)
		}
	}
	if repository.called != "" {
		t.Fatalf("invalid requests should not reach repository, last call=%q", repository.called)
	}
}

func TestServiceDeniesMissingActionAndBoundsListQuery(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionList)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	if _, err := service.Create(context.Background(), testIdentity(), validCreateCommand()); !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("Create without Add permission error = %v", err)
	}
	page, err := service.List(context.Background(), testIdentity(), ListQuery{Language: " id ", Limit: 0})
	if err != nil {
		t.Fatal(err)
	}
	if page.Limit != 0 || repository.query.Language != "id" || repository.query.Limit != defaultPageSize {
		t.Fatalf("normalized list query = %#v", repository.query)
	}
	if _, err := service.List(context.Background(), testIdentity(), ListQuery{Limit: maximumPageSize + 1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oversized list limit error = %v", err)
	}
}

func testIdentity() authentication.VerifiedIdentity {
	return authentication.VerifiedIdentity{Username: "help.manager"}
}

func validCreateCommand() CreateCommand {
	return CreateCommand{Language: "id", Topic: "Login", Description: "Help text", CategoryID: 3, Order: 1, DisplayInPage: "login"}
}

package announcements

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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
	called       string
	language     string
	translated   int64
	exists       bool
	count        int64
	overlaps     bool
	page         Page
	options      Options
	announcement Announcement
	err          error
}

func (repository *repositoryStub) List(_ context.Context, query ListQuery) (Page, error) {
	repository.called = "list"
	repository.language = query.Language
	return repository.page, repository.err
}
func (repository *repositoryStub) Options(context.Context) (Options, error) {
	repository.called = "options"
	return repository.options, repository.err
}
func (repository *repositoryStub) LanguageExists(_ context.Context, language string) (bool, error) {
	repository.called = "language_exists"
	repository.language = language
	return repository.exists, repository.err
}
func (repository *repositoryStub) CountByLanguage(_ context.Context, language string, translatedID int64) (int64, error) {
	repository.called = "count_language"
	repository.language = language
	repository.translated = translatedID
	return repository.count, repository.err
}
func (repository *repositoryStub) Get(_ context.Context, _ int64) (Announcement, error) {
	repository.called = "get"
	return repository.announcement, repository.err
}
func (repository *repositoryStub) FindOverlap(_ context.Context, language string, _, _ time.Time, _ int64) (bool, error) {
	repository.called = "find_overlap"
	repository.language = language
	return repository.overlaps, repository.err
}
func (repository *repositoryStub) Create(_ context.Context, value Announcement, actor string) (Announcement, error) {
	repository.called = "create"
	repository.announcement = value
	if repository.err == nil {
		repository.announcement.CreatedBy = &actor
	}
	return repository.announcement, repository.err
}
func (repository *repositoryStub) Update(_ context.Context, value Announcement, _ string) (Announcement, error) {
	repository.called = "update"
	return value, repository.err
}
func (repository *repositoryStub) Delete(_ context.Context, _ int64) error {
	repository.called = "delete"
	return repository.err
}

func TestServiceAuthorizesAndDelegatesEachAction(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "manager"}
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error {
			_, err := service.List(context.Background(), identity, ListQuery{})
			return err
		}},
		{name: "lookup", action: authorization.ActionLookup, call: func(service *Service) error { _, err := service.Options(context.Background(), identity); return err }},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error { _, err := service.Get(context.Background(), identity, 12); return err }},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), identity, validCreateCommand())
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), identity, validUpdateCommand())
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error { return service.Delete(context.Background(), identity, 12) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(test.action)}}
			repository := &repositoryStub{exists: true, count: 0}
			service := NewService(authorization.NewAccessService(loader), repository)
			if err := test.call(service); err != nil {
				t.Fatalf("authorized action failed: %v", err)
			}
			if loader.table != TableVariable || repository.called == "" {
				t.Fatalf("table/call = %q/%q", loader.table, repository.called)
			}
		})
	}
}

func TestServiceValidatesLanguageAndOverlapOnCreate(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionAdd)}}
	repository := &repositoryStub{exists: false}
	service := NewService(authorization.NewAccessService(loader), repository)
	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, validCreateCommand()); !errors.Is(err, ErrInvalidLanguage) {
		t.Fatalf("invalid language error = %v", err)
	}
	repository.exists = true
	repository.overlaps = true
	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, validCreateCommand()); !errors.Is(err, ErrDateOverlaps) {
		t.Fatalf("overlap error = %v", err)
	}
}

func TestServiceRejectsInvalidAnnouncementInput(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionAdd)}}
	repository := &repositoryStub{exists: true}
	service := NewService(authorization.NewAccessService(loader), repository)
	for _, command := range []CreateCommand{
		{Topic: "", Message: "x", Language: "en"},
		{Topic: "Topic", Message: "", Language: "en"},
		{Topic: strings.Repeat("x", 51), Message: "x", Language: "en"},
		{Topic: "Topic", Message: "x", Language: "english"},
	} {
		if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, command); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Create(%#v) error = %v, want invalid input", command, err)
		}
	}
}

func validCreateCommand() CreateCommand {
	start, end := time.Now(), time.Now().Add(24*time.Hour)
	return CreateCommand{Topic: "Deadline", Message: "Submit today", Language: "en", DateStart: &start, DateEnd: &end}
}

func validUpdateCommand() UpdateCommand {
	start, end := time.Now(), time.Now().Add(24*time.Hour)
	return UpdateCommand{ID: 12, Topic: "Deadline", Message: "Submit today", Language: "en", DateStart: &start, DateEnd: &end}
}

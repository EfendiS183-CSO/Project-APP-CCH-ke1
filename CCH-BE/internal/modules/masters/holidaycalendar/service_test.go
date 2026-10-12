package holidaycalendar

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
	called string
	value  Holiday
}

func (repository *repositoryStub) List(context.Context) ([]Holiday, error) {
	repository.called = "list"
	return nil, nil
}
func (repository *repositoryStub) Get(context.Context, string) (Holiday, error) {
	repository.called = "view"
	return Holiday{}, nil
}
func (repository *repositoryStub) Insert(_ context.Context, value Holiday) error {
	repository.called = "add"
	repository.value = value
	return nil
}
func (repository *repositoryStub) Update(_ context.Context, value Holiday) error {
	repository.called = "edit"
	repository.value = value
	return nil
}
func (repository *repositoryStub) Delete(context.Context, string) error {
	repository.called = "delete"
	return nil
}

func fixedNow() time.Time { return time.Date(2026, time.October, 5, 3, 0, 0, 0, time.UTC) }

func TestServiceAuthorizesEveryActionOnHolidayCalendar(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "manager"}
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error { _, err := service.List(context.Background(), identity); return err }},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error {
			_, err := service.Get(context.Background(), identity, "2026-08-17")
			return err
		}},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), identity, CreateCommand{Date: "2026-08-17", Description: "Kemerdekaan"})
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), identity, UpdateCommand{Date: "2026-08-17", Description: "HUT RI"})
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error { return service.Delete(context.Background(), identity, "2026-08-17") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(test.action)}}
			repository := &repositoryStub{}
			service := NewService(authorization.NewAccessService(loader), repository)
			service.now = fixedNow
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
	service.now = fixedNow
	identity := authentication.VerifiedIdentity{Username: "viewer"}

	if _, err := service.Create(context.Background(), identity, CreateCommand{Date: "2026-01-01", Description: "Tahun Baru"}); !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("Create() without Add permission = %v", err)
	}
	if repository.called != "" {
		t.Fatal("unauthorized create reached repository")
	}

	loader.access.PermissionMask = int64(authorization.ActionAdd)
	tooLongDescription := strings.Repeat("x", 51)
	tooLongActor := strings.Repeat("y", 21)
	for _, command := range []CreateCommand{
		{Date: "17-08-2026", Description: "Kemerdekaan"},
		{Date: "2026-02-30", Description: "Tanggal tidak valid"},
		{Date: "", Description: "Kosong"},
		{Date: "2026-08-17", Description: "   "},
		{Date: "2026-08-17", Description: tooLongDescription},
	} {
		if _, err := service.Create(context.Background(), identity, command); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Create(%#v) error = %v, want invalid input", command, err)
		}
	}
	if repository.called != "" {
		t.Fatalf("invalid request reached repository: %q", repository.called)
	}

	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: tooLongActor}, CreateCommand{Date: "2026-08-17", Description: "Kemerdekaan"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unrepresentable actor error = %v, want invalid input", err)
	}
}

func TestServiceStampsCreatedAuditAndKeepsDateKey(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionAdd | authorization.ActionEdit)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	service.now = fixedNow

	created, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, CreateCommand{Date: " 2026-08-17 ", Description: "  Kemerdekaan  "})
	if err != nil {
		t.Fatal(err)
	}
	if created.Date != "2026-08-17" || created.Description != "Kemerdekaan" || created.CreatedBy != "manager" || created.CreatedAt == nil || !created.CreatedAt.Equal(fixedNow()) {
		t.Fatalf("created holiday = %#v", created)
	}
	if repository.value.Date != "2026-08-17" || repository.value.CreatedBy != "manager" {
		t.Fatalf("repository insert = %#v", repository.value)
	}

	if _, err := service.Update(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, UpdateCommand{Date: "2026-08-17", Description: "HUT RI"}); err != nil {
		t.Fatal(err)
	}
	if repository.value.Date != "2026-08-17" || repository.value.Description != "HUT RI" || repository.value.CreatedAt != nil {
		t.Fatalf("repository update = %#v, want description-only change without rewriting created audit", repository.value)
	}
}

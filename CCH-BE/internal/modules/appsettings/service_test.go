package appsettings

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
	table  string
}

func (loader *accessLoaderStub) Load(_ context.Context, _ authentication.VerifiedIdentity, table string) (authorization.AccountAccess, error) {
	loader.table = table
	return loader.access, nil
}

type repositoryStub struct {
	called string
	value  Settings
}

func (repository *repositoryStub) List(context.Context) ([]Settings, error) {
	repository.called = "list"
	return nil, nil
}
func (repository *repositoryStub) Get(context.Context, int64) (Settings, error) {
	repository.called = "view"
	return Settings{}, nil
}
func (repository *repositoryStub) Insert(_ context.Context, value Settings) error {
	repository.called = "add"
	repository.value = value
	return nil
}
func (repository *repositoryStub) Update(_ context.Context, value Settings) error {
	repository.called = "edit"
	repository.value = value
	return nil
}
func (repository *repositoryStub) Delete(context.Context, int64) error {
	repository.called = "delete"
	return nil
}

func maintenanceFinish() *time.Time {
	value := time.Date(2026, time.October, 20, 12, 30, 0, 0, time.UTC)
	return &value
}

func TestServiceAuthorizesEveryActionOnSettings(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "admin"}
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error { _, err := service.List(context.Background(), identity); return err }},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error { _, err := service.Get(context.Background(), identity, 1); return err }},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), identity, CreateCommand{MaintenanceMode: true, MaintenanceFinishDateTime: maintenanceFinish()})
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), identity, UpdateCommand{OptionID: 1, MaintenanceMode: true, MaintenanceFinishDateTime: maintenanceFinish()})
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
	identity := authentication.VerifiedIdentity{Username: "viewer"}

	if _, err := service.Create(context.Background(), identity, CreateCommand{}); !errors.Is(err, authorization.ErrPermissionDenied) {
		t.Fatalf("Create() without Add permission = %v", err)
	}
	if repository.called != "" {
		t.Fatal("unauthorized create reached repository")
	}

	loader.access.PermissionMask = int64(authorization.ActionAdd)

	// maintenance mode on requires finish datetime
	if _, err := service.Create(context.Background(), identity, CreateCommand{MaintenanceMode: true}); !errors.Is(err, ErrCannotSetMaintenanceNow) {
		t.Fatalf("maintenance without finish error = %v, want cannot set maintenance now", err)
	}
	// invalid id on update
	if _, err := service.Update(context.Background(), identity, UpdateCommand{OptionID: 0}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update(0) error = %v, want invalid input", err)
	}
	if repository.called != "" {
		t.Fatalf("invalid request reached repository: %q", repository.called)
	}
}

func TestServiceDeleteIsAlwaysRestricted(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionDelete)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	err := service.Delete(context.Background(), authentication.VerifiedIdentity{Username: "admin"}, 1)
	if err != ErrDeleteRestricted {
		t.Fatalf("Delete() error = %v, want delete restricted", err)
	}
	if repository.called != "" {
		t.Fatal("restricted delete reached repository")
	}
}

func TestServiceDelegatesMutationCommandsToRepository(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionAdd | authorization.ActionEdit)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)

	created, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "admin"}, CreateCommand{
		ShowAnnouncement: true, UseAnnouncementTable: true, MaintenanceMode: true,
		MaintenanceFinishDateTime: maintenanceFinish(), AutoNormalAfterMaintenance: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.ShowAnnouncement || !created.UseAnnouncementTable || !created.MaintenanceMode || created.MaintenanceFinishDateTime == nil || !created.AutoNormalAfterMaintenance || repository.value.OptionID != 0 {
		t.Fatalf("created settings = %#v, repository insert = %#v", created, repository.value)
	}

	updated, err := service.Update(context.Background(), authentication.VerifiedIdentity{Username: "admin"}, UpdateCommand{
		OptionID: 1, ShowAnnouncement: false, UseAnnouncementTable: false, MaintenanceMode: false,
		AutoNormalAfterMaintenance: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.OptionID != 1 || updated.ShowAnnouncement || repository.value.OptionID != 1 {
		t.Fatalf("updated settings = %#v, repository update = %#v", updated, repository.value)
	}
}

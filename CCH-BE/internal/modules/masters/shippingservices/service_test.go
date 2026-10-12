package shippingservices

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
	loaded string
}

func (loader *accessLoaderStub) Load(_ context.Context, _ authentication.VerifiedIdentity, table string) (authorization.AccountAccess, error) {
	loader.loaded = table
	return loader.access, nil
}

type repositoryStub struct {
	called string
	err    error
}

func (repository *repositoryStub) List(context.Context) ([]ShippingService, error) {
	repository.called = "list"
	return nil, repository.err
}

func (repository *repositoryStub) Get(context.Context, string) (ShippingService, error) {
	repository.called = "get"
	return ShippingService{}, repository.err
}

func (repository *repositoryStub) Insert(context.Context, ShippingService) error {
	repository.called = "add"
	return repository.err
}

func (repository *repositoryStub) Update(context.Context, ShippingService) error {
	repository.called = "edit"
	return repository.err
}

func (repository *repositoryStub) Deactivate(context.Context, string, time.Time) error {
	repository.called = "delete"
	return repository.err
}

func TestServiceUsesMatchingCCHActionForEveryOperation(t *testing.T) {
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
			_, err := service.Get(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, "REG")
			return err
		}},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, CreateCommand{Code: "REG", TrackingURL: "https://track.example/{item}"})
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, UpdateCommand{Code: "REG", TrackingURL: "https://track.example/{item}"})
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error {
			return service.Deactivate(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, "REG")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(test.action)}}
			repository := &repositoryStub{}
			service := NewService(authorization.NewAccessService(loader), repository)
			if err := test.call(service); err != nil {
				t.Fatalf("operation with matching ACL failed: %v", err)
			}
			if loader.loaded != TableVariable || repository.called == "" {
				t.Fatalf("operation did not authorize its table: loader=%q repository=%q", loader.loaded, repository.called)
			}
		})
	}
}

func TestServiceRejectsMissingActionAndInvalidTrackingURL(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{PermissionMask: int64(authorization.ActionView)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, CreateCommand{Code: "REG", TrackingURL: "https://track.example/{item}"})
	if !errors.Is(err, authorization.ErrPermissionDenied) || repository.called != "" {
		t.Fatalf("Create() without Add permission = %v, repository=%q", err, repository.called)
	}
	loader.access.PermissionMask = int64(authorization.ActionAdd)
	_, err = service.Create(context.Background(), authentication.VerifiedIdentity{Username: "manager"}, CreateCommand{Code: "REG"})
	if !errors.Is(err, ErrInvalidInput) || repository.called != "" {
		t.Fatalf("Create() without tracking URL = %v, repository=%q", err, repository.called)
	}
}

package documenttypes

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
	value  DocumentType
	err    error
}

func (repository *repositoryStub) List(context.Context) ([]DocumentType, error) {
	repository.called = "list"
	return []DocumentType{repository.value}, repository.err
}
func (repository *repositoryStub) Get(context.Context, string) (DocumentType, error) {
	repository.called = "get"
	return repository.value, repository.err
}
func (repository *repositoryStub) Insert(_ context.Context, value DocumentType) error {
	repository.called, repository.value = "add", value
	return repository.err
}
func (repository *repositoryStub) Update(_ context.Context, value DocumentType) error {
	repository.called, repository.value = "edit", value
	return repository.err
}
func (repository *repositoryStub) Deactivate(context.Context, string, string, time.Time) error {
	repository.called = "delete"
	return repository.err
}

func TestServiceRequiresMatchingActionForEachOperation(t *testing.T) {
	identity := authentication.VerifiedIdentity{Username: "manager"}
	for _, test := range []struct {
		name   string
		action authorization.Action
		call   func(*Service) error
	}{
		{name: "list", action: authorization.ActionList, call: func(service *Service) error { _, err := service.List(context.Background(), identity); return err }},
		{name: "view", action: authorization.ActionView, call: func(service *Service) error { _, err := service.Get(context.Background(), identity, "PDF"); return err }},
		{name: "add", action: authorization.ActionAdd, call: func(service *Service) error {
			_, err := service.Create(context.Background(), identity, CreateCommand{Code: "PDF", Description: "Dokumen"})
			return err
		}},
		{name: "edit", action: authorization.ActionEdit, call: func(service *Service) error {
			_, err := service.Update(context.Background(), identity, UpdateCommand{Code: "PDF", Description: "Dokumen"})
			return err
		}},
		{name: "delete", action: authorization.ActionDelete, call: func(service *Service) error { return service.Deactivate(context.Background(), identity, "PDF") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(test.action)}}
			repository := &repositoryStub{}
			service := NewService(authorization.NewAccessService(loader), repository)
			if err := test.call(service); err != nil {
				t.Fatalf("authorized operation failed: %v", err)
			}
			if loader.table != TableVariable || repository.called == "" {
				t.Fatalf("operation did not use table ACL: table=%q repository=%q", loader.table, repository.called)
			}
		})
	}
}

func TestServiceDeniesMissingActionBeforeRepositoryAccess(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionView)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "viewer"}, CreateCommand{Code: "PDF", Description: "Dokumen"})
	if !errors.Is(err, authorization.ErrPermissionDenied) || repository.called != "" {
		t.Fatalf("Create without Add permission = %v, repository=%q", err, repository.called)
	}
}

func TestServiceValidatesLegacyFieldSizesAndAuditsActor(t *testing.T) {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(authorization.ActionAdd)}}
	repository := &repositoryStub{}
	service := NewService(authorization.NewAccessService(loader), repository)
	createdAt := time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return createdAt }
	identity := authentication.VerifiedIdentity{Username: "document.admin"}
	value, err := service.Create(context.Background(), identity, CreateCommand{Code: " PDF ", Description: "  Dokumen  ", Active: true})
	if err != nil || value.Code != "PDF" || value.Description != "Dokumen" || value.CreatedBy != identity.Username || value.UpdatedBy != identity.Username || value.CreatedAt == nil || !value.CreatedAt.Equal(createdAt) {
		t.Fatalf("Create() = %#v, error = %v", value, err)
	}
	for _, command := range []CreateCommand{{Code: "PD", Description: "Dokumen"}, {Code: "PDFX", Description: "Dokumen"}, {Code: "PDF", Description: ""}, {Code: "PDF", Description: string(make([]byte, 101))}} {
		if _, err := service.Create(context.Background(), identity, command); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("Create(%#v) error = %v, want invalid input", command, err)
		}
	}
	if _, err := service.Create(context.Background(), authentication.VerifiedIdentity{Username: "this-username-is-too-long"}, CreateCommand{Code: "PDF", Description: "Dokumen"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("Create with oversized audit actor error = %v, want invalid input", err)
	}
}

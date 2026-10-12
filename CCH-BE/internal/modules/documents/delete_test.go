package documents

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type deleteAccessLoader struct {
	accessByTable map[string]authorization.AccountAccess
}

func (loader *deleteAccessLoader) Load(_ context.Context, _ authentication.VerifiedIdentity, table string) (authorization.AccountAccess, error) {
	return loader.accessByTable[table], nil
}

type deleteRepositoryStub struct {
	result DeleteResult
	err    error
	called bool
}

func (repository *deleteRepositoryStub) DeleteCallCenterDocument(_ context.Context, complaintID string, documentID string, officeCode string) (DeleteResult, error) {
	repository.called = true
	return repository.result, repository.err
}

type deleteStoreStub struct {
	deletedKeys []string
}

func (store *deleteStoreStub) Delete(_ context.Context, key string) error {
	store.deletedKeys = append(store.deletedKeys, key)
	return nil
}

func newTestDeleteService(t *testing.T, repository DeleteRepository, store interface {
	Delete(context.Context, string) error
}) *DeleteService {
	t.Helper()
	access := authorization.AccountAccess{
		Username: "agent.test", OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionView | authorization.ActionDelete),
	}
	loader := &deleteAccessLoader{accessByTable: map[string]authorization.AccountAccess{
		complaintTableName: access,
		documentTableName:  access,
	}}
	if store == nil {
		store = &deleteStoreStub{}
	}
	return NewDeleteService(authorization.NewAccessService(loader), repository, store)
}

func TestDeleteServiceRemovesOfficeOwnedDocumentMetadataAndObject(t *testing.T) {
	repository := &deleteRepositoryStub{result: DeleteResult{
		ComplaintID: "TKT-001", DocumentID: "0123456789abcdef0123456789abcdef",
		StorageKey: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}}
	store := &deleteStoreStub{}
	service := newTestDeleteService(t, repository, store)
	result, err := service.Delete(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "UNTRUSTED"}, DeleteRequest{
		ComplaintID: "TKT-001", DocumentID: "0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.called || result.ComplaintID != "TKT-001" || result.DocumentID != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("delete result = %#v, repository called = %t", result, repository.called)
	}
	if len(store.deletedKeys) != 1 || store.deletedKeys[0] != result.StorageKey {
		t.Fatalf("stored object not removed: %#v", store.deletedKeys)
	}
}

func TestDeleteServiceRejectsInvalidDocumentIDBeforeAuthorization(t *testing.T) {
	repository := &deleteRepositoryStub{}
	store := &deleteStoreStub{}
	service := newTestDeleteService(t, repository, store)
	for _, documentID := range []string{"", "short", "not-hex-0000000000000000000000000000000"} {
		if _, err := service.Delete(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, DeleteRequest{
			ComplaintID: "TKT-001", DocumentID: documentID,
		}); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("Delete() error = %v, want invalid query", err)
		}
	}
	if repository.called {
		t.Fatal("invalid delete request reached repository")
	}
}

func TestDeleteServiceHidesMissingOrOutOfScopeDocument(t *testing.T) {
	repository := &deleteRepositoryStub{err: sql.ErrNoRows}
	store := &deleteStoreStub{}
	service := newTestDeleteService(t, repository, store)
	if _, err := service.Delete(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, DeleteRequest{
		ComplaintID: "TKT-001", DocumentID: "0123456789abcdef0123456789abcdef",
	}); !errors.Is(err, ErrNotFoundOrNotVisible) {
		t.Fatalf("Delete() error = %v, want hidden not found", err)
	}
	if len(store.deletedKeys) != 0 {
		t.Fatalf("stored object removed for missing metadata: %#v", store.deletedKeys)
	}
}

func TestDeleteServiceRequiresCallCenterRoleAndMatchingAccess(t *testing.T) {
	for _, test := range []struct {
		name        string
		accessByMap map[string]authorization.AccountAccess
		want        error
	}{
		{
			name: "document table missing delete permission",
			accessByMap: map[string]authorization.AccountAccess{
				complaintTableName: {OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionView)},
				documentTableName:  {OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionList)},
			},
			want: authorization.ErrPermissionDenied,
		},
		{
			name: "office mismatch",
			accessByMap: map[string]authorization.AccountAccess{
				complaintTableName: {OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionView)},
				documentTableName:  {OfficeCode: "40006CC", ReportTo: "40005", UserLevelIDs: []int64{3}, PermissionMask: int64(authorization.ActionDelete)},
			},
			want: authorization.ErrIdentityMismatch,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := NewDeleteService(authorization.NewAccessService(&deleteAccessLoader{accessByTable: test.accessByMap}), &deleteRepositoryStub{}, &deleteStoreStub{})
			if _, err := service.Delete(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "UNTRUSTED"}, DeleteRequest{
				ComplaintID: "TKT-001", DocumentID: "0123456789abcdef0123456789abcdef",
			}); !errors.Is(err, test.want) {
				t.Fatalf("Delete() error = %v, want %v", err, test.want)
			}
		})
	}
}

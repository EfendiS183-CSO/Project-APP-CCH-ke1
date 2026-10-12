package documents

import (
	"context"
	"errors"
	"testing"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type stubAccessLoader struct {
	accessByTable map[string]authorization.AccountAccess
	loadedTables  []string
	err           error
}

func (loader *stubAccessLoader) Load(_ context.Context, _ authentication.VerifiedIdentity, tableName string) (authorization.AccountAccess, error) {
	loader.loadedTables = append(loader.loadedTables, tableName)
	return loader.accessByTable[tableName], loader.err
}

type stubRepository struct {
	query  Query
	items  []Metadata
	err    error
	called bool
}

func (repository *stubRepository) ListForOpenCallCenterTicket(_ context.Context, query Query) ([]Metadata, error) {
	repository.called = true
	repository.query = query
	return repository.items, repository.err
}

func TestServiceListsMetadataUsingTrustedCallCenterOffice(t *testing.T) {
	access := authorization.AccountAccess{
		Username: "agent.test", OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionView | authorization.ActionList),
	}
	loader := &stubAccessLoader{accessByTable: map[string]authorization.AccountAccess{
		complaintTableName: access,
		documentTableName:  access,
	}}
	metadata := []Metadata{{ComplaintID: "TKT-001", HistoryID: "HIST-001", DocumentTypeCode: "BA1", FileCount: 2}}
	repository := &stubRepository{items: metadata}
	service := NewService(authorization.NewAccessService(loader), repository)
	items, err := service.ListForCallCenterTicket(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "ATTACKER"}, Query{
		ComplaintID: " TKT-001 ", OfficeCode: "ATTACKER", Limit: 10, Offset: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.called || repository.query.ComplaintID != "TKT-001" || repository.query.OfficeCode != "40005CC" || repository.query.Limit != 10 || repository.query.Offset != 20 {
		t.Fatalf("repository query = %#v", repository.query)
	}
	if len(items) != 1 || items[0].HistoryID != "HIST-001" {
		t.Fatalf("metadata = %#v", items)
	}
	if len(loader.loadedTables) != 2 || loader.loadedTables[0] != complaintTableName || loader.loadedTables[1] != documentTableName {
		t.Fatalf("authorization tables = %#v", loader.loadedTables)
	}
}

func TestServiceRequiresBothPermissionsSingleRoleAndMatchingIdentity(t *testing.T) {
	for _, test := range []struct {
		name      string
		complaint authorization.AccountAccess
		document  authorization.AccountAccess
		wantErr   error
	}{
		{
			name:      "missing document List",
			complaint: authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID}, PermissionMask: int64(authorization.ActionView)},
			document:  authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID}, PermissionMask: int64(authorization.ActionView)},
			wantErr:   authorization.ErrPermissionDenied,
		},
		{
			name:      "wrong role",
			complaint: authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{1}, PermissionMask: int64(authorization.ActionView)},
			document:  authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{1}, PermissionMask: int64(authorization.ActionList)},
			wantErr:   ErrCallCenterRoleRequired,
		},
		{
			name:      "office mismatch",
			complaint: authorization.AccountAccess{OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID}, PermissionMask: int64(authorization.ActionView)},
			document:  authorization.AccountAccess{OfficeCode: "40006CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID}, PermissionMask: int64(authorization.ActionList)},
			wantErr:   authorization.ErrIdentityMismatch,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &stubAccessLoader{accessByTable: map[string]authorization.AccountAccess{
				complaintTableName: test.complaint,
				documentTableName:  test.document,
			}}
			repository := &stubRepository{}
			service := NewService(authorization.NewAccessService(loader), repository)
			_, err := service.ListForCallCenterTicket(context.Background(), authentication.VerifiedIdentity{}, Query{ComplaintID: "TKT-001"})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ListForCallCenterTicket() error = %v, want %v", err, test.wantErr)
			}
			if repository.called {
				t.Fatal("unauthorized query reached document repository")
			}
		})
	}
}

func TestServiceValidatesQueryBeforeAuthorization(t *testing.T) {
	loader := &stubAccessLoader{}
	repository := &stubRepository{}
	service := NewService(authorization.NewAccessService(loader), repository)
	_, err := service.ListForCallCenterTicket(context.Background(), authentication.VerifiedIdentity{}, Query{})
	if !errors.Is(err, ErrInvalidQuery) || repository.called || len(loader.loadedTables) != 0 {
		t.Fatalf("error/calls = %v/%v/%#v", err, repository.called, loader.loadedTables)
	}
}

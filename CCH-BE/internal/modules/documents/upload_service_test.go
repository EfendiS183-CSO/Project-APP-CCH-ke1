package documents

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/documents/storage"
	"cch.local/backend/internal/platform/authentication"
)

type uploadAccessLoader struct {
	accessByTable map[string]authorization.AccountAccess
	loaded        []string
}

func (loader *uploadAccessLoader) Load(_ context.Context, _ authentication.VerifiedIdentity, table string) (authorization.AccountAccess, error) {
	loader.loaded = append(loader.loaded, table)
	return loader.accessByTable[table], nil
}

type uploadStoreStub struct {
	order     *[]string
	key       string
	content   []byte
	deleted   []string
	saveError error
}

func (store *uploadStoreStub) Save(_ context.Context, content []byte) (string, error) {
	if store.order != nil {
		*store.order = append(*store.order, "store")
	}
	if store.saveError != nil {
		return "", store.saveError
	}
	store.content = append([]byte(nil), content...)
	return store.key, nil
}

func (*uploadStoreStub) Open(context.Context, string) (io.ReadCloser, error) { return nil, nil }

func (store *uploadStoreStub) Delete(_ context.Context, key string) error {
	store.deleted = append(store.deleted, key)
	return nil
}

type uploadScannerStub struct {
	order  *[]string
	err    error
	called int
}

func (scanner *uploadScannerStub) Scan(_ context.Context, _ []byte) error {
	scanner.called++
	if scanner.order != nil {
		*scanner.order = append(*scanner.order, "scan")
	}
	return scanner.err
}

type uploadRepositoryStub struct {
	command UploadCommand
	result  UploadResult
	err     error
	called  bool
}

func (repository *uploadRepositoryStub) CreateCallCenterDocument(_ context.Context, command UploadCommand) (UploadResult, error) {
	repository.called = true
	repository.command = command
	return repository.result, repository.err
}

func TestUploadServiceAuthorizesScansThenPersistsMetadata(t *testing.T) {
	order := make([]string, 0, 2)
	store := &uploadStoreStub{order: &order, key: "private-object-key"}
	scanner := &uploadScannerStub{order: &order}
	ingestor, err := storage.NewIngestor(store, scanner)
	if err != nil {
		t.Fatal(err)
	}
	access := authorization.AccountAccess{
		Username: "agent.test", OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionView | authorization.ActionAdd),
	}
	loader := &uploadAccessLoader{accessByTable: map[string]authorization.AccountAccess{
		complaintTableName: access,
		documentTableName:  access,
	}}
	repository := &uploadRepositoryStub{result: UploadResult{ComplaintID: "TKT-001", StorageKey: "private-object-key"}}
	service := NewUploadService(authorization.NewAccessService(loader), repository, ingestor)
	request := validUploadRequest()
	result, err := service.Upload(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "UNTRUSTED"}, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "scan" || order[1] != "store" || !repository.called {
		t.Fatalf("scan/store/repository order = %#v/%v", order, repository.called)
	}
	command := repository.command
	if command.OfficeCode != "40005CC" || command.Actor != "agent.test" || command.StorageKey != "private-object-key" || command.MediaType != "application/pdf" || command.FileSize != int64(len(request.Content)) || command.SHA256 == "" || command.DocumentID == "" || command.RequestHash == "" {
		t.Fatalf("upload command = %#v", command)
	}
	if result.StorageKey != "private-object-key" || result.ComplaintID != request.ComplaintID {
		t.Fatalf("upload result = %#v", result)
	}
}

func TestUploadServiceRejectsInvalidOrUnauthorizedRequestsBeforeScanning(t *testing.T) {
	store := &uploadStoreStub{key: "private-object-key"}
	scanner := &uploadScannerStub{}
	ingestor, _ := storage.NewIngestor(store, scanner)
	loader := &uploadAccessLoader{}
	repository := &uploadRepositoryStub{}
	service := NewUploadService(authorization.NewAccessService(loader), repository, ingestor)
	request := validUploadRequest()
	request.IdempotencyKey = "short"
	if _, err := service.Upload(context.Background(), authentication.VerifiedIdentity{}, request); !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("invalid request error = %v", err)
	}
	if scanner.called != 0 || len(loader.loaded) != 0 || repository.called {
		t.Fatal("invalid upload reached authorization, scanner, or repository")
	}
	access := authorization.AccountAccess{OfficeCode: "40005CC", UserLevelIDs: []int64{1}, PermissionMask: int64(authorization.ActionView | authorization.ActionAdd)}
	loader = &uploadAccessLoader{accessByTable: map[string]authorization.AccountAccess{complaintTableName: access, documentTableName: access}}
	repository = &uploadRepositoryStub{}
	service = NewUploadService(authorization.NewAccessService(loader), repository, ingestor)
	if _, err := service.Upload(context.Background(), authentication.VerifiedIdentity{}, validUploadRequest()); !errors.Is(err, ErrCallCenterRoleRequired) {
		t.Fatalf("wrong role error = %v", err)
	}
	if scanner.called != 0 || repository.called {
		t.Fatal("unauthorized upload reached scanner or repository")
	}
}

func TestUploadServiceDeletesStoredObjectAfterKnownFailureAndReplay(t *testing.T) {
	for _, test := range []struct {
		name       string
		repository UploadResult
		repoError  error
		wantDelete bool
	}{
		{name: "known database failure", repoError: errors.New("insert failed"), wantDelete: true},
		{name: "exact replay", repository: UploadResult{DocumentID: "existing", StorageKey: "existing-key", Replayed: true}, wantDelete: true},
		{name: "uncertain commit", repoError: ErrUploadCommitUncertain, wantDelete: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &uploadStoreStub{key: "new-private-object"}
			ingestor, _ := storage.NewIngestor(store, &uploadScannerStub{})
			access := authorization.AccountAccess{Username: "agent.test", OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID}, PermissionMask: int64(authorization.ActionView | authorization.ActionAdd)}
			loader := &uploadAccessLoader{accessByTable: map[string]authorization.AccountAccess{complaintTableName: access, documentTableName: access}}
			repository := &uploadRepositoryStub{result: test.repository, err: test.repoError}
			service := NewUploadService(authorization.NewAccessService(loader), repository, ingestor)
			_, err := service.Upload(context.Background(), authentication.VerifiedIdentity{}, validUploadRequest())
			if test.repoError != nil && !errors.Is(err, test.repoError) {
				t.Fatalf("Upload() error = %v, want %v", err, test.repoError)
			}
			if test.repoError == nil && err != nil {
				t.Fatal(err)
			}
			if gotDelete := len(store.deleted) > 0; gotDelete != test.wantDelete {
				t.Fatalf("object cleanup = %v, want %v", gotDelete, test.wantDelete)
			}
		})
	}
}

func validUploadRequest() UploadRequest {
	return UploadRequest{
		ComplaintID: "TKT-001", HistoryID: "HIST-001", DocumentType: "BA1", FileName: "report.pdf", ContentType: "application/pdf",
		IdempotencyKey: "upload-operation-key-0001", Content: []byte("%PDF-1.7\ncontent"),
	}
}

var _ = time.Second

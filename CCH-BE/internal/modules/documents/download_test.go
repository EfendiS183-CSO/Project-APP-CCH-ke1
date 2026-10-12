package documents

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/modules/documents/storage"
	"cch.local/backend/internal/platform/authentication"
)

type downloadAccessLoader struct {
	accessByTable map[string]authorization.AccountAccess
}

func (loader *downloadAccessLoader) Load(_ context.Context, _ authentication.VerifiedIdentity, table string) (authorization.AccountAccess, error) {
	return loader.accessByTable[table], nil
}

type downloadRepositoryStub struct {
	metadata DownloadMetadata
	err      error
	called   bool
}

func (repository *downloadRepositoryStub) GetCallCenterDocument(_ context.Context, _ string, _ string, _ string) (DownloadMetadata, error) {
	repository.called = true
	return repository.metadata, repository.err
}

type cleanScanner struct{}

func (cleanScanner) Scan(context.Context, []byte) error { return nil }

type unavailableDownloadStore struct{}

func (unavailableDownloadStore) Save(context.Context, []byte) (string, error) {
	return "", storage.ErrStorageUnavailable
}
func (unavailableDownloadStore) Open(context.Context, string) (io.ReadCloser, error) {
	return nil, storage.ErrStorageUnavailable
}
func (unavailableDownloadStore) Delete(context.Context, string) error {
	return storage.ErrStorageUnavailable
}

func TestDownloadServiceReturnsVerifiedBytesFromPrivateStore(t *testing.T) {
	basePath := t.TempDir()
	publicRoot := filepath.Join(basePath, "www")
	if err := os.Mkdir(publicRoot, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewPrivateFileStore(filepath.Join(basePath, "private"), publicRoot)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("%PDF-1.7\ntrusted test bytes")
	key, err := store.Save(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	ingestor, err := storage.NewIngestor(store, cleanScanner{})
	if err != nil {
		t.Fatal(err)
	}
	account := authorization.AccountAccess{
		Username: "agent.test", OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionView | authorization.ActionList),
	}
	loader := &downloadAccessLoader{accessByTable: map[string]authorization.AccountAccess{
		complaintTableName: account,
		documentTableName:  account,
	}}
	repository := &downloadRepositoryStub{metadata: DownloadMetadata{
		DocumentID: "0123456789abcdef0123456789abcdef", StorageKey: key, MediaType: "application/pdf",
		Extension: ".pdf", FileSize: int64(len(content)), SHA256: sha256Hex(content),
	}}
	service := NewDownloadService(authorization.NewAccessService(loader), repository, ingestor)
	result, err := service.Download(context.Background(), authentication.VerifiedIdentity{Username: "agent.test", OfficeCode: "UNTRUSTED"}, DownloadRequest{
		ComplaintID: "TKT-001", DocumentID: repository.metadata.DocumentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repository.called || string(result.Content) != string(content) || result.Metadata.DocumentID != repository.metadata.DocumentID {
		t.Fatalf("download result = %#v", result)
	}
}

func TestDownloadServiceRejectsModifiedObjectAndInaccessibleMetadata(t *testing.T) {
	t.Run("checksum mismatch", func(t *testing.T) {
		basePath := t.TempDir()
		publicRoot := filepath.Join(basePath, "www")
		if err := os.Mkdir(publicRoot, 0700); err != nil {
			t.Fatal(err)
		}
		store, err := storage.NewPrivateFileStore(filepath.Join(basePath, "private"), publicRoot)
		if err != nil {
			t.Fatal(err)
		}
		key, err := store.Save(context.Background(), []byte("%PDF-1.7\nmodified"))
		if err != nil {
			t.Fatal(err)
		}
		ingestor, _ := storage.NewIngestor(store, cleanScanner{})
		service := newTestDownloadService(t, ingestor, DownloadMetadata{
			DocumentID: "0123456789abcdef0123456789abcdef", StorageKey: key, MediaType: "application/pdf",
			Extension: ".pdf", FileSize: int64(len("%PDF-1.7\nmodified")), SHA256: sha256Hex([]byte("%PDF-1.7\noriginal")),
		}, nil)
		if _, err := service.Download(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, DownloadRequest{
			ComplaintID: "TKT-001", DocumentID: "0123456789abcdef0123456789abcdef",
		}); !errors.Is(err, ErrDownloadUnavailable) {
			t.Fatalf("Download() error = %v, want unavailable for checksum mismatch", err)
		}
	})
	t.Run("missing scope", func(t *testing.T) {
		service := newTestDownloadService(t, nil, DownloadMetadata{}, sql.ErrNoRows)
		if _, err := service.Download(context.Background(), authentication.VerifiedIdentity{Username: "agent.test"}, DownloadRequest{
			ComplaintID: "TKT-001", DocumentID: "0123456789abcdef0123456789abcdef",
		}); !errors.Is(err, ErrNotFoundOrNotVisible) {
			t.Fatalf("Download() error = %v, want hidden not found", err)
		}
	})
}

func newTestDownloadService(t *testing.T, ingestor *storage.Ingestor, metadata DownloadMetadata, repositoryError error) *DownloadService {
	t.Helper()
	if ingestor == nil {
		ingestor, _ = storage.NewIngestor(unavailableDownloadStore{}, cleanScanner{})
	}
	access := authorization.AccountAccess{
		Username: "agent.test", OfficeCode: "40005CC", ReportTo: "40005", UserLevelIDs: []int64{callCenterRoleID},
		PermissionMask: int64(authorization.ActionView | authorization.ActionList),
	}
	return NewDownloadService(authorization.NewAccessService(&downloadAccessLoader{accessByTable: map[string]authorization.AccountAccess{
		complaintTableName: access, documentTableName: access,
	}}), &downloadRepositoryStub{metadata: metadata, err: repositoryError}, ingestor)
}

func sha256Hex(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

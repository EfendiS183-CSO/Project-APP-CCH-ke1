package storage

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
)

type stubStore struct {
	order   *[]string
	content []byte
	err     error
	calls   int
}

func (store *stubStore) Save(_ context.Context, content []byte) (string, error) {
	store.calls++
	if store.order != nil {
		*store.order = append(*store.order, "store")
	}
	store.content = append([]byte(nil), content...)
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", store.err
}

func (*stubStore) Open(context.Context, string) (io.ReadCloser, error) { return nil, nil }

func (*stubStore) Delete(context.Context, string) error { return nil }

type stubScanner struct {
	order *[]string
	err   error
	calls int
}

func (scanner *stubScanner) Scan(context.Context, []byte) error {
	scanner.calls++
	if scanner.order != nil {
		*scanner.order = append(*scanner.order, "scan")
	}
	return scanner.err
}

func TestIngestorScansBeforeStoringAndReturnsIntegrityMetadata(t *testing.T) {
	order := make([]string, 0, 2)
	store := &stubStore{order: &order}
	scanner := &stubScanner{order: &order}
	ingestor, err := NewIngestor(store, scanner)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("%PDF-1.7\ntrusted test content")
	result, err := ingestor.Ingest(context.Background(), "report.pdf", content)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"scan", "store"}) || scanner.calls != 1 || store.calls != 1 {
		t.Fatalf("call order/counts = %#v/%d/%d", order, scanner.calls, store.calls)
	}
	if result.MediaType != "application/pdf" || result.Extension != ".pdf" || result.SizeBytes != int64(len(content)) || result.SHA256 == "" || result.StorageKey == "" {
		t.Fatalf("ingested metadata = %#v", result)
	}
}

func TestIngestorNeverStoresInvalidOrInfectedFiles(t *testing.T) {
	t.Run("invalid file", func(t *testing.T) {
		store := &stubStore{}
		scanner := &stubScanner{}
		ingestor, _ := NewIngestor(store, scanner)
		if _, err := ingestor.Ingest(context.Background(), "payload.exe", []byte("MZ")); !errors.Is(err, ErrUnsupportedFile) {
			t.Fatalf("Ingest() error = %v, want unsupported file", err)
		}
		if scanner.calls != 0 || store.calls != 0 {
			t.Fatalf("invalid file reached scanner/store: %d/%d", scanner.calls, store.calls)
		}
	})
	t.Run("malware", func(t *testing.T) {
		store := &stubStore{}
		scanner := &stubScanner{err: ErrMalwareDetected}
		ingestor, _ := NewIngestor(store, scanner)
		if _, err := ingestor.Ingest(context.Background(), "report.pdf", []byte("%PDF-1.7\ncontent")); !errors.Is(err, ErrMalwareDetected) {
			t.Fatalf("Ingest() error = %v, want malware detected", err)
		}
		if scanner.calls != 1 || store.calls != 0 {
			t.Fatalf("infected file reached store: scan/store=%d/%d", scanner.calls, store.calls)
		}
	})
}

package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

var ErrIngestionNotConfigured = errors.New("document ingestion is not configured")

type IngestedFile struct {
	StorageKey string
	MediaType  string
	Extension  string
	SizeBytes  int64
	SHA256     string
}

type Ingestor struct {
	store   Store
	scanner Scanner
}

func NewIngestor(store Store, scanner Scanner) (*Ingestor, error) {
	if store == nil || scanner == nil {
		return nil, ErrIngestionNotConfigured
	}
	return &Ingestor{store: store, scanner: scanner}, nil
}

func (ingestor *Ingestor) Ingest(ctx context.Context, filename string, content []byte) (IngestedFile, error) {
	if ingestor == nil || ingestor.store == nil || ingestor.scanner == nil {
		return IngestedFile{}, ErrIngestionNotConfigured
	}
	fileInfo, err := ValidateFile(filename, content)
	if err != nil {
		return IngestedFile{}, err
	}
	if err := ingestor.scanner.Scan(ctx, content); err != nil {
		return IngestedFile{}, fmt.Errorf("scan uploaded document: %w", err)
	}
	storageKey, err := ingestor.store.Save(ctx, content)
	if err != nil {
		return IngestedFile{}, fmt.Errorf("store scanned document: %w", err)
	}
	digest := sha256.Sum256(content)
	return IngestedFile{
		StorageKey: storageKey,
		MediaType:  fileInfo.MediaType,
		Extension:  fileInfo.Extension,
		SizeBytes:  int64(len(content)),
		SHA256:     hex.EncodeToString(digest[:]),
	}, nil
}

func (ingestor *Ingestor) Delete(ctx context.Context, storageKey string) error {
	if ingestor == nil || ingestor.store == nil {
		return ErrIngestionNotConfigured
	}
	return ingestor.store.Delete(ctx, storageKey)
}

func (ingestor *Ingestor) Open(ctx context.Context, storageKey string) (io.ReadCloser, error) {
	if ingestor == nil || ingestor.store == nil {
		return nil, ErrIngestionNotConfigured
	}
	return ingestor.store.Open(ctx, storageKey)
}

package storage

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateFileRequiresAllowedExtensionAndMatchingContent(t *testing.T) {
	for _, test := range []struct {
		name      string
		filename  string
		content   []byte
		wantType  string
		wantError error
	}{
		{name: "pdf", filename: "report.pdf", content: []byte("%PDF-1.7\ncontent"), wantType: "application/pdf"},
		{name: "jpeg", filename: "photo.jpg", content: []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 'J', 'F', 'I', 'F', 0, 1}, wantType: "image/jpeg"},
		{name: "png", filename: "photo.png", content: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0}, wantType: "image/png"},
		{name: "extension mismatch", filename: "payload.pdf", content: []byte{0xff, 0xd8, 0xff}, wantError: ErrUnsupportedFile},
		{name: "unsupported archive", filename: "bundle.zip", content: []byte("PK\x03\x04"), wantError: ErrUnsupportedFile},
		{name: "empty", filename: "empty.pdf", content: nil, wantError: ErrUnsupportedFile},
		{name: "oversized", filename: "large.pdf", content: make([]byte, MaxFileBytes+1), wantError: ErrFileTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := ValidateFile(test.filename, test.content)
			if test.wantError != nil {
				if !errors.Is(err, test.wantError) {
					t.Fatalf("ValidateFile() error = %v, want %v", err, test.wantError)
				}
				return
			}
			if err != nil || result.MediaType != test.wantType {
				t.Fatalf("ValidateFile() = %#v, %v, want type %q", result, err, test.wantType)
			}
		})
	}
}

func TestPrivateFileStoreRejectsPublicOrInvalidRoots(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "www")
	if err := os.MkdirAll(publicRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPrivateFileStore(filepath.Join(publicRoot, "files"), publicRoot); !errors.Is(err, ErrInvalidStorageRoot) {
		t.Fatalf("public descendant error = %v, want invalid root", err)
	}
	if _, err := NewPrivateFileStore("relative-private", publicRoot); !errors.Is(err, ErrInvalidStorageRoot) {
		t.Fatalf("relative root error = %v, want invalid root", err)
	}
	if _, err := NewPrivateFileStore(filepath.Join(root, "private"), filepath.Join(root, "missing-public")); !errors.Is(err, ErrInvalidStorageRoot) {
		t.Fatalf("missing public root error = %v, want invalid root", err)
	}
}

func TestPrivateFileStoreSavesReadsAndDeletesOpaqueObjects(t *testing.T) {
	basePath := t.TempDir()
	publicRoot := filepath.Join(basePath, "www")
	storageRoot := filepath.Join(basePath, "private", "documents")
	if err := os.MkdirAll(publicRoot, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := NewPrivateFileStore(storageRoot, publicRoot)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("private document bytes")
	key, err := store.Save(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 64 || strings.ContainsAny(key, `/\.`) {
		t.Fatalf("storage key is not opaque: %q", key)
	}
	storedPath := filepath.Join(storageRoot, key)
	if _, err := os.Stat(storedPath); err != nil {
		t.Fatalf("stored object is missing: %v", err)
	}
	file, err := store.Open(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	readContent, err := io.ReadAll(file)
	file.Close()
	if err != nil || string(readContent) != string(content) {
		t.Fatalf("read content/error = %q/%v", readContent, err)
	}
	if _, err := store.Open(context.Background(), "../secret"); !errors.Is(err, ErrInvalidStorageKey) {
		t.Fatalf("traversal key error = %v", err)
	}
	if err := store.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(storedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted file stat error = %v, want not exists", err)
	}
}

func TestClamAVScannerAcceptsCleanAndRejectsInfectedStreams(t *testing.T) {
	for _, test := range []struct {
		name      string
		response  string
		wantError error
	}{
		{name: "clean", response: "stream: OK\x00"},
		{name: "infected", response: "stream: Eicar-Test-Signature FOUND\x00", wantError: ErrMalwareDetected},
		{name: "scanner protocol error", response: "unexpected response\x00", wantError: ErrScannerUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			address := startFakeClamAV(t, test.response)
			scanner, err := NewClamAVScanner(address, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			err = scanner.Scan(context.Background(), []byte("sample bytes"))
			if test.wantError == nil && err != nil {
				t.Fatalf("Scan() error = %v", err)
			}
			if test.wantError != nil && !errors.Is(err, test.wantError) {
				t.Fatalf("Scan() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

func TestClamAVScannerFailsClosedWhenUnavailable(t *testing.T) {
	scanner, err := NewClamAVScanner("127.0.0.1:1", 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if err := scanner.Scan(context.Background(), []byte("sample bytes")); !errors.Is(err, ErrScannerUnavailable) {
		t.Fatalf("Scan() error = %v, want scanner unavailable", err)
	}
}

func startFakeClamAV(t *testing.T, response string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		reader := bufio.NewReader(connection)
		if _, err := reader.ReadString(0); err != nil {
			return
		}
		for {
			var header [4]byte
			if _, err := io.ReadFull(reader, header[:]); err != nil {
				return
			}
			chunkSize := binary.BigEndian.Uint32(header[:])
			if chunkSize == 0 {
				break
			}
			if _, err := io.CopyN(io.Discard, reader, int64(chunkSize)); err != nil {
				return
			}
		}
		_, _ = io.WriteString(connection, response)
	}()
	return listener.Addr().String()
}

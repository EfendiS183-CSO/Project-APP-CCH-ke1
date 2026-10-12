package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MaxFileBytes  = 2_000_000
	clamChunkSize = 32 * 1024
)

var (
	ErrInvalidStorageRoot = errors.New("document storage root must be private and outside the public root")
	ErrInvalidStorageKey  = errors.New("invalid document storage key")
	ErrStorageUnavailable = errors.New("document storage is unavailable")
	ErrScannerUnavailable = errors.New("malware scanner is unavailable")
	ErrMalwareDetected    = errors.New("malware was detected in the document")
	ErrUnsupportedFile    = errors.New("unsupported document type or content")
	ErrFileTooLarge       = errors.New("document exceeds the configured size limit")
)

type Scanner interface {
	Scan(context.Context, []byte) error
}

type Store interface {
	Save(context.Context, []byte) (string, error)
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

type FileInfo struct {
	MediaType string
	Extension string
}

func ValidateFile(filename string, content []byte) (FileInfo, error) {
	if len(content) == 0 {
		return FileInfo{}, ErrUnsupportedFile
	}
	if len(content) > MaxFileBytes {
		return FileInfo{}, ErrFileTooLarge
	}
	baseName := filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	extension := strings.ToLower(filepath.Ext(baseName))
	mediaType, _, err := mime.ParseMediaType(http.DetectContentType(content[:min(len(content), 512)]))
	if err != nil {
		return FileInfo{}, ErrUnsupportedFile
	}
	switch {
	case extension == ".pdf" && mediaType == "application/pdf":
		return FileInfo{MediaType: mediaType, Extension: extension}, nil
	case (extension == ".jpg" || extension == ".jpeg") && mediaType == "image/jpeg":
		return FileInfo{MediaType: mediaType, Extension: extension}, nil
	case extension == ".png" && mediaType == "image/png":
		return FileInfo{MediaType: mediaType, Extension: extension}, nil
	default:
		return FileInfo{}, ErrUnsupportedFile
	}
}

type PrivateFileStore struct {
	root string
}

func NewPrivateFileStore(storageRoot string, publicRoot string) (*PrivateFileStore, error) {
	storageRoot = strings.TrimSpace(storageRoot)
	publicRoot = strings.TrimSpace(publicRoot)
	if storageRoot == "" || publicRoot == "" || !filepath.IsAbs(storageRoot) || !filepath.IsAbs(publicRoot) {
		return nil, ErrInvalidStorageRoot
	}
	storageRoot, err := filepath.Abs(filepath.Clean(storageRoot))
	if err != nil {
		return nil, ErrInvalidStorageRoot
	}
	publicRoot, err = filepath.Abs(filepath.Clean(publicRoot))
	if err != nil {
		return nil, ErrInvalidStorageRoot
	}
	if err := ensurePrivateRoot(storageRoot); err != nil {
		return nil, err
	}
	resolvedStorageRoot, err := filepath.EvalSymlinks(storageRoot)
	if err != nil {
		return nil, ErrInvalidStorageRoot
	}
	resolvedPublicRoot, err := filepath.EvalSymlinks(publicRoot)
	if err != nil {
		return nil, ErrInvalidStorageRoot
	}
	if pathWithin(resolvedPublicRoot, resolvedStorageRoot) {
		return nil, ErrInvalidStorageRoot
	}
	return &PrivateFileStore{root: resolvedStorageRoot}, nil
}

func (store *PrivateFileStore) Save(ctx context.Context, content []byte) (string, error) {
	if store == nil || store.root == "" {
		return "", ErrStorageUnavailable
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(content) == 0 || len(content) > MaxFileBytes {
		return "", ErrFileTooLarge
	}
	var randomKey [32]byte
	if _, err := rand.Read(randomKey[:]); err != nil {
		return "", fmt.Errorf("generate document storage key: %w", err)
	}
	key := hex.EncodeToString(randomKey[:])
	temporaryPath := filepath.Join(store.root, key+".tmp")
	finalPath := filepath.Join(store.root, key)
	file, err := osOpenFileExclusive(temporaryPath)
	if err != nil {
		return "", fmt.Errorf("create private document object: %w", err)
	}
	cleanup := func() {
		_ = file.Close()
		_ = osRemove(temporaryPath)
	}
	if _, err := file.Write(content); err != nil {
		cleanup()
		return "", fmt.Errorf("write private document object: %w", err)
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return "", fmt.Errorf("sync private document object: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = osRemove(temporaryPath)
		return "", fmt.Errorf("close private document object: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = osRemove(temporaryPath)
		return "", err
	}
	if err := osRename(temporaryPath, finalPath); err != nil {
		_ = osRemove(temporaryPath)
		return "", fmt.Errorf("publish private document object: %w", err)
	}
	return key, nil
}

func (store *PrivateFileStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if store == nil || store.root == "" {
		return nil, ErrStorageUnavailable
	}
	if err := validateKey(key); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := filepath.Join(store.root, key)
	info, err := osLstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrStorageUnavailable
	}
	file, err := osOpen(path)
	if err != nil {
		return nil, ErrStorageUnavailable
	}
	return file, nil
}

func (store *PrivateFileStore) Delete(ctx context.Context, key string) error {
	if store == nil || store.root == "" {
		return ErrStorageUnavailable
	}
	if err := validateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := osRemove(filepath.Join(store.root, key)); err != nil && !errors.Is(err, errNotExist()) {
		return ErrStorageUnavailable
	}
	return nil
}

type ClamAVScanner struct {
	address string
	timeout time.Duration
}

func NewClamAVScanner(address string, timeout time.Duration) (*ClamAVScanner, error) {
	address = strings.TrimSpace(address)
	if _, _, err := net.SplitHostPort(address); err != nil || timeout <= 0 {
		return nil, ErrScannerUnavailable
	}
	return &ClamAVScanner{address: address, timeout: timeout}, nil
}

func (scanner *ClamAVScanner) Scan(ctx context.Context, content []byte) error {
	if scanner == nil || scanner.address == "" || scanner.timeout <= 0 {
		return ErrScannerUnavailable
	}
	dialer := net.Dialer{Timeout: scanner.timeout}
	connection, err := dialer.DialContext(ctx, "tcp", scanner.address)
	if err != nil {
		return fmt.Errorf("connect malware scanner: %w", ErrScannerUnavailable)
	}
	stopClose := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stopClose()
	deadline := time.Now().Add(scanner.timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set malware scanner deadline: %w", ErrScannerUnavailable)
	}
	if _, err := io.WriteString(connection, "zINSTREAM\x00"); err != nil {
		return fmt.Errorf("start malware scan: %w", ErrScannerUnavailable)
	}
	for offset := 0; offset < len(content); offset += clamChunkSize {
		chunkEnd := min(offset+clamChunkSize, len(content))
		if err := writeChunk(connection, content[offset:chunkEnd]); err != nil {
			return fmt.Errorf("send file to malware scanner: %w", ErrScannerUnavailable)
		}
	}
	if err := writeChunk(connection, nil); err != nil {
		return fmt.Errorf("finish malware scan: %w", ErrScannerUnavailable)
	}
	response, err := io.ReadAll(io.LimitReader(connection, 1024))
	if err != nil {
		return fmt.Errorf("read malware scan result: %w", ErrScannerUnavailable)
	}
	result := strings.TrimSpace(strings.TrimSuffix(string(response), "\x00"))
	if strings.HasSuffix(result, " OK") {
		return nil
	}
	if strings.HasSuffix(result, " FOUND") {
		return ErrMalwareDetected
	}
	return ErrScannerUnavailable
}

func writeChunk(writer io.Writer, chunk []byte) error {
	length := uint32(len(chunk))
	var header [4]byte
	header[0] = byte(length >> 24)
	header[1] = byte(length >> 16)
	header[2] = byte(length >> 8)
	header[3] = byte(length)
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	if len(chunk) == 0 {
		return nil
	}
	_, err := writer.Write(chunk)
	return err
}

func validateKey(key string) error {
	if len(key) != 64 {
		return ErrInvalidStorageKey
	}
	if _, err := hex.DecodeString(key); err != nil {
		return ErrInvalidStorageKey
	}
	return nil
}

func pathWithin(parent string, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func ensurePrivateRoot(root string) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return ErrInvalidStorageRoot
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalidStorageRoot
	}
	if err := os.Chmod(root, 0700); err != nil {
		return ErrInvalidStorageRoot
	}
	return nil
}

func osOpenFileExclusive(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
}

func osRemove(path string) error {
	return os.Remove(path)
}

func osRename(oldPath string, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func osLstat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

func osOpen(path string) (*os.File, error) {
	return os.Open(path)
}

func errNotExist() error {
	return fs.ErrNotExist
}

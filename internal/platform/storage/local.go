package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var generatedObjectKeyPattern = regexp.MustCompile(`^[0-9a-f]{32}\.blob$`)

// LocalProvider stores private objects below a configured root. Keys are
// generated from cryptographic random bytes and are never accepted as paths.
type LocalProvider struct {
	root   string
	newKey func() (string, error)
}

func NewLocalProvider(root string) (*LocalProvider, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, ErrInvalidProvider
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root: %w", err)
	}
	if info, statErr := os.Lstat(absolute); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, ErrInvalidProvider
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect storage root: %w", statErr)
	} else if makeErr := os.MkdirAll(absolute, 0700); makeErr != nil {
		return nil, fmt.Errorf("create storage root: %w", makeErr)
	}
	if info, statErr := os.Lstat(absolute); statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, ErrInvalidProvider
	}
	if chmodErr := os.Chmod(absolute, 0700); chmodErr != nil {
		return nil, fmt.Errorf("protect storage root: %w", chmodErr)
	}
	return &LocalProvider{root: absolute, newKey: randomObjectKey}, nil
}

func (provider *LocalProvider) Put(ctx context.Context, request PutRequest) (ObjectInfo, error) {
	if err := ValidateContext(ctx); err != nil {
		return ObjectInfo{}, err
	}
	if err := ValidatePutRequest(request); err != nil {
		return ObjectInfo{}, err
	}
	if provider == nil || provider.root == "" || provider.newKey == nil {
		return ObjectInfo{}, ErrInvalidProvider
	}
	for attempt := 0; attempt < 3; attempt++ {
		key, err := provider.newKey()
		if err != nil {
			return ObjectInfo{}, fmt.Errorf("generate storage key: %w", err)
		}
		info, putErr := provider.put(ctx, key, request)
		if !errors.Is(putErr, ErrObjectExists) {
			return info, putErr
		}
	}
	return ObjectInfo{}, ErrObjectExists
}

func (provider *LocalProvider) put(ctx context.Context, key string, request PutRequest) (ObjectInfo, error) {
	if err := ValidateContext(ctx); err != nil {
		return ObjectInfo{}, err
	}
	if err := ValidatePutRequest(request); err != nil {
		return ObjectInfo{}, err
	}
	path, err := provider.pathForKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ObjectInfo{}, ErrObjectExists
		}
		return ObjectInfo{}, fmt.Errorf("create storage object: %w", err)
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	if chmodErr := file.Chmod(0600); chmodErr != nil {
		return ObjectInfo{}, fmt.Errorf("protect storage object: %w", chmodErr)
	}
	written, copyErr := io.Copy(file, contextReader{ctx: ctx, reader: request.Body})
	if copyErr != nil {
		return ObjectInfo{}, fmt.Errorf("write storage object: %w", copyErr)
	}
	if request.Size >= 0 && written != request.Size {
		return ObjectInfo{}, ErrSizeMismatch
	}
	if syncErr := file.Sync(); syncErr != nil {
		return ObjectInfo{}, fmt.Errorf("sync storage object: %w", syncErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		return ObjectInfo{}, fmt.Errorf("close storage object: %w", closeErr)
	}
	fileInfo, statErr := os.Stat(path)
	if statErr != nil {
		return ObjectInfo{}, fmt.Errorf("stat stored object: %w", statErr)
	}
	keep = true
	return ObjectInfo{Key: key, Size: written, ModTime: fileInfo.ModTime().UTC()}, nil
}

func (provider *LocalProvider) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	if err := ValidateContext(ctx); err != nil {
		return ObjectInfo{}, err
	}
	path, err := provider.pathForKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ObjectInfo{}, ErrObjectNotFound
	}
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("stat storage object: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ObjectInfo{}, ErrInvalidObject
	}
	return ObjectInfo{Key: NormalizeKey(key), Size: info.Size(), ModTime: info.ModTime().UTC()}, nil
}

func (provider *LocalProvider) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	if err := ValidateContext(ctx); err != nil {
		return nil, ObjectInfo{}, err
	}
	path, err := provider.pathForKey(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	linkInfo, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ObjectInfo{}, ErrObjectNotFound
	}
	if err != nil {
		return nil, ObjectInfo{}, fmt.Errorf("inspect storage object: %w", err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 || !linkInfo.Mode().IsRegular() {
		return nil, ObjectInfo{}, ErrInvalidObject
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ObjectInfo{}, ErrObjectNotFound
	}
	if err != nil {
		return nil, ObjectInfo{}, fmt.Errorf("open storage object: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		if err != nil {
			return nil, ObjectInfo{}, fmt.Errorf("stat opened storage object: %w", err)
		}
		return nil, ObjectInfo{}, ErrInvalidObject
	}
	return file, ObjectInfo{Key: NormalizeKey(key), Size: info.Size(), ModTime: info.ModTime().UTC()}, nil
}

func (provider *LocalProvider) Delete(ctx context.Context, key string) error {
	if err := ValidateContext(ctx); err != nil {
		return err
	}
	path, err := provider.pathForKey(key)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect storage object for delete: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ErrInvalidObject
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("delete storage object: %w", err)
	}
	return nil
}

func (provider *LocalProvider) pathForKey(key string) (string, error) {
	if provider == nil || strings.TrimSpace(provider.root) == "" {
		return "", ErrInvalidProvider
	}
	key = NormalizeKey(key)
	if !generatedObjectKeyPattern.MatchString(key) {
		return "", ErrInvalidKey
	}
	path := filepath.Join(provider.root, key)
	relative, err := filepath.Rel(provider.root, path)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", ErrInvalidKey
	}
	return path, nil
}

func randomObjectKey() (string, error) {
	var raw [16]byte
	if _, err := io.ReadFull(rand.Reader, raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]) + ".blob", nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(buffer)
}

var _ ObjectStore = (*LocalProvider)(nil)

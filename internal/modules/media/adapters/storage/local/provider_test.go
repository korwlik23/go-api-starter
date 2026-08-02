package local

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	platformstorage "github.com/korwlik23/go-api-starter/internal/platform/storage"
	"github.com/stretchr/testify/require"
)

func TestM3B1LocalProviderGeneratesOpaqueKeysAndSupportsCompensation(t *testing.T) {
	t.Parallel()

	provider, err := NewProvider(t.TempDir())
	require.NoError(t, err)
	first, err := provider.Put(context.Background(), platformstorage.PutRequest{
		Body: strings.NewReader("first"), Size: 5,
	})
	require.NoError(t, err)
	second, err := provider.Put(context.Background(), platformstorage.PutRequest{
		Body: strings.NewReader("second"), Size: 6,
	})
	require.NoError(t, err)
	require.NotEqual(t, first.Key, second.Key)
	require.NotContains(t, first.Key, "/")
	require.NotContains(t, first.Key, `\`)

	info, err := provider.Stat(context.Background(), first.Key)
	require.NoError(t, err)
	require.Equal(t, int64(5), info.Size)
	reader, opened, err := provider.Open(context.Background(), first.Key)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.Equal(t, first, opened)
	require.Equal(t, []byte("first"), body)

	require.NoError(t, provider.Delete(context.Background(), first.Key))
	require.NoError(t, provider.Delete(context.Background(), first.Key))
	_, err = provider.Stat(context.Background(), first.Key)
	require.ErrorIs(t, err, platformstorage.ErrObjectNotFound)
}

func TestM3B1LocalProviderRejectsTraversalAndSymlinks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	provider, err := NewProvider(root)
	require.NoError(t, err)
	outside := filepath.Join(root, "..", "outside.txt")
	require.NoError(t, os.WriteFile(outside, []byte("keep"), 0600))
	for _, key := range []string{"../outside.txt", `..\outside.txt`, "/etc/passwd", "not-a-generated-key"} {
		_, err := provider.Stat(context.Background(), key)
		require.ErrorIs(t, err, platformstorage.ErrInvalidKey, key)
		require.ErrorIs(t, provider.Delete(context.Background(), key), platformstorage.ErrInvalidKey, key)
	}
	content, err := os.ReadFile(outside)
	require.NoError(t, err)
	require.Equal(t, []byte("keep"), content)

	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions are not consistently available on Windows")
	}
	link := filepath.Join(root, "018f1f64-7b2a-7c10-8bf2-3c40d5f67585.blob")
	require.NoError(t, os.Symlink(outside, link))
	_, err = provider.Stat(context.Background(), filepath.Base(link))
	require.ErrorIs(t, err, platformstorage.ErrInvalidObject)
}

func TestM3B1LocalProviderDoesNotOverwriteAndCleansFailedPut(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	provider, err := NewProvider(root)
	require.NoError(t, err)
	key := "018f1f647b2a7c108bf23c40d5f67585.blob"
	require.NoError(t, os.WriteFile(filepath.Join(root, key), []byte("original"), 0600))
	_, err = provider.put(context.Background(), key, platformstorage.PutRequest{Body: strings.NewReader("replacement"), Size: 11})
	require.ErrorIs(t, err, platformstorage.ErrObjectExists)
	content, err := os.ReadFile(filepath.Join(root, key))
	require.NoError(t, err)
	require.Equal(t, []byte("original"), content)

	_, err = provider.Put(context.Background(), platformstorage.PutRequest{Body: strings.NewReader("wrong"), Size: 99})
	require.ErrorIs(t, err, platformstorage.ErrSizeMismatch)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestM3B1LocalProviderUsesPrivatePermissions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	provider, err := NewProvider(root)
	require.NoError(t, err)
	object, err := provider.Put(context.Background(), platformstorage.PutRequest{Body: bytes.NewReader([]byte("private")), Size: 7})
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		rootInfo, err := os.Stat(root)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0700), rootInfo.Mode().Perm())
		objectInfo, err := os.Stat(filepath.Join(root, object.Key))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), objectInfo.Mode().Perm())
	}
}

func TestM3B1LocalProviderRequiresValidInputs(t *testing.T) {
	t.Parallel()

	_, err := NewProvider("")
	require.ErrorIs(t, err, platformstorage.ErrInvalidProvider)
	provider, err := NewProvider(t.TempDir())
	require.NoError(t, err)
	_, err = provider.Put(context.Background(), platformstorage.PutRequest{})
	require.ErrorIs(t, err, platformstorage.ErrInvalidRequest)
	_, err = provider.Put(nil, platformstorage.PutRequest{Body: strings.NewReader("x")})
	require.ErrorIs(t, err, platformstorage.ErrInvalidContext)
	_, err = provider.Stat(context.Background(), "")
	require.ErrorIs(t, err, platformstorage.ErrInvalidKey)
	_, _, err = provider.Open(context.Background(), "")
	require.ErrorIs(t, err, platformstorage.ErrInvalidKey)
	require.Error(t, provider.Delete(nil, "key"))
	require.True(t, errors.Is(provider.Delete(nil, "key"), platformstorage.ErrInvalidContext))
}

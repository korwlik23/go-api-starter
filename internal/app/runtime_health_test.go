package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckStorageRootAcceptsDirectory(t *testing.T) {
	root := t.TempDir()

	require.NoError(t, checkStorageRoot(context.Background(), root))
}

func TestCheckStorageRootRejectsMissingOrNonDirectoryPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	require.Error(t, checkStorageRoot(context.Background(), root))

	file := filepath.Join(t.TempDir(), "media")
	require.NoError(t, os.WriteFile(file, []byte("not a directory"), 0o600))
	require.Error(t, checkStorageRoot(context.Background(), file))
}

func TestCheckStorageRootHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, checkStorageRoot(ctx, t.TempDir()), context.Canceled)
}

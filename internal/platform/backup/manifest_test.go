package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestManifestRoundTripVerifiesArtifact(t *testing.T) {
	root := t.TempDir()
	artifact := filepath.Join(root, "dump.sql")
	manifestPath := filepath.Join(root, "dump.manifest.json")
	require.NoError(t, os.WriteFile(artifact, []byte("CREATE TABLE starter;\n"), 0o600))

	manifest, err := CreateManifest(artifact, "postgres", time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NoError(t, WriteManifest(manifestPath, manifest))

	loaded, err := ReadManifest(manifestPath)
	require.NoError(t, err)
	require.NoError(t, Verify(loaded))

	require.NoError(t, os.WriteFile(artifact, []byte("tampered"), 0o600))
	require.ErrorIs(t, Verify(loaded), ErrChecksumMismatch)
}

func TestManifestRejectsUnsupportedDialectAndMalformedHash(t *testing.T) {
	root := t.TempDir()
	artifact := filepath.Join(root, "dump.sql")
	require.NoError(t, os.WriteFile(artifact, []byte("dump"), 0o600))

	_, err := CreateManifest(artifact, "sqlite", time.Now().UTC())
	require.ErrorIs(t, err, ErrInvalidManifest)

	manifest := Manifest{FormatVersion: 1, Dialect: "postgres", Artifact: artifact, SHA256: "not-a-hash", CreatedAt: time.Now().UTC()}
	require.ErrorIs(t, Verify(manifest), ErrInvalidManifest)
	require.False(t, errors.Is(Verify(manifest), ErrChecksumMismatch))
}

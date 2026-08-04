// Package backup provides portable, verifiable metadata for database dump
// artifacts. The actual dump can be produced by pg_dump, mysqldump, or a
// MariaDB-compatible tool; the manifest prevents restoring an altered or
// mismatched artifact by accident.
package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

var (
	ErrInvalidManifest  = errors.New("backup: invalid manifest")
	ErrChecksumMismatch = errors.New("backup: checksum mismatch")
)

type Manifest struct {
	FormatVersion int       `json:"format_version"`
	Dialect       string    `json:"dialect"`
	Artifact      string    `json:"artifact"`
	SHA256        string    `json:"sha256"`
	CreatedAt     time.Time `json:"created_at"`
}

func CreateManifest(path, dialect string, now time.Time) (Manifest, error) {
	dialect = strings.ToLower(strings.TrimSpace(dialect))
	if dialect != "postgres" && dialect != "mysql" && dialect != "mariadb" || strings.TrimSpace(path) == "" || now.IsZero() {
		return Manifest{}, ErrInvalidManifest
	}
	hash, err := hashFile(path)
	if err != nil {
		return Manifest{}, err
	}
	return Manifest{FormatVersion: 1, Dialect: dialect, Artifact: path, SHA256: hash, CreatedAt: now.UTC()}, nil
}

func WriteManifest(path string, manifest Manifest) error {
	if err := validateManifest(manifest); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return os.WriteFile(path, contents, 0o600)
}

func ReadManifest(path string) (Manifest, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return Manifest{}, ErrInvalidManifest
	}
	if err := validateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Verify(manifest Manifest) error {
	if err := validateManifest(manifest); err != nil {
		return err
	}
	hash, err := hashFile(manifest.Artifact)
	if err != nil {
		return err
	}
	if hash != manifest.SHA256 {
		return ErrChecksumMismatch
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	if manifest.FormatVersion != 1 || manifest.CreatedAt.IsZero() || manifest.Artifact == "" || len(manifest.SHA256) != 64 || manifest.Dialect == "" {
		return ErrInvalidManifest
	}
	if _, err := hex.DecodeString(manifest.SHA256); err != nil {
		return ErrInvalidManifest
	}
	return nil
}
func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("backup: hash artifact: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

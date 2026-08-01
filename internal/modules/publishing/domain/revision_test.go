package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestM3A3RevisionRequiresImmutableSnapshotIdentity(t *testing.T) {
	t.Parallel()

	revision, err := NewRevision(RevisionInput{
		ID:            "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		ContentID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		AuthorID:      "018f1f64-7b2a-7c10-8bf2-3c40d5f67588",
		SnapshotHash:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		SourceVersion: 2,
		CreatedAt:     time.Now().UTC(),
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), revision.SourceVersion)

	_, err = NewRevision(RevisionInput{SnapshotHash: "secret"})
	require.ErrorIs(t, err, ErrInvalidRevision)
}

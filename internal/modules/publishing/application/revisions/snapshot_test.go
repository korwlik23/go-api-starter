package revisions

import (
	"testing"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/stretchr/testify/require"
)

func TestM3A6SnapshotHashRoundTripAndMismatchProtection(t *testing.T) {
	t.Parallel()

	raw, hash, err := encodeSnapshot(Snapshot{
		Kind:       publishingdomain.KindPage,
		ContentKey: "home",
		LocaleID:   "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		Path:       "/home",
		Title:      "Home",
		Blocks: []publishingdomain.Block{{
			Type: publishingdomain.BlockText,
			Data: map[string]any{"text": "Welcome"},
		}},
		Status: publishingdomain.StatusDraft,
	})
	require.NoError(t, err)
	decoded, err := decodeSnapshot(raw, hash)
	require.NoError(t, err)
	require.Equal(t, "home", decoded.ContentKey)
	_, err = decodeSnapshot(raw, "0000000000000000000000000000000000000000000000000000000000000000")
	require.ErrorIs(t, err, ErrRevisionSnapshotMismatch)
}

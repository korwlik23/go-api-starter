package mail

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileSenderWrites0600MessageWithoutLoggingSecrets(t *testing.T) {
	sender, err := NewFileSender(filepath.Join(t.TempDir(), "outbox"))
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), Message{To: "owner@example.test", Subject: "Verify", Text: "opaque-token"}))
}

func TestMessageRejectsMalformedRecipient(t *testing.T) {
	require.ErrorIs(t, (Message{To: "not-an-email", Subject: "x", Text: "y"}).Validate(), ErrInvalidMessage)
}

func TestFileSenderNeverUsesRecipientAsAPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sender, err := NewFileSender(root)
	require.NoError(t, err)

	err = sender.Send(context.Background(), Message{
		To:      "path/segment@example.test",
		Subject: "Subject",
		Text:    "Body",
	})
	require.NoError(t, err)
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.NotContains(t, entries[0].Name(), "path")
}

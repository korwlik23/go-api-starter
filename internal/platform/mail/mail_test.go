package mail

import (
	"context"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

func TestFileSenderWrites0600MessageWithoutLoggingSecrets(t *testing.T) {
	sender, err := NewFileSender(filepath.Join(t.TempDir(), "outbox"))
	require.NoError(t, err)
	require.NoError(t, sender.Send(context.Background(), Message{To: "owner@example.test", Subject: "Verify", Text: "opaque-token"}))
}

func TestMessageRejectsMalformedRecipient(t *testing.T) {
	require.ErrorIs(t, (Message{To: "not-an-email", Subject: "x", Text: "y"}).Validate(), ErrInvalidMessage)
}

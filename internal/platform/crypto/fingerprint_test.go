package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFingerprinterIsStableAndDomainSeparated(t *testing.T) {
	key := bytes.Repeat([]byte{0x55}, 32)
	subject, err := NewFingerprinter(key, "identity.auth-attempt.subject.v1")
	require.NoError(t, err)
	ip, err := NewFingerprinter(key, "identity.auth-attempt.ip.v1")
	require.NoError(t, err)

	first := subject.Sum("user@example.com")
	second := subject.Sum("user@example.com")
	require.Equal(t, first, second)
	require.NotEqual(t, first, ip.Sum("user@example.com"))
	require.Len(t, first, 32)
}

func TestFingerprinterRejectsWeakConfiguration(t *testing.T) {
	_, err := NewFingerprinter([]byte("short"), "identity.auth-attempt.subject.v1")
	require.ErrorIs(t, err, ErrFingerprintKeyTooShort)

	_, err = NewFingerprinter(bytes.Repeat([]byte{0x55}, 32), "")
	require.ErrorIs(t, err, ErrFingerprintDomainRequired)
}

package mfa

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/stretchr/testify/require"
)

func TestEnrollCreatesEncryptedPendingSecretAndOneTimeProvisioningURI(t *testing.T) {
	now := time.Date(2026, time.July, 30, 7, 0, 0, 0, time.UTC)
	cipher := &envelopeCipherStub{envelope: platformcrypto.Envelope{
		KeyVersion: 2,
		Nonce:      bytes.Repeat([]byte{0x01}, 12),
		Ciphertext: []byte("encrypted-secret"),
	}}
	repository := &mfaEnrollmentRepositoryStub{}
	enroller, err := NewEnroller(EnrollerDependencies{
		Repository: repository,
		Cipher:     cipher,
		Random:     bytes.NewReader(bytes.Repeat([]byte{0x42}, TOTPSecretBytes)),
		NewID:      func() (string, error) { return "enrollment-id", nil },
		Now:        func() time.Time { return now },
		PendingTTL: 10 * time.Minute,
		Issuer:     "go-api-starter",
	})
	require.NoError(t, err)

	result, err := enroller.Enroll(
		context.Background(),
		"user-id",
		"user@example.com",
	)
	require.NoError(t, err)
	require.Equal(t, "enrollment-id", result.EnrollmentID)
	require.True(t, strings.HasPrefix(result.ProvisioningURI, "otpauth://totp/"))
	require.Contains(t, result.ProvisioningURI, "issuer=go-api-starter")
	require.Contains(t, result.ProvisioningURI, "secret=")
	require.Equal(t, identitydomain.MFAEnrollment{
		ID:         "enrollment-id",
		UserID:     "user-id",
		State:      identitydomain.MFAEnrollmentPending,
		KeyVersion: 2,
		Nonce:      bytes.Repeat([]byte{0x01}, 12),
		Ciphertext: []byte("encrypted-secret"),
		ExpiresAt:  now.Add(10 * time.Minute),
		CreatedAt:  now,
	}, repository.pending)
	require.Equal(
		t,
		[]byte("user-id\x00totp-secret\x00envelope-v1\x00enrollment-id"),
		cipher.associatedData,
	)
	require.NotContains(t, string(repository.pending.Ciphertext), result.ProvisioningURI)
	require.NotContains(t, string(repository.pending.Ciphertext), cipher.plaintext)
}

func TestEnrollReturnsNoSecretMaterialWhenPersistenceFails(t *testing.T) {
	repositoryErr := errors.New("database unavailable")
	enroller, err := NewEnroller(EnrollerDependencies{
		Repository: &mfaEnrollmentRepositoryStub{err: repositoryErr},
		Cipher:     &envelopeCipherStub{},
		Random:     bytes.NewReader(bytes.Repeat([]byte{0x42}, TOTPSecretBytes)),
		NewID:      func() (string, error) { return "enrollment-id", nil },
		Now:        time.Now,
		PendingTTL: time.Minute,
		Issuer:     "go-api-starter",
	})
	require.NoError(t, err)

	result, err := enroller.Enroll(context.Background(), "user-id", "user@example.com")
	require.ErrorIs(t, err, repositoryErr)
	require.Empty(t, result)
}

type envelopeCipherStub struct {
	envelope       platformcrypto.Envelope
	plaintext      string
	associatedData []byte
	err            error
}

func (cipher *envelopeCipherStub) Encrypt(
	plaintext []byte,
	associatedData []byte,
) (platformcrypto.Envelope, error) {
	cipher.plaintext = string(plaintext)
	cipher.associatedData = append([]byte(nil), associatedData...)
	return cipher.envelope, cipher.err
}

type mfaEnrollmentRepositoryStub struct {
	pending identitydomain.MFAEnrollment
	err     error
}

func (repository *mfaEnrollmentRepositoryStub) SavePending(
	_ context.Context,
	enrollment identitydomain.MFAEnrollment,
) error {
	repository.pending = enrollment
	return repository.err
}

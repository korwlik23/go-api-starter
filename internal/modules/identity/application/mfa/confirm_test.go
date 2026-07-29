package mfa

import (
	"bytes"
	"context"
	"testing"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/stretchr/testify/require"
)

func TestConfirmEnrollmentVerifiesTOTPAndPersistsRecoveryHashesAtomically(
	t *testing.T,
) {
	now := time.Unix(59, 0).UTC()
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	repository := &confirmationRepositoryStub{pending: identitydomain.MFAEnrollment{
		ID:         "enrollment-id",
		UserID:     "user-id",
		State:      identitydomain.MFAEnrollmentPending,
		KeyVersion: 2,
		Nonce:      bytes.Repeat([]byte{0x01}, 12),
		Ciphertext: []byte("encrypted-secret"),
		ExpiresAt:  now.Add(time.Minute),
		CreatedAt:  now.Add(-time.Minute),
	}}
	cipher := confirmationCipherStub{plaintext: []byte(secret)}
	codes := &confirmationCodeGeneratorStub{}
	confirmer, err := NewConfirmer(ConfirmerDependencies{
		Repository: repository,
		Cipher:     cipher,
		Codes:      codes,
		Now:        func() time.Time { return now },
		CodeCount:  10,
	})
	require.NoError(t, err)
	validCode := "287082"

	result, err := confirmer.Confirm(
		context.Background(),
		ConfirmCommand{
			UserID:       "user-id",
			EnrollmentID: "enrollment-id",
			Code:         validCode,
		},
	)

	require.NoError(t, err)
	require.Len(t, result.RecoveryCodes, 10)
	require.Equal(t, int64(1), repository.step)
	require.Len(t, repository.codes, 10)
	require.Equal(t, now, repository.confirmedAt)
	for index, code := range repository.codes {
		require.Equal(t, int64(3), code.HashKeyVersion)
		require.Equal(t, platformcrypto.RecoveryCodeHashVersion, code.HashVersion)
		require.Equal(t, "recovery-"+string(rune('a'+index)), result.RecoveryCodes[index])
	}
}

type confirmationRepositoryStub struct {
	pending     identitydomain.MFAEnrollment
	step        int64
	codes       []identitydomain.MFARecoveryCode
	confirmedAt time.Time
	err         error
}

func (stub *confirmationRepositoryStub) SavePending(
	context.Context,
	identitydomain.MFAEnrollment,
) error {
	return nil
}

func (stub *confirmationRepositoryStub) LoadPending(
	context.Context,
	string,
	string,
) (identitydomain.MFAEnrollment, error) {
	return stub.pending, stub.err
}

func (stub *confirmationRepositoryStub) Confirm(
	_ context.Context,
	_ string,
	_ string,
	step int64,
	codes []identitydomain.MFARecoveryCode,
	confirmedAt time.Time,
) error {
	stub.step = step
	stub.codes = append([]identitydomain.MFARecoveryCode(nil), codes...)
	stub.confirmedAt = confirmedAt
	return stub.err
}

type confirmationCipherStub struct {
	plaintext []byte
	err       error
}

func (stub confirmationCipherStub) Decrypt(
	platformcrypto.Envelope,
	[]byte,
) ([]byte, error) {
	return append([]byte(nil), stub.plaintext...), stub.err
}

type confirmationCodeGeneratorStub struct {
	index int
}

func (stub *confirmationCodeGeneratorStub) Generate(
	string,
) (platformcrypto.RecoveryCode, error) {
	raw := "recovery-" + string(rune('a'+stub.index))
	stub.index++
	return platformcrypto.RecoveryCode{
		Raw:            raw,
		Hash:           [32]byte{byte(stub.index)},
		HashVersion:    platformcrypto.RecoveryCodeHashVersion,
		HashKeyVersion: 3,
	}, nil
}

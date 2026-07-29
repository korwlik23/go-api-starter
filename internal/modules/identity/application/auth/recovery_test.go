package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/stretchr/testify/require"
)

func TestRequestRecoveryIssuesHashOnlyTokenAndEncryptedDelivery(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 4, 0, 0, 0, time.UTC)
	user := identitydomain.User{
		ID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		Email:  "person@example.com",
		Status: identitydomain.UserStatusActive,
	}
	users := &recoveryUserReaderStub{user: user}
	tokens := &recoveryTokenCodecStub{
		token: platformcrypto.BoundToken{
			Raw:            "token-id.raw-secret",
			Hash:           [32]byte{0x42},
			HashVersion:    1,
			HashKeyVersion: 3,
		},
	}
	encrypter := &recoveryDeliveryEncrypterStub{
		envelope: platformcrypto.Envelope{
			KeyVersion: 7,
			Nonce:      []byte("safe-nonce"),
			Ciphertext: []byte("encrypted-delivery"),
		},
	}
	repository := &oneTimeTokenIssuerSpy{}
	useCase, err := NewRequestRecovery(RequestRecoveryDependencies{
		Users:      users,
		Tokens:     tokens,
		Delivery:   encrypter,
		Repository: repository,
		NewID: func() (string, error) {
			return "token-id", nil
		},
		Now: func() time.Time { return now },
		TTL: 30 * time.Minute,
	})
	require.NoError(t, err)

	err = useCase.Execute(context.Background(), " Person@Example.COM ")

	require.NoError(t, err)
	require.Equal(t, "person@example.com", users.email)
	require.Equal(t, "password_reset", tokens.purpose)
	require.Equal(t, "token-id", tokens.tokenID)
	require.Equal(t, user.ID, tokens.userID)
	require.Equal(t, []byte("token-id.raw-secret"), encrypter.plaintext)
	require.NotContains(t, string(encrypter.associatedData), "raw-secret")
	require.Equal(t, identitydomain.TokenHash{0x42}, repository.token.TokenHash)
	require.Equal(t, int16(1), repository.token.HashVersion)
	require.Equal(t, int64(3), repository.token.HashKeyVersion)
	require.Equal(t, now.Add(30*time.Minute), repository.token.ExpiresAt)
	require.Equal(t, []byte("encrypted-delivery"), repository.delivery.Ciphertext)
	require.NotContains(t, repository.serialized(), "raw-secret")
}

func TestRequestRecoveryUsesGenericSuccessAndDummyHMACForUnknownOrDisabledUser(
	t *testing.T,
) {
	t.Parallel()

	testCases := []struct {
		name string
		user identitydomain.User
		err  error
	}{
		{name: "unknown", err: ports.ErrUserNotFound},
		{
			name: "disabled",
			user: identitydomain.User{
				ID:     "disabled-user",
				Email:  "person@example.com",
				Status: identitydomain.UserStatusDisabled,
			},
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			tokens := &recoveryTokenCodecStub{
				token: platformcrypto.BoundToken{
					Raw:            "dummy-id.dummy-secret",
					Hash:           [32]byte{0x24},
					HashVersion:    1,
					HashKeyVersion: 3,
				},
			}
			repository := &oneTimeTokenIssuerSpy{}
			useCase, err := NewRequestRecovery(RequestRecoveryDependencies{
				Users: &recoveryUserReaderStub{
					user: testCase.user,
					err:  testCase.err,
				},
				Tokens:     tokens,
				Delivery:   &recoveryDeliveryEncrypterStub{},
				Repository: repository,
				NewID: func() (string, error) {
					return "dummy-id", nil
				},
				Now: time.Now,
				TTL: 30 * time.Minute,
			})
			require.NoError(t, err)

			require.NoError(
				t,
				useCase.Execute(context.Background(), "person@example.com"),
			)
			require.Equal(t, 1, tokens.calls)
			require.Zero(t, repository.calls)
		})
	}
}

func TestRequestRecoveryReturnsIssuanceFailureWithoutRawToken(t *testing.T) {
	t.Parallel()

	persistenceFailure := errors.New("delivery transaction failed")
	repository := &oneTimeTokenIssuerSpy{err: persistenceFailure}
	useCase, err := NewRequestRecovery(RequestRecoveryDependencies{
		Users: &recoveryUserReaderStub{user: identitydomain.User{
			ID:     "user-id",
			Email:  "person@example.com",
			Status: identitydomain.UserStatusActive,
		}},
		Tokens: &recoveryTokenCodecStub{token: platformcrypto.BoundToken{
			Raw:            "token-id.raw-secret",
			Hash:           [32]byte{0x42},
			HashVersion:    1,
			HashKeyVersion: 3,
		}},
		Delivery: &recoveryDeliveryEncrypterStub{envelope: platformcrypto.Envelope{
			KeyVersion: 1,
			Nonce:      []byte("nonce"),
			Ciphertext: []byte("ciphertext"),
		}},
		Repository: repository,
		NewID:      func() (string, error) { return "token-id", nil },
		Now:        time.Now,
		TTL:        time.Minute,
	})
	require.NoError(t, err)

	err = useCase.Execute(context.Background(), "person@example.com")

	require.ErrorIs(t, err, persistenceFailure)
	require.NotContains(t, err.Error(), "raw-secret")
}

type recoveryUserReaderStub struct {
	user  identitydomain.User
	err   error
	email string
}

func (stub *recoveryUserReaderStub) FindByNormalizedEmail(
	_ context.Context,
	email string,
) (identitydomain.User, error) {
	stub.email = email
	return stub.user, stub.err
}

type recoveryTokenCodecStub struct {
	token   platformcrypto.BoundToken
	purpose string
	tokenID string
	userID  string
	calls   int
}

func (stub *recoveryTokenCodecStub) Generate(
	purpose string,
	tokenID string,
	userID string,
) (platformcrypto.BoundToken, error) {
	stub.calls++
	stub.purpose = purpose
	stub.tokenID = tokenID
	stub.userID = userID
	return stub.token, nil
}

type recoveryDeliveryEncrypterStub struct {
	envelope       platformcrypto.Envelope
	plaintext      []byte
	associatedData []byte
}

func (stub *recoveryDeliveryEncrypterStub) Encrypt(
	plaintext []byte,
	associatedData []byte,
) (platformcrypto.Envelope, error) {
	stub.plaintext = append([]byte(nil), plaintext...)
	stub.associatedData = append([]byte(nil), associatedData...)
	return stub.envelope, nil
}

type oneTimeTokenIssuerSpy struct {
	token    identitydomain.OneTimeToken
	delivery identitydomain.EncryptedTokenDelivery
	err      error
	calls    int
}

func (spy *oneTimeTokenIssuerSpy) Issue(
	_ context.Context,
	token identitydomain.OneTimeToken,
	delivery identitydomain.EncryptedTokenDelivery,
) error {
	spy.calls++
	spy.token = token
	spy.delivery = delivery
	return spy.err
}

func (spy *oneTimeTokenIssuerSpy) serialized() string {
	return string(spy.token.TokenHash[:]) +
		string(spy.delivery.Nonce) +
		string(spy.delivery.Ciphertext)
}

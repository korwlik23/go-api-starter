package crypto

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordHasherHashAndVerify(t *testing.T) {
	hasher, err := NewPasswordHasher(testPasswordPolicy())
	require.NoError(t, err)

	encoded, err := hasher.Hash("correct horse battery staple")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=1,p=1$"))

	matches, err := hasher.Verify("correct horse battery staple", encoded)
	require.NoError(t, err)
	require.True(t, matches)

	matches, err = hasher.Verify("wrong password", encoded)
	require.NoError(t, err)
	require.False(t, matches)
}

func TestPasswordHasherUsesFreshSalt(t *testing.T) {
	hasher, err := NewPasswordHasher(testPasswordPolicy())
	require.NoError(t, err)

	first, err := hasher.Hash("same password")
	require.NoError(t, err)
	second, err := hasher.Hash("same password")
	require.NoError(t, err)

	require.NotEqual(t, first, second)
}

func TestPasswordPolicyRejectsWeakOrUnsafeParameters(t *testing.T) {
	testCases := []PasswordPolicy{
		{MemoryKiB: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		{MemoryKiB: 64 * 1024, Iterations: 0, Parallelism: 1, SaltLength: 16, KeyLength: 32},
		{MemoryKiB: 64 * 1024, Iterations: 1, Parallelism: 0, SaltLength: 16, KeyLength: 32},
		{MemoryKiB: 64 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 15, KeyLength: 32},
		{MemoryKiB: 64 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 31},
	}

	for _, policy := range testCases {
		_, err := NewPasswordHasher(policy)
		require.ErrorIs(t, err, ErrInvalidPasswordPolicy)
	}
}

func TestPasswordHasherReturnsGenericErrorForInvalidHashes(t *testing.T) {
	hasher, err := NewPasswordHasher(testPasswordPolicy())
	require.NoError(t, err)

	invalidHashes := []string{
		"",
		"not-a-password-hash",
		"$argon2i$v=19$m=65536,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=16$m=65536,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=999999999,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=65536,t=1,p=1$invalid!$invalid!",
	}

	for _, encoded := range invalidHashes {
		_, err := hasher.Verify("password", encoded)
		require.True(t, errors.Is(err, ErrInvalidPasswordHash))
		require.Equal(t, ErrInvalidPasswordHash, err)
	}
}

func testPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MemoryKiB:   64 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

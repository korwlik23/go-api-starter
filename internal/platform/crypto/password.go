package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	minPasswordMemoryKiB   = 64 * 1024
	maxPasswordMemoryKiB   = 1024 * 1024
	maxPasswordIterations  = 10
	maxPasswordParallelism = 16
	minPasswordSaltLength  = 16
	maxPasswordSaltLength  = 64
	minPasswordKeyLength   = 32
	maxPasswordKeyLength   = 64
)

var (
	ErrInvalidPasswordPolicy = errors.New("invalid password hashing policy")
	ErrInvalidPasswordHash   = errors.New("invalid password hash")
)

type PasswordPolicy struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultPasswordPolicy is the Argon2id policy shared by runtime bootstrap and
// local seed tooling. Callers still choose the password; this only centralizes
// the hashing parameters so credentials cannot silently diverge between CLIs.
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MemoryKiB:   64 * 1024,
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

type PasswordHasher struct {
	policy PasswordPolicy
	random io.Reader
}

func NewPasswordHasher(policy PasswordPolicy) (*PasswordHasher, error) {
	return newPasswordHasher(policy, rand.Reader)
}

func newPasswordHasher(policy PasswordPolicy, random io.Reader) (*PasswordHasher, error) {
	if !validHashPolicy(policy) {
		return nil, ErrInvalidPasswordPolicy
	}
	return &PasswordHasher{policy: policy, random: random}, nil
}

func (hasher *PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, hasher.policy.SaltLength)
	if _, err := io.ReadFull(hasher.random, salt); err != nil {
		return "", err
	}

	derived := argon2.IDKey(
		[]byte(password),
		salt,
		hasher.policy.Iterations,
		hasher.policy.MemoryKiB,
		hasher.policy.Parallelism,
		hasher.policy.KeyLength,
	)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		hasher.policy.MemoryKiB,
		hasher.policy.Iterations,
		hasher.policy.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(derived),
	), nil
}

func (hasher *PasswordHasher) Verify(password, encoded string) (bool, error) {
	policy, salt, expected, ok := decodePasswordHash(encoded)
	if !ok {
		return false, ErrInvalidPasswordHash
	}

	actual := argon2.IDKey(
		[]byte(password),
		salt,
		policy.Iterations,
		policy.MemoryKiB,
		policy.Parallelism,
		uint32(len(expected)),
	)
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func validHashPolicy(policy PasswordPolicy) bool {
	return policy.MemoryKiB >= minPasswordMemoryKiB &&
		policy.MemoryKiB <= maxPasswordMemoryKiB &&
		policy.Iterations >= 1 &&
		policy.Iterations <= maxPasswordIterations &&
		policy.Parallelism >= 1 &&
		policy.Parallelism <= maxPasswordParallelism &&
		policy.SaltLength >= minPasswordSaltLength &&
		policy.SaltLength <= maxPasswordSaltLength &&
		policy.KeyLength >= minPasswordKeyLength &&
		policy.KeyLength <= maxPasswordKeyLength
}

func decodePasswordHash(encoded string) (PasswordPolicy, []byte, []byte, bool) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return PasswordPolicy{}, nil, nil, false
	}

	version, ok := parsePHCParameter(parts[2], "v", 8)
	if !ok || version != uint64(argon2.Version) {
		return PasswordPolicy{}, nil, nil, false
	}

	parameters := strings.Split(parts[3], ",")
	if len(parameters) != 3 {
		return PasswordPolicy{}, nil, nil, false
	}
	memory, memoryOK := parsePHCParameter(parameters[0], "m", 32)
	iterations, iterationsOK := parsePHCParameter(parameters[1], "t", 32)
	parallelism, parallelismOK := parsePHCParameter(parameters[2], "p", 8)
	if !memoryOK || !iterationsOK || !parallelismOK {
		return PasswordPolicy{}, nil, nil, false
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return PasswordPolicy{}, nil, nil, false
	}
	expected, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return PasswordPolicy{}, nil, nil, false
	}

	policy := PasswordPolicy{
		MemoryKiB:   uint32(memory),
		Iterations:  uint32(iterations),
		Parallelism: uint8(parallelism),
		SaltLength:  uint32(len(salt)),
		KeyLength:   uint32(len(expected)),
	}
	if !validDecodedPolicy(policy) {
		return PasswordPolicy{}, nil, nil, false
	}

	return policy, salt, expected, true
}

func validDecodedPolicy(policy PasswordPolicy) bool {
	return policy.MemoryKiB >= 8*1024 &&
		policy.MemoryKiB <= maxPasswordMemoryKiB &&
		policy.Iterations >= 1 &&
		policy.Iterations <= maxPasswordIterations &&
		policy.Parallelism >= 1 &&
		policy.Parallelism <= maxPasswordParallelism &&
		policy.SaltLength >= 8 &&
		policy.SaltLength <= maxPasswordSaltLength &&
		policy.KeyLength >= 16 &&
		policy.KeyLength <= maxPasswordKeyLength
}

func parsePHCParameter(value, name string, bitSize int) (uint64, bool) {
	prefix := name + "="
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return 0, false
	}

	parsed, err := strconv.ParseUint(strings.TrimPrefix(value, prefix), 10, bitSize)
	return parsed, err == nil
}

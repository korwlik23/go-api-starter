package mfa

import (
	"context"
	"encoding/base32"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
)

const TOTPSecretBytes = 20

var ErrInvalidMFAEnrollment = errors.New("invalid MFA enrollment")

type envelopeEncryptor interface {
	Encrypt(
		plaintext []byte,
		associatedData []byte,
	) (platformcrypto.Envelope, error)
}

type EnrollerDependencies struct {
	Repository ports.MFAEnrollmentRepository
	Cipher     envelopeEncryptor
	Random     io.Reader
	NewID      func() (string, error)
	Now        func() time.Time
	PendingTTL time.Duration
	Issuer     string
}

type EnrollmentResult struct {
	EnrollmentID    string
	ProvisioningURI string
}

type Enroller struct {
	dependencies EnrollerDependencies
}

func NewEnroller(dependencies EnrollerDependencies) (*Enroller, error) {
	if dependencies.Repository == nil ||
		dependencies.Cipher == nil ||
		dependencies.Random == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil ||
		dependencies.PendingTTL <= 0 ||
		strings.TrimSpace(dependencies.Issuer) == "" {
		return nil, ErrInvalidMFAEnrollment
	}
	return &Enroller{dependencies: dependencies}, nil
}

func (enroller *Enroller) Enroll(
	ctx context.Context,
	userID string,
	email string,
) (EnrollmentResult, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(email) == "" {
		return EnrollmentResult{}, ErrInvalidMFAEnrollment
	}

	enrollmentID, err := enroller.dependencies.NewID()
	if err != nil {
		return EnrollmentResult{}, err
	}
	secretBytes := make([]byte, TOTPSecretBytes)
	if _, err := io.ReadFull(enroller.dependencies.Random, secretBytes); err != nil {
		return EnrollmentResult{}, err
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)
	associatedData := enrollmentAssociatedData(userID, enrollmentID)
	envelope, err := enroller.dependencies.Cipher.Encrypt([]byte(secret), associatedData)
	if err != nil {
		return EnrollmentResult{}, err
	}

	now := enroller.dependencies.Now()
	pending := identitydomain.MFAEnrollment{
		ID:         enrollmentID,
		UserID:     userID,
		State:      identitydomain.MFAEnrollmentPending,
		KeyVersion: envelope.KeyVersion,
		Nonce:      append([]byte(nil), envelope.Nonce...),
		Ciphertext: append([]byte(nil), envelope.Ciphertext...),
		ExpiresAt:  now.Add(enroller.dependencies.PendingTTL),
		CreatedAt:  now,
	}
	if err := enroller.dependencies.Repository.SavePending(ctx, pending); err != nil {
		return EnrollmentResult{}, err
	}

	return EnrollmentResult{
		EnrollmentID:    enrollmentID,
		ProvisioningURI: provisioningURI(enroller.dependencies.Issuer, email, secret),
	}, nil
}

func enrollmentAssociatedData(userID, enrollmentID string) []byte {
	return []byte(userID + "\x00totp-secret\x00envelope-v1\x00" + enrollmentID)
}

func provisioningURI(issuer, email, secret string) string {
	uri := url.URL{
		Scheme: "otpauth",
		Host:   "totp",
		Path:   "/" + issuer + ":" + email,
	}
	query := uri.Query()
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", strconv.Itoa(6))
	query.Set("period", strconv.Itoa(30))
	uri.RawQuery = query.Encode()
	return uri.String()
}

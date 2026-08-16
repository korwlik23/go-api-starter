package seed

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Profile controls the cumulative fixture set written by the local seed CLI.
type Profile string

const (
	ProfileFoundation       Profile = "foundation"
	ProfilePermissionMatrix Profile = "permission-matrix"
	ProfileDemoCMS          Profile = "demo-cms"
	ProfileFullLocal        Profile = "full-local"
)

var (
	ErrInvalidConfig       = errors.New("seed: invalid configuration")
	ErrProductionForbidden = errors.New("seed: production environment is not allowed")
	ErrUnknownProfile      = errors.New("seed: unknown profile")
)

type LookupFunc func(string) (string, bool)

// Config contains seed-only values. Database/runtime configuration remains in
// internal/platform/config so the seed command uses the same connection rules
// as the API and migration commands.
type Config struct {
	Environment string
	Profile     Profile
	AccountID   string
	AccountSlug string
	Email       string
	Password    string
}

func (profile Profile) Includes(required Profile) bool {
	levels := map[Profile]int{
		ProfileFoundation:       1,
		ProfilePermissionMatrix: 2,
		ProfileDemoCMS:          3,
		ProfileFullLocal:        4,
	}
	current, currentOK := levels[profile]
	wanted, wantedOK := levels[required]
	return currentOK && wantedOK && current >= wanted
}

func ParseConfig(lookup LookupFunc) (Config, error) {
	if lookup == nil {
		return Config{}, ErrInvalidConfig
	}
	environment := strings.ToLower(strings.TrimSpace(valueOr(lookup, "APP_ENV")))
	if environment != "development" && environment != "test" && environment != "local" {
		if environment == "production" || environment == "staging" {
			return Config{}, ErrProductionForbidden
		}
		return Config{}, fmt.Errorf("%w: APP_ENV must be development, local, or test", ErrInvalidConfig)
	}
	profile := Profile(strings.ToLower(strings.TrimSpace(valueOr(lookup, "SEED_PROFILE"))))
	if !profile.Includes(ProfileFoundation) {
		return Config{}, fmt.Errorf("%w: %q", ErrUnknownProfile, profile)
	}
	accountID := strings.TrimSpace(valueOr(lookup, "SEED_ACCOUNT_ID"))
	if accountID == "" {
		accountID = NewIDs().Account
	}
	if _, err := uuid.Parse(accountID); err != nil {
		return Config{}, fmt.Errorf("%w: SEED_ACCOUNT_ID must be a UUID", ErrInvalidConfig)
	}
	return Config{
		Environment: environment,
		Profile:     profile,
		AccountID:   accountID,
		AccountSlug: firstNonEmpty(valueOr(lookup, "SEED_ACCOUNT_SLUG"), "personal"),
		Email:       firstNonEmpty(valueOr(lookup, "SEED_EMAIL"), valueOr(lookup, "BOOTSTRAP_EMAIL"), "owner@example.com"),
		Password:    firstNonEmpty(valueOr(lookup, "SEED_PASSWORD"), valueOr(lookup, "BOOTSTRAP_PASSWORD")),
	}, nil
}

func valueOr(lookup LookupFunc, key string) string {
	value, _ := lookup(key)
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

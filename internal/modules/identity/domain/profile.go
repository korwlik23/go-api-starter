package domain

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidProfile = errors.New("identity: invalid profile")

type Profile struct {
	UserID          string
	DisplayName     string
	Timezone        string
	PreferredLocale string
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (profile Profile) Update(displayName, timezone, preferredLocale string, expectedVersion int64, now time.Time) (Profile, error) {
	displayName = strings.TrimSpace(displayName)
	timezone = strings.TrimSpace(timezone)
	preferredLocale = strings.TrimSpace(preferredLocale)
	if strings.TrimSpace(profile.UserID) == "" || profile.Version < 1 || expectedVersion != profile.Version || now.IsZero() || len(displayName) > 128 || len(timezone) > 64 || len(preferredLocale) > 32 {
		return Profile{}, ErrInvalidProfile
	}
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return Profile{}, ErrInvalidProfile
		}
	}
	return Profile{UserID: profile.UserID, DisplayName: displayName, Timezone: timezone, PreferredLocale: preferredLocale, Version: profile.Version + 1, CreatedAt: profile.CreatedAt, UpdatedAt: now.UTC()}, nil
}

package domain

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestProfileUpdateValidatesTimezoneAndOptimisticVersion(t *testing.T) {
	now := time.Now().UTC()
	profile := Profile{UserID: "user-1", DisplayName: "Admin", Timezone: "Asia/Bangkok", Version: 1, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	updated, err := profile.Update("Owner", "Europe/Berlin", "th", 1, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, int64(2), updated.Version)
	require.Equal(t, "Europe/Berlin", updated.Timezone)
	_, err = profile.Update("Owner", "Not/AZone", "th", 2, now.Add(time.Minute))
	require.ErrorIs(t, err, ErrInvalidProfile)
	_, err = profile.Update("Owner", "UTC", "th", 2, now.Add(time.Minute))
	require.ErrorIs(t, err, ErrInvalidProfile)
}

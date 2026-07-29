package gorm

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/stretchr/testify/require"
)

func TestNextAuthAttemptAppliesWindowThresholdAndSaturation(t *testing.T) {
	now := time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	policy := ports.AuthAttemptPolicy{
		Threshold:    3,
		Window:       15 * time.Minute,
		LockDuration: 30 * time.Minute,
	}

	record := authAttemptRecord{
		FailureCount:    2,
		WindowStartedAt: now.Add(-time.Minute),
		WindowExpiresAt: now.Add(time.Minute),
		LastFailedAt:    now.Add(-time.Second),
		CreatedAt:       now.Add(-time.Minute),
		UpdatedAt:       now.Add(-time.Second),
	}
	next, decision := nextAuthAttempt(record, now, policy)
	require.Equal(t, int64(3), next.FailureCount)
	require.Equal(t, now.Add(30*time.Minute), *next.LockedUntil)
	require.Equal(t, next.LockedUntil, decision.LockedUntil)
	require.Equal(t, now, next.UpdatedAt)

	record.FailureCount = math.MaxInt64
	record.LockedUntil = nil
	next, _ = nextAuthAttempt(record, now, policy)
	require.Equal(t, int64(math.MaxInt64), next.FailureCount)
}

func TestNextAuthAttemptDoesNotExtendActiveLock(t *testing.T) {
	now := time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	lockedUntil := now.Add(20 * time.Minute)
	record := authAttemptRecord{
		FailureCount:    5,
		WindowStartedAt: now.Add(-time.Minute),
		WindowExpiresAt: now.Add(time.Minute),
		LastFailedAt:    now.Add(-time.Second),
		LockedUntil:     &lockedUntil,
		CreatedAt:       now.Add(-time.Minute),
		UpdatedAt:       now.Add(-time.Second),
	}

	next, decision := nextAuthAttempt(record, now, ports.AuthAttemptPolicy{
		Threshold:    3,
		Window:       15 * time.Minute,
		LockDuration: time.Hour,
	})

	require.Equal(t, record, next)
	require.Equal(t, lockedUntil, *decision.LockedUntil)
}

func TestNextAuthAttemptStartsNewWindowAfterExpiry(t *testing.T) {
	now := time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	oldLock := now.Add(-time.Second)
	record := authAttemptRecord{
		FailureCount:    10,
		WindowStartedAt: now.Add(-time.Hour),
		WindowExpiresAt: now.Add(-time.Minute),
		LastFailedAt:    now.Add(-time.Minute),
		LockedUntil:     &oldLock,
		CreatedAt:       now.Add(-time.Hour),
		UpdatedAt:       now.Add(-time.Minute),
	}

	next, decision := nextAuthAttempt(record, now, ports.AuthAttemptPolicy{
		Threshold:    3,
		Window:       15 * time.Minute,
		LockDuration: 30 * time.Minute,
	})

	require.Equal(t, int64(1), next.FailureCount)
	require.Equal(t, now, next.WindowStartedAt)
	require.Equal(t, now.Add(15*time.Minute), next.WindowExpiresAt)
	require.Nil(t, next.LockedUntil)
	require.Nil(t, decision.LockedUntil)
	require.Equal(t, record.CreatedAt, next.CreatedAt)
}

func TestAuthAttemptRepositoryRecordsKeysInFixedOrderWithParameterizedSQL(t *testing.T) {
	db, mock := sessionRepositoryDatabase(t)
	repository, err := NewAuthAttemptRepository(db)
	require.NoError(t, err)
	now := time.Date(2026, time.July, 30, 5, 0, 0, 0, time.UTC)
	policy := ports.AuthAttemptPolicy{
		Threshold:    5,
		Window:       15 * time.Minute,
		LockDuration: 30 * time.Minute,
	}
	keys := []ports.AuthAttemptKey{
		{Type: ports.AuthAttemptKeySubject, Hash: [32]byte{0x11}},
		{Type: ports.AuthAttemptKeyIP, Hash: [32]byte{0x22}},
	}

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO auth_attempts`).
		WithArgs(
			"subject",
			keys[0].Hash[:],
			now,
			now.Add(policy.Window),
			now,
			now,
			now,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO auth_attempts`).
		WithArgs(
			"ip",
			keys[1].Hash[:],
			now,
			now.Add(policy.Window),
			now,
			now,
			now,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	decision, err := repository.RecordFailure(context.Background(), keys, now, policy)
	require.NoError(t, err)
	require.Nil(t, decision.LockedUntil)
	require.NoError(t, mock.ExpectationsWereMet())
}

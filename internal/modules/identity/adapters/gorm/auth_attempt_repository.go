package gorm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	gormdb "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maximumAuthAttemptCleanupBatch = 10_000

var (
	ErrInvalidAuthAttemptRepository = errors.New("invalid auth attempt repository")
	ErrInvalidAuthAttemptKeys       = errors.New("invalid auth attempt keys")
	ErrInvalidAuthAttemptCleanup    = errors.New("invalid auth attempt cleanup limit")
)

type AuthAttemptRepository struct {
	db *gormdb.DB
}

type authAttemptRecord struct {
	KeyType         string     `gorm:"column:key_type;primaryKey"`
	KeyHash         []byte     `gorm:"column:key_hash;primaryKey"`
	FailureCount    int64      `gorm:"column:failure_count"`
	WindowStartedAt time.Time  `gorm:"column:window_started_at"`
	WindowExpiresAt time.Time  `gorm:"column:window_expires_at"`
	LastFailedAt    time.Time  `gorm:"column:last_failed_at"`
	LockedUntil     *time.Time `gorm:"column:locked_until"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
}

func (authAttemptRecord) TableName() string {
	return "auth_attempts"
}

func NewAuthAttemptRepository(db *gormdb.DB) (*AuthAttemptRepository, error) {
	if db == nil {
		return nil, ErrInvalidAuthAttemptRepository
	}
	return &AuthAttemptRepository{db: db}, nil
}

func (repository *AuthAttemptRepository) Check(
	ctx context.Context,
	keys []ports.AuthAttemptKey,
	now time.Time,
) (ports.AuthAttemptDecision, error) {
	if !validAuthAttemptKeys(keys) {
		return ports.AuthAttemptDecision{}, ErrInvalidAuthAttemptKeys
	}

	var decision ports.AuthAttemptDecision
	for _, key := range keys {
		var record authAttemptRecord
		result := repository.db.WithContext(ctx).
			Where("key_type = ? AND key_hash = ?", string(key.Type), key.Hash[:]).
			Limit(1).
			Find(&record)
		if result.Error != nil {
			return ports.AuthAttemptDecision{}, fmt.Errorf("check auth attempt: %w", result.Error)
		}
		if result.RowsAffected == 1 && record.LockedUntil != nil && now.Before(*record.LockedUntil) {
			decision.LockedUntil = laterTime(decision.LockedUntil, record.LockedUntil)
		}
	}
	return decision, nil
}

func (repository *AuthAttemptRepository) RecordFailure(
	ctx context.Context,
	keys []ports.AuthAttemptKey,
	now time.Time,
	policy ports.AuthAttemptPolicy,
) (ports.AuthAttemptDecision, error) {
	if !validAuthAttemptKeys(keys) ||
		policy.Threshold < 2 ||
		policy.Threshold > math.MaxInt64 ||
		policy.Window <= 0 ||
		policy.LockDuration <= 0 {
		return ports.AuthAttemptDecision{}, ErrInvalidAuthAttemptKeys
	}

	var decision ports.AuthAttemptDecision
	err := repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		for _, key := range keys {
			keyDecision, err := recordAuthAttemptFailure(tx, key, now, policy)
			if err != nil {
				return err
			}
			decision.LockedUntil = laterTime(decision.LockedUntil, keyDecision.LockedUntil)
		}
		return nil
	})
	if err != nil {
		return ports.AuthAttemptDecision{}, fmt.Errorf("record auth attempt: %w", err)
	}
	return decision, nil
}

func (repository *AuthAttemptRepository) Reset(
	ctx context.Context,
	key ports.AuthAttemptKey,
) error {
	if key.Type != ports.AuthAttemptKeySubject {
		return ErrInvalidAuthAttemptKeys
	}

	err := repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		var record authAttemptRecord
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("key_type = ? AND key_hash = ?", string(key.Type), key.Hash[:]).
			Limit(1).
			Find(&record)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		return tx.
			Where("key_type = ? AND key_hash = ?", string(key.Type), key.Hash[:]).
			Delete(&authAttemptRecord{}).
			Error
	})
	if err != nil {
		return fmt.Errorf("reset auth attempt: %w", err)
	}
	return nil
}

func (repository *AuthAttemptRepository) CleanupExpired(
	ctx context.Context,
	now time.Time,
	limit int,
) (int64, error) {
	if limit < 1 || limit > maximumAuthAttemptCleanupBatch {
		return 0, ErrInvalidAuthAttemptCleanup
	}

	var records []authAttemptRecord
	err := repository.db.WithContext(ctx).
		Select("key_type", "key_hash").
		Where(
			"window_expires_at <= ? AND (locked_until IS NULL OR locked_until <= ?)",
			now,
			now,
		).
		Order("window_expires_at ASC").
		Limit(limit).
		Find(&records).
		Error
	if err != nil {
		return 0, fmt.Errorf("select expired auth attempts: %w", err)
	}

	var deleted int64
	err = repository.db.WithContext(ctx).Transaction(func(tx *gormdb.DB) error {
		for _, record := range records {
			result := tx.
				Where(
					"key_type = ? AND key_hash = ? AND window_expires_at <= ? "+
						"AND (locked_until IS NULL OR locked_until <= ?)",
					record.KeyType,
					record.KeyHash,
					now,
					now,
				).
				Delete(&authAttemptRecord{})
			if result.Error != nil {
				return result.Error
			}
			deleted += result.RowsAffected
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete expired auth attempts: %w", err)
	}
	return deleted, nil
}

func recordAuthAttemptFailure(
	tx *gormdb.DB,
	key ports.AuthAttemptKey,
	now time.Time,
	policy ports.AuthAttemptPolicy,
) (ports.AuthAttemptDecision, error) {
	inserted, err := insertInitialAuthAttempt(tx, key, now, policy.Window)
	if err != nil {
		return ports.AuthAttemptDecision{}, err
	}
	if inserted {
		return ports.AuthAttemptDecision{}, nil
	}

	var record authAttemptRecord
	result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("key_type = ? AND key_hash = ?", string(key.Type), key.Hash[:]).
		First(&record)
	if result.Error != nil {
		return ports.AuthAttemptDecision{}, result.Error
	}

	wasActivelyLocked := record.LockedUntil != nil && now.Before(*record.LockedUntil)
	next, decision := nextAuthAttempt(record, now, policy)
	if wasActivelyLocked {
		return decision, nil
	}
	err = tx.Model(&authAttemptRecord{}).
		Where("key_type = ? AND key_hash = ?", string(key.Type), key.Hash[:]).
		Updates(map[string]any{
			"failure_count":     next.FailureCount,
			"window_started_at": next.WindowStartedAt,
			"window_expires_at": next.WindowExpiresAt,
			"last_failed_at":    next.LastFailedAt,
			"locked_until":      next.LockedUntil,
			"updated_at":        next.UpdatedAt,
		}).
		Error
	return decision, err
}

func insertInitialAuthAttempt(
	tx *gormdb.DB,
	key ports.AuthAttemptKey,
	now time.Time,
	window time.Duration,
) (bool, error) {
	statement := ""
	switch tx.Dialector.Name() {
	case "postgres":
		statement = `INSERT INTO auth_attempts (` +
			`key_type, key_hash, failure_count, window_started_at, window_expires_at, ` +
			`last_failed_at, locked_until, created_at, updated_at` +
			`) VALUES (?, ?, 1, ?, ?, ?, NULL, ?, ?) ` +
			`ON CONFLICT (key_type, key_hash) DO NOTHING`
	case "mysql":
		statement = `INSERT INTO auth_attempts (` +
			`key_type, key_hash, failure_count, window_started_at, window_expires_at, ` +
			`last_failed_at, locked_until, created_at, updated_at` +
			`) VALUES (?, ?, 1, ?, ?, ?, NULL, ?, ?) ` +
			`ON DUPLICATE KEY UPDATE key_type = VALUES(key_type)`
	default:
		return false, ErrInvalidAuthAttemptRepository
	}

	result := tx.Exec(
		statement,
		string(key.Type),
		key.Hash[:],
		now,
		now.Add(window),
		now,
		now,
		now,
	)
	return result.RowsAffected == 1, result.Error
}

func nextAuthAttempt(
	record authAttemptRecord,
	now time.Time,
	policy ports.AuthAttemptPolicy,
) (authAttemptRecord, ports.AuthAttemptDecision) {
	if record.LockedUntil != nil && now.Before(*record.LockedUntil) {
		return record, ports.AuthAttemptDecision{LockedUntil: record.LockedUntil}
	}

	if !now.Before(record.WindowExpiresAt) {
		record.FailureCount = 1
		record.WindowStartedAt = now
		record.WindowExpiresAt = now.Add(policy.Window)
		record.LastFailedAt = now
		record.LockedUntil = nil
		record.UpdatedAt = now
		return record, ports.AuthAttemptDecision{}
	}

	if record.FailureCount < math.MaxInt64 {
		record.FailureCount++
	}
	record.LastFailedAt = now
	record.UpdatedAt = now
	if uint64(record.FailureCount) >= policy.Threshold {
		lockedUntil := now.Add(policy.LockDuration)
		record.LockedUntil = &lockedUntil
	}
	return record, ports.AuthAttemptDecision{LockedUntil: record.LockedUntil}
}

func validAuthAttemptKeys(keys []ports.AuthAttemptKey) bool {
	return len(keys) == 2 &&
		keys[0].Type == ports.AuthAttemptKeySubject &&
		keys[1].Type == ports.AuthAttemptKeyIP
}

func laterTime(current, candidate *time.Time) *time.Time {
	if candidate == nil {
		return current
	}
	if current == nil || candidate.After(*current) {
		value := *candidate
		return &value
	}
	return current
}

var _ ports.AuthAttemptRepository = (*AuthAttemptRepository)(nil)

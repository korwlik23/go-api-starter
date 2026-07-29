package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	gormdb "gorm.io/gorm"
)

const maximumPurgeBatch = 10_000

var ErrInvalidSessionRepository = errors.New("invalid session repository")

type SessionRepository struct {
	db *gormdb.DB
}

type sessionRecord struct {
	ID                   databaseID  `gorm:"column:id"`
	TokenHash            []byte      `gorm:"column:token_hash"`
	CSRFTokenHash        []byte      `gorm:"column:csrf_token_hash"`
	State                string      `gorm:"column:state"`
	UserID               *databaseID `gorm:"column:user_id"`
	AuthorizationVersion *int64      `gorm:"column:authorization_version"`
	IdleExpiresAt        time.Time   `gorm:"column:idle_expires_at"`
	AbsoluteExpiresAt    time.Time   `gorm:"column:absolute_expires_at"`
	RevokedAt            *time.Time  `gorm:"column:revoked_at"`
	CreatedAt            time.Time   `gorm:"column:created_at"`
}

func (sessionRecord) TableName() string {
	return "sessions"
}

func NewSessionRepository(db *gormdb.DB) (*SessionRepository, error) {
	if db == nil {
		return nil, ErrInvalidSessionRepository
	}
	return &SessionRepository{db: db}, nil
}

func (repository *SessionRepository) Create(
	ctx context.Context,
	session identitydomain.Session,
) error {
	sessionID, err := encodeDatabaseID(repository.db.Dialector.Name(), session.ID)
	if err != nil {
		return fmt.Errorf("encode session ID: %w", err)
	}
	var userID any
	if session.UserID != nil {
		userID, err = encodeDatabaseID(repository.db.Dialector.Name(), *session.UserID)
		if err != nil {
			return fmt.Errorf("encode session user ID: %w", err)
		}
	}

	values := map[string]any{
		"id":                    sessionID,
		"token_hash":            session.TokenHash[:],
		"csrf_token_hash":       session.CSRFTokenHash[:],
		"state":                 string(session.State),
		"user_id":               userID,
		"authorization_version": session.AuthorizationVersion,
		"idle_expires_at":       session.IdleExpiresAt,
		"absolute_expires_at":   session.AbsoluteExpiresAt,
		"revoked_at":            session.RevokedAt,
		"created_at":            session.CreatedAt,
	}
	if err := repository.db.WithContext(ctx).Table("sessions").Create(values).Error; err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (repository *SessionRepository) FindByTokenHash(
	ctx context.Context,
	hash identitydomain.TokenHash,
) (identitydomain.Session, error) {
	var record sessionRecord
	err := repository.db.WithContext(ctx).
		Where("token_hash = ?", hash[:]).
		First(&record).
		Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return identitydomain.Session{}, ports.ErrSessionNotFound
	}
	if err != nil {
		return identitydomain.Session{}, fmt.Errorf("find session: %w", err)
	}

	tokenHash, err := identitydomain.NewTokenHash(record.TokenHash)
	if err != nil {
		return identitydomain.Session{}, fmt.Errorf("decode session token hash: %w", err)
	}
	csrfTokenHash, err := identitydomain.NewTokenHash(record.CSRFTokenHash)
	if err != nil {
		return identitydomain.Session{}, fmt.Errorf("decode session CSRF token hash: %w", err)
	}

	return identitydomain.Session{
		ID:                   record.ID.String(),
		TokenHash:            tokenHash,
		CSRFTokenHash:        csrfTokenHash,
		State:                identitydomain.SessionState(record.State),
		UserID:               databaseIDStringPointer(record.UserID),
		AuthorizationVersion: record.AuthorizationVersion,
		IdleExpiresAt:        record.IdleExpiresAt,
		AbsoluteExpiresAt:    record.AbsoluteExpiresAt,
		RevokedAt:            record.RevokedAt,
		CreatedAt:            record.CreatedAt,
	}, nil
}

func (repository *SessionRepository) RevokeByTokenHash(
	ctx context.Context,
	hash identitydomain.TokenHash,
	revokedAt time.Time,
) (bool, error) {
	result := repository.db.WithContext(ctx).
		Model(&sessionRecord{}).
		Where("token_hash = ? AND revoked_at IS NULL", hash[:]).
		Update("revoked_at", revokedAt)
	if result.Error != nil {
		return false, fmt.Errorf("revoke session: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (repository *SessionRepository) PurgeExpired(
	ctx context.Context,
	now time.Time,
	limit int,
) (int64, error) {
	if limit < 1 || limit > maximumPurgeBatch {
		return 0, ports.ErrInvalidPurgeLimit
	}

	var ids []databaseID
	err := repository.db.WithContext(ctx).
		Model(&sessionRecord{}).
		Select("id").
		Where("absolute_expires_at <= ?", now).
		Order("absolute_expires_at ASC").
		Limit(limit).
		Pluck("id", &ids).
		Error
	if err != nil {
		return 0, fmt.Errorf("select expired sessions: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	encodedIDs := make([]any, 0, len(ids))
	for _, id := range ids {
		encoded, encodeErr := encodeDatabaseID(repository.db.Dialector.Name(), id.String())
		if encodeErr != nil {
			return 0, fmt.Errorf("encode expired session ID: %w", encodeErr)
		}
		encodedIDs = append(encodedIDs, encoded)
	}

	result := repository.db.WithContext(ctx).
		Where("id IN ? AND absolute_expires_at <= ?", encodedIDs, now).
		Delete(&sessionRecord{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func databaseIDStringPointer(value *databaseID) *string {
	if value == nil {
		return nil
	}
	converted := value.String()
	return &converted
}

var _ ports.SessionRepository = (*SessionRepository)(nil)

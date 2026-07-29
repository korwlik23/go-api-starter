package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidAssignmentRepository = errors.New(
	"invalid assignment repository",
)

type AssignmentRepository struct {
	db *gormdb.DB
}

func NewAssignmentRepository(
	db *gormdb.DB,
) (*AssignmentRepository, error) {
	if db == nil {
		return nil, ErrInvalidAssignmentRepository
	}
	return &AssignmentRepository{db: db}, nil
}

func (repository *AssignmentRepository) AssignMembershipRole(
	ctx context.Context,
	accountID string,
	membershipID string,
	roleID string,
	createdAt time.Time,
) error {
	ids, err := repository.encodeIDs(accountID, membershipID, roleID)
	if err != nil || createdAt.IsZero() {
		return ErrInvalidAssignmentRepository
	}
	err = repository.db.WithContext(ctx).
		Table("membership_roles").
		Create(map[string]any{
			"account_id":    ids[0],
			"membership_id": ids[1],
			"role_id":       ids[2],
			"created_at":    createdAt,
		}).
		Error
	if err != nil {
		return fmt.Errorf("%w: %v", ports.ErrAssignmentConflict, err)
	}
	return nil
}

func (repository *AssignmentRepository) RevokeMembershipRole(
	ctx context.Context,
	accountID string,
	membershipID string,
	roleID string,
) error {
	ids, err := repository.encodeIDs(accountID, membershipID, roleID)
	if err != nil {
		return ports.ErrAssignmentConflict
	}
	result := repository.db.WithContext(ctx).Exec(
		"DELETE FROM membership_roles "+
			"WHERE account_id = ? AND membership_id = ? AND role_id = ?",
		ids[0],
		ids[1],
		ids[2],
	)
	if result.Error != nil {
		return fmt.Errorf("revoke membership role: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.ErrAssignmentConflict
	}
	return nil
}

func (repository *AssignmentRepository) AssignSystemRole(
	ctx context.Context,
	userID string,
	roleID string,
	createdAt time.Time,
) error {
	ids, err := repository.encodeIDs(userID, roleID)
	if err != nil || createdAt.IsZero() {
		return ErrInvalidAssignmentRepository
	}
	err = repository.db.WithContext(ctx).
		Table("system_role_assignments").
		Create(map[string]any{
			"user_id":         ids[0],
			"role_owner_type": "system",
			"role_id":         ids[1],
			"created_at":      createdAt,
		}).
		Error
	if err != nil {
		return fmt.Errorf("%w: %v", ports.ErrAssignmentConflict, err)
	}
	return nil
}

func (repository *AssignmentRepository) RevokeSystemRole(
	ctx context.Context,
	userID string,
	roleID string,
) error {
	ids, err := repository.encodeIDs(userID, roleID)
	if err != nil {
		return ports.ErrAssignmentConflict
	}
	result := repository.db.WithContext(ctx).Exec(
		"DELETE FROM system_role_assignments "+
			"WHERE user_id = ? AND role_owner_type = ? AND role_id = ?",
		ids[0],
		"system",
		ids[1],
	)
	if result.Error != nil {
		return fmt.Errorf("revoke system role: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ports.ErrAssignmentConflict
	}
	return nil
}

func (repository *AssignmentRepository) encodeIDs(
	values ...string,
) ([]any, error) {
	encoded := make([]any, len(values))
	for index, value := range values {
		id, err := databaseid.Encode(repository.db.Dialector.Name(), value)
		if err != nil {
			return nil, err
		}
		encoded[index] = id
	}
	return encoded, nil
}

var _ ports.AssignmentRepository = (*AssignmentRepository)(nil)

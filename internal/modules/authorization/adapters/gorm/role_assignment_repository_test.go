package gorm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTransactionalRoleAssignmentRepositoryRejectsInvalidDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewTransactionalRoleAssignmentRepository(nil, "mysql")
	require.ErrorIs(t, err, ErrInvalidRoleAssignmentRepository)
}


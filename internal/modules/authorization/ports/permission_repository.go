package ports

import (
	"context"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
)

type GrantSource string

const (
	GrantSourceAccountRole GrantSource = "account_role"
	GrantSourceSystemRole  GrantSource = "system_role"
)

type EffectiveGrant struct {
	RoleID          string
	AccountID       string
	PermissionID    string
	Key             authorizationdomain.PermissionKey
	Source          GrantSource
	CatalogEpoch    int64
	CatalogChecksum [32]byte
}

type PermissionRepository interface {
	EffectiveGrants(
		ctx context.Context,
		userID string,
		accountID string,
	) ([]EffectiveGrant, error)
}

type ResourceOwnershipRepository interface {
	OwnerWithinAccount(
		ctx context.Context,
		accountID string,
		resourceID string,
	) (userID string, found bool, err error)
}

package application

import (
	"testing"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/stretchr/testify/require"
)

func TestPermissionCacheRequiresExactPrincipalAndCatalogVersions(t *testing.T) {
	cache := NewPermissionCache()
	permission, err := authorizationdomain.ParsePermissionKey(
		"content.articles.read.any",
	)
	require.NoError(t, err)
	version := PermissionCacheVersion{
		SystemAuthorizationVersion:  7,
		AccountAuthorizationVersion: 3,
		CatalogEpoch:                9,
		CatalogChecksum:             [32]byte{0x44},
	}
	require.True(t, cache.Put(
		"user-id",
		"account-id",
		version,
		[]authorizationdomain.PermissionKey{permission},
	))

	allowed, hit := cache.Has(
		"user-id",
		"account-id",
		version,
		permission,
	)
	require.True(t, hit)
	require.True(t, allowed)

	stale := version
	stale.CatalogEpoch++
	allowed, hit = cache.Has(
		"user-id",
		"account-id",
		stale,
		permission,
	)
	require.False(t, hit)
	require.False(t, allowed)

	allowed, hit = cache.Has(
		"user-id",
		"account-id",
		version,
		permission,
	)
	require.False(t, hit, "a version mismatch must evict the stale entry")
	require.False(t, allowed)
}

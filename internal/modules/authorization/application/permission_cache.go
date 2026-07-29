package application

import (
	"sync"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
)

type PermissionCacheVersion struct {
	SystemAuthorizationVersion  int64
	AccountAuthorizationVersion int64
	CatalogEpoch                int64
	CatalogChecksum             [32]byte
}

type permissionCacheKey struct {
	UserID    string
	AccountID string
}

type permissionCacheEntry struct {
	version     PermissionCacheVersion
	permissions map[authorizationdomain.PermissionKey]struct{}
}

type PermissionCache struct {
	mu      sync.Mutex
	entries map[permissionCacheKey]permissionCacheEntry
}

func NewPermissionCache() *PermissionCache {
	return &PermissionCache{
		entries: make(map[permissionCacheKey]permissionCacheEntry),
	}
}

func (cache *PermissionCache) Put(
	userID string,
	accountID string,
	version PermissionCacheVersion,
	permissions []authorizationdomain.PermissionKey,
) bool {
	if !validPermissionCacheVersion(userID, version) {
		return false
	}
	values := make(map[authorizationdomain.PermissionKey]struct{}, len(permissions))
	for _, permission := range permissions {
		if _, err := authorizationdomain.ParsePermissionKey(
			permission.String(),
		); err != nil {
			return false
		}
		values[permission] = struct{}{}
	}
	cache.mu.Lock()
	cache.entries[permissionCacheKey{
		UserID: userID, AccountID: accountID,
	}] = permissionCacheEntry{version: version, permissions: values}
	cache.mu.Unlock()
	return true
}

func (cache *PermissionCache) Has(
	userID string,
	accountID string,
	version PermissionCacheVersion,
	permission authorizationdomain.PermissionKey,
) (allowed bool, hit bool) {
	if !validPermissionCacheVersion(userID, version) {
		return false, false
	}
	key := permissionCacheKey{UserID: userID, AccountID: accountID}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	entry, ok := cache.entries[key]
	if !ok {
		return false, false
	}
	if entry.version != version {
		delete(cache.entries, key)
		return false, false
	}
	_, allowed = entry.permissions[permission]
	return allowed, true
}

func (cache *PermissionCache) Invalidate(userID, accountID string) {
	cache.mu.Lock()
	delete(cache.entries, permissionCacheKey{
		UserID: userID, AccountID: accountID,
	})
	cache.mu.Unlock()
}

func validPermissionCacheVersion(
	userID string,
	version PermissionCacheVersion,
) bool {
	return userID != "" &&
		version.SystemAuthorizationVersion > 0 &&
		version.AccountAuthorizationVersion > 0 &&
		version.CatalogEpoch > 0 &&
		version.CatalogChecksum != [32]byte{}
}

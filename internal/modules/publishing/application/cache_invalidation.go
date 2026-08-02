package application

import (
	"context"
	"errors"

	platformcache "github.com/korwlik23/go-api-starter/internal/platform/cache"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var ErrInvalidCacheInvalidation = errors.New("publishing: invalid cache invalidation")

// CacheInvalidationEvent identifies public representations affected by a
// committed write. Empty selectors are wildcards within the namespace.
type CacheInvalidationEvent struct {
	Namespace string
	AccountID string
	ContentID string
	LocaleID  string
	Path      string
}

type CacheInvalidator interface {
	InvalidateAfterCommit(context.Context, CacheInvalidationEvent) error
}

type CacheInvalidationService struct {
	cache platformcache.Cache
}

func NewCacheInvalidationService(cache platformcache.Cache) (*CacheInvalidationService, error) {
	if cache == nil {
		return nil, ErrInvalidCacheInvalidation
	}
	return &CacheInvalidationService{cache: cache}, nil
}

func (service *CacheInvalidationService) InvalidateAfterCommit(
	ctx context.Context,
	event CacheInvalidationEvent,
) error {
	if service == nil || service.cache == nil || ctx == nil {
		return ErrInvalidCacheInvalidation
	}
	if err := validateInvalidationEvent(event); err != nil {
		return err
	}
	_, err := service.cache.Invalidate(ctx, platformcache.Invalidation{
		Namespace: event.Namespace, AccountID: event.AccountID,
		ContentID: event.ContentID, Locale: event.LocaleID, Path: event.Path,
	})
	return err
}

// RunMutationWithInvalidation makes the cache side effect occur only after
// the transaction boundary returns success. A cache failure is returned to
// the caller, but it cannot roll back a database commit that already happened.
func RunMutationWithInvalidation(
	ctx context.Context,
	transactions database.Transactor,
	invalidator CacheInvalidator,
	event CacheInvalidationEvent,
	operation func(context.Context) error,
) error {
	if ctx == nil || operation == nil {
		return ErrInvalidCacheInvalidation
	}
	if err := validateInvalidationEvent(event); err != nil {
		return err
	}
	if transactions != nil {
		if err := transactions.WithinTransaction(ctx, operation); err != nil {
			return err
		}
	} else if err := operation(ctx); err != nil {
		return err
	}
	if invalidator == nil {
		return nil
	}
	return invalidator.InvalidateAfterCommit(ctx, event)
}

func validateInvalidationEvent(event CacheInvalidationEvent) error {
	return (platformcache.Invalidation{
		Namespace: event.Namespace, AccountID: event.AccountID,
		ContentID: event.ContentID, Locale: event.LocaleID, Path: event.Path,
	}).Validate()
}

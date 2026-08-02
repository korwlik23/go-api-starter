// Package local adapts the platform's private local object store to the media
// module's storage port. The media module deliberately does not expose public
// URLs or filesystem paths.
package local

import (
	"context"
	"io"

	mediaports "github.com/korwlik23/go-api-starter/internal/modules/media/ports"
	platformstorage "github.com/korwlik23/go-api-starter/internal/platform/storage"
)

type Provider struct {
	store *platformstorage.LocalProvider
}

func NewProvider(root string) (*Provider, error) {
	store, err := platformstorage.NewLocalProvider(root)
	if err != nil {
		return nil, err
	}
	return &Provider{store: store}, nil
}

func (provider *Provider) Put(
	ctx context.Context,
	request platformstorage.PutRequest,
) (platformstorage.ObjectInfo, error) {
	if provider == nil || provider.store == nil {
		return platformstorage.ObjectInfo{}, platformstorage.ErrInvalidProvider
	}
	return provider.store.Put(ctx, request)
}

func (provider *Provider) Stat(
	ctx context.Context,
	key string,
) (platformstorage.ObjectInfo, error) {
	if provider == nil || provider.store == nil {
		return platformstorage.ObjectInfo{}, platformstorage.ErrInvalidProvider
	}
	return provider.store.Stat(ctx, key)
}

func (provider *Provider) Delete(ctx context.Context, key string) error {
	if provider == nil || provider.store == nil {
		return platformstorage.ErrInvalidProvider
	}
	return provider.store.Delete(ctx, key)
}

func (provider *Provider) Open(
	ctx context.Context,
	key string,
) (io.ReadCloser, platformstorage.ObjectInfo, error) {
	if provider == nil || provider.store == nil {
		return nil, platformstorage.ObjectInfo{}, platformstorage.ErrInvalidProvider
	}
	return provider.store.Open(ctx, key)
}

// put is kept package-private for the adapter contract test. Public callers
// always use Put, which generates the opaque key; this helper only verifies
// that an existing opaque key is never replaced during compensation paths.
func (provider *Provider) put(
	ctx context.Context,
	key string,
	request platformstorage.PutRequest,
) (platformstorage.ObjectInfo, error) {
	if _, err := provider.Stat(ctx, key); err == nil {
		return platformstorage.ObjectInfo{}, platformstorage.ErrObjectExists
	}
	return provider.Put(ctx, request)
}

var _ mediaports.Storage = (*Provider)(nil)

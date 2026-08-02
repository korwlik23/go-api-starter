package ports

import (
	"context"
	"io"

	platformstorage "github.com/korwlik23/go-api-starter/internal/platform/storage"
)

// Storage is the media module's object-storage boundary. Implementations must
// keep object keys opaque to callers and must not expose a direct public URL.
type Storage interface {
	Put(context.Context, platformstorage.PutRequest) (platformstorage.ObjectInfo, error)
	Stat(context.Context, string) (platformstorage.ObjectInfo, error)
	Delete(context.Context, string) error
	Open(context.Context, string) (io.ReadCloser, platformstorage.ObjectInfo, error)
}

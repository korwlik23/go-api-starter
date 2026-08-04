package ports

import (
	"context"

	"github.com/korwlik23/go-api-starter/internal/modules/settings/domain"
)

var ErrNotFound = domain.ErrInvalidSetting

type Repository interface {
	List(context.Context) ([]domain.Setting, error)
	Find(context.Context, string) (domain.Setting, error)
	Save(context.Context, domain.Setting, int64) error
}

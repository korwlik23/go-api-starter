package ports

import (
	"context"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
)

type LocaleRepository interface {
	ListEnabled(ctx context.Context) ([]localizationdomain.Locale, error)
}

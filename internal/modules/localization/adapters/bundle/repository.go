package bundle

import (
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	bundledlocales "github.com/korwlik23/go-api-starter/locales"
)

type Repository struct{}

func (Repository) Category(
	locale string,
	category string,
) (map[string]string, error) {
	return bundledlocales.Category(locale, category)
}

var _ ports.BundleCatalogRepository = Repository{}

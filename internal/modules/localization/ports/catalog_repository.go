package ports

import (
	"context"
	"errors"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
)

var (
	ErrCatalogNotFound = errors.New("translation catalog not found")
	ErrCatalogConflict = errors.New("translation catalog version conflict")
)

type CatalogOverride struct {
	Key     string
	Value   string
	Version int64
}

type CatalogRepository interface {
	FindEnabledLocaleByTag(
		ctx context.Context,
		tag string,
	) (localizationdomain.Locale, error)
	FindEnabledLocaleByID(
		ctx context.Context,
		id string,
	) (localizationdomain.Locale, error)
	ListActiveOverrides(
		ctx context.Context,
		localeID string,
		category string,
	) ([]CatalogOverride, error)
}

type BundleCatalogRepository interface {
	Category(locale string, category string) (map[string]string, error)
}

type CatalogUpdateRepository interface {
	ReplaceCategory(
		ctx context.Context,
		localeID string,
		category string,
		entries map[string]string,
		expectedVersion int64,
		source string,
	) (int64, error)
}

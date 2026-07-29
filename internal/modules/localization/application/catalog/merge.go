package catalog

import (
	"context"
	"errors"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
)

const MaximumFallbackDepth = 32

var ErrInvalidCatalog = errors.New("invalid translation catalog")

type Result struct {
	Locale   string
	Category string
	Version  int64
	Entries  map[string]string
}

type Merge struct {
	catalog ports.CatalogRepository
	bundles ports.BundleCatalogRepository
}

func NewMerge(
	catalog ports.CatalogRepository,
	bundles ports.BundleCatalogRepository,
) (*Merge, error) {
	if catalog == nil || bundles == nil {
		return nil, ErrInvalidCatalog
	}
	return &Merge{catalog: catalog, bundles: bundles}, nil
}

func (useCase *Merge) Execute(
	ctx context.Context,
	localeTag string,
	category string,
) (Result, error) {
	if strings.TrimSpace(localeTag) == "" ||
		strings.TrimSpace(category) == "" {
		return Result{}, ErrInvalidCatalog
	}
	current, err := useCase.catalog.FindEnabledLocaleByTag(ctx, localeTag)
	if err != nil {
		return Result{}, err
	}
	chain := []struct {
		id  string
		tag string
	}{{id: current.ID, tag: current.Tag}}
	visited := map[string]struct{}{current.ID: {}}
	for depth := 0; current.FallbackID != ""; depth++ {
		if depth >= MaximumFallbackDepth {
			return Result{}, ErrInvalidCatalog
		}
		if _, cycle := visited[current.FallbackID]; cycle {
			return Result{}, ErrInvalidCatalog
		}
		visited[current.FallbackID] = struct{}{}
		current, err = useCase.catalog.FindEnabledLocaleByID(
			ctx,
			current.FallbackID,
		)
		if err != nil {
			return Result{}, ErrInvalidCatalog
		}
		chain = append(chain, struct {
			id  string
			tag string
		}{id: current.ID, tag: current.Tag})
	}

	entries := make(map[string]string)
	var version int64 = 1
	for index := len(chain) - 1; index >= 0; index-- {
		bundled, bundleErr := useCase.bundles.Category(
			chain[index].tag,
			category,
		)
		if bundleErr == nil {
			for key, value := range bundled {
				entries[key] = value
			}
		}
		overrides, overrideErr := useCase.catalog.ListActiveOverrides(
			ctx,
			chain[index].id,
			category,
		)
		if overrideErr != nil {
			return Result{}, overrideErr
		}
		for _, override := range overrides {
			if override.Key == "" || override.Value == "" || override.Version < 1 {
				return Result{}, ErrInvalidCatalog
			}
			entries[override.Key] = override.Value
			if index == 0 && override.Version > version {
				version = override.Version
			}
		}
	}
	if len(entries) == 0 {
		return Result{}, ports.ErrCatalogNotFound
	}
	return Result{
		Locale: localeTag, Category: category, Version: version, Entries: entries,
	}, nil
}

package application

import (
	"context"
	"errors"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
)

var ErrInvalidLocaleRegistry = errors.New("invalid locale registry")

type ListLocales struct {
	repository ports.LocaleRepository
}

func NewListLocales(
	repository ports.LocaleRepository,
) (*ListLocales, error) {
	if repository == nil {
		return nil, ErrInvalidLocaleRegistry
	}
	return &ListLocales{repository: repository}, nil
}

func (useCase *ListLocales) Execute(
	ctx context.Context,
) ([]localizationdomain.Locale, error) {
	locales, err := useCase.repository.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	defaults := 0
	seen := make(map[string]struct{}, len(locales))
	for _, locale := range locales {
		tag, normalizeErr := localizationdomain.NormalizeLocaleTag(locale.Tag)
		if normalizeErr != nil ||
			tag != locale.Tag ||
			!locale.Enabled ||
			locale.ID == "" ||
			locale.Version < 1 {
			return nil, ErrInvalidLocaleRegistry
		}
		if _, duplicate := seen[tag]; duplicate {
			return nil, ErrInvalidLocaleRegistry
		}
		seen[tag] = struct{}{}
		if locale.Default {
			defaults++
		}
	}
	if len(locales) == 0 || defaults != 1 {
		return nil, ErrInvalidLocaleRegistry
	}
	return locales, nil
}

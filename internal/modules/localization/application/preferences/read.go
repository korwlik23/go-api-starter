package preferences

import (
	"context"
	"errors"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
)

type Read struct {
	preferences ports.PreferenceRepository
	locales     ports.SelectableLocaleRepository
}

func NewRead(
	preferences ports.PreferenceRepository,
	locales ports.SelectableLocaleRepository,
) (*Read, error) {
	if preferences == nil || locales == nil {
		return nil, ErrInvalidLocalePreference
	}
	return &Read{preferences: preferences, locales: locales}, nil
}

func (useCase *Read) Execute(
	ctx context.Context,
	authenticatedUserID string,
) (localizationdomain.LocalePreference, error) {
	preference, err := useCase.preferences.Load(ctx, authenticatedUserID)
	if err != nil && !errors.Is(err, ports.ErrLocalePreferenceNotFound) {
		return localizationdomain.LocalePreference{}, err
	}
	if errors.Is(err, ports.ErrLocalePreferenceNotFound) ||
		preference.AllLanguages {
		locales, listErr := useCase.locales.ListSelectable(ctx)
		if listErr != nil {
			return localizationdomain.LocalePreference{}, listErr
		}
		ids := make([]string, 0, len(locales))
		for _, locale := range locales {
			if locale.Enabled && locale.UserSelectable {
				ids = append(ids, locale.ID)
			}
		}
		return localizationdomain.LocalePreference{
			AllLanguages: true,
			LocaleIDs:    ids,
			Version:      preference.Version,
		}, nil
	}
	return preference, nil
}

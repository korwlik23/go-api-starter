package preferences

import (
	"context"
	"errors"
	"slices"
	"strings"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
)

const MaximumLocaleSubset = 256

var ErrInvalidLocalePreference = errors.New("invalid locale preference")

type UpdateCommand struct {
	AllLanguages    bool
	LocaleIDs       []string
	ExpectedVersion int64
}

type Update struct {
	preferences ports.PreferenceRepository
	locales     ports.SelectableLocaleRepository
}

func NewUpdate(
	preferences ports.PreferenceRepository,
	locales ports.SelectableLocaleRepository,
) (*Update, error) {
	if preferences == nil || locales == nil {
		return nil, ErrInvalidLocalePreference
	}
	return &Update{preferences: preferences, locales: locales}, nil
}

func (useCase *Update) Execute(
	ctx context.Context,
	authenticatedUserID string,
	command UpdateCommand,
) (localizationdomain.LocalePreference, error) {
	if strings.TrimSpace(authenticatedUserID) == "" ||
		command.ExpectedVersion < 0 {
		return localizationdomain.LocalePreference{}, ErrInvalidLocalePreference
	}
	if command.AllLanguages {
		if len(command.LocaleIDs) != 0 {
			return localizationdomain.LocalePreference{}, ErrInvalidLocalePreference
		}
		return useCase.preferences.Save(
			ctx,
			authenticatedUserID,
			command.ExpectedVersion,
			localizationdomain.LocalePreference{AllLanguages: true},
		)
	}
	if len(command.LocaleIDs) < 1 ||
		len(command.LocaleIDs) > MaximumLocaleSubset {
		return localizationdomain.LocalePreference{}, ErrInvalidLocalePreference
	}
	unique := make(map[string]struct{}, len(command.LocaleIDs))
	for _, localeID := range command.LocaleIDs {
		if strings.TrimSpace(localeID) == "" {
			return localizationdomain.LocalePreference{}, ErrInvalidLocalePreference
		}
		if _, duplicate := unique[localeID]; duplicate {
			return localizationdomain.LocalePreference{}, ErrInvalidLocalePreference
		}
		unique[localeID] = struct{}{}
	}
	selectable, err := useCase.locales.ListSelectable(ctx)
	if err != nil {
		return localizationdomain.LocalePreference{}, err
	}
	eligible := make(map[string]struct{}, len(selectable))
	for _, locale := range selectable {
		if locale.Enabled && locale.UserSelectable {
			eligible[locale.ID] = struct{}{}
		}
	}
	for localeID := range unique {
		if _, ok := eligible[localeID]; !ok {
			return localizationdomain.LocalePreference{}, ErrInvalidLocalePreference
		}
	}
	localeIDs := append([]string(nil), command.LocaleIDs...)
	slices.Sort(localeIDs)
	return useCase.preferences.Save(
		ctx,
		authenticatedUserID,
		command.ExpectedVersion,
		localizationdomain.LocalePreference{
			LocaleIDs: localeIDs,
		},
	)
}

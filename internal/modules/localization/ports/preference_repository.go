package ports

import (
	"context"
	"errors"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
)

var (
	ErrLocalePreferenceNotFound = errors.New("locale preference not found")
	ErrLocalePreferenceConflict = errors.New("locale preference version conflict")
)

type PreferenceRepository interface {
	Load(
		ctx context.Context,
		userID string,
	) (localizationdomain.LocalePreference, error)
	Save(
		ctx context.Context,
		userID string,
		expectedVersion int64,
		preference localizationdomain.LocalePreference,
	) (localizationdomain.LocalePreference, error)
}

type SelectableLocaleRepository interface {
	ListSelectable(ctx context.Context) ([]localizationdomain.Locale, error)
}

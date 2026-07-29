package ports

import (
	"context"
	"errors"
	"time"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
)

var (
	ErrLocaleNotFound = errors.New("locale not found")
	ErrLocaleConflict = errors.New("locale version conflict")
)

type LocaleRepository interface {
	ListEnabled(ctx context.Context) ([]localizationdomain.Locale, error)
}

type CreateLocaleCommand struct {
	Locale localizationdomain.Locale
}

type UpdateLocaleCommand struct {
	ID              string
	ExpectedVersion int64
	Name            string
	Direction       localizationdomain.Direction
	FallbackID      string
	Enabled         bool
	UserSelectable  bool
	UpdatedAt       time.Time
}

type LocaleManagementRepository interface {
	FindByTag(
		ctx context.Context,
		tag string,
	) (localizationdomain.Locale, error)
	FindByID(
		ctx context.Context,
		localeID string,
	) (localizationdomain.Locale, error)
	Create(ctx context.Context, command CreateLocaleCommand) error
	Update(ctx context.Context, command UpdateLocaleCommand) (int64, error)
	SetDefault(
		ctx context.Context,
		localeID string,
		expectedVersion int64,
		updatedAt time.Time,
	) (DefaultLocaleResult, error)
}

type DefaultLocaleResult struct {
	PreviousDefaultLocaleID string
	Version                 int64
}

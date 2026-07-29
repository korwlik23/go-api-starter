package application

import (
	"context"
	"errors"
	"time"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
)

type initialLocaleRepository interface {
	ports.LocaleManagementRepository
	ports.LocaleRepository
}

type InitializeLocaleDependencies struct {
	Repository initialLocaleRepository
	NewID      func() (string, error)
	Now        func() time.Time
}

type InitializeLocales struct {
	dependencies InitializeLocaleDependencies
}

type bundledLocaleMetadata struct {
	name      string
	direction localizationdomain.Direction
}

var bundledLocaleRegistry = map[string]bundledLocaleMetadata{
	"en": {name: "English", direction: localizationdomain.DirectionLTR},
	"th": {name: "ไทย", direction: localizationdomain.DirectionLTR},
}

func NewInitializeLocales(
	dependencies InitializeLocaleDependencies,
) (*InitializeLocales, error) {
	if dependencies.Repository == nil ||
		dependencies.NewID == nil ||
		dependencies.Now == nil {
		return nil, ErrInvalidLocaleManagement
	}
	return &InitializeLocales{dependencies: dependencies}, nil
}

func (initializer *InitializeLocales) Execute(
	ctx context.Context,
	configuredTags []string,
	configuredDefault string,
) error {
	defaultTag, err := localizationdomain.NormalizeLocaleTag(configuredDefault)
	if err != nil || len(configuredTags) == 0 {
		return ErrInvalidLocaleManagement
	}
	normalized := make([]string, 0, len(configuredTags))
	seen := make(map[string]struct{}, len(configuredTags))
	defaultConfigured := false
	for _, configuredTag := range configuredTags {
		tag, normalizeErr := localizationdomain.NormalizeLocaleTag(configuredTag)
		metadata, bundled := bundledLocaleRegistry[tag]
		if normalizeErr != nil || !bundled || metadata.name == "" {
			return ErrInvalidLocaleManagement
		}
		if _, duplicate := seen[tag]; duplicate {
			return ErrInvalidLocaleManagement
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
		defaultConfigured = defaultConfigured || tag == defaultTag
	}
	if !defaultConfigured {
		return ErrInvalidLocaleManagement
	}

	now := initializer.dependencies.Now().UTC()
	if now.IsZero() {
		return ErrInvalidLocaleManagement
	}
	for _, tag := range normalized {
		_, findErr := initializer.dependencies.Repository.FindByTag(ctx, tag)
		switch {
		case findErr == nil:
			continue
		case !errors.Is(findErr, ports.ErrLocaleNotFound):
			return findErr
		}
		id, idErr := initializer.dependencies.NewID()
		if idErr != nil || id == "" {
			return ErrInvalidLocaleManagement
		}
		metadata := bundledLocaleRegistry[tag]
		createErr := initializer.dependencies.Repository.Create(
			ctx,
			ports.CreateLocaleCommand{Locale: localizationdomain.Locale{
				ID: id, Tag: tag, Name: metadata.name,
				Direction: metadata.direction,
				Enabled:   true, UserSelectable: true,
				Version: 1, CreatedAt: now, UpdatedAt: now,
			}},
		)
		if createErr != nil {
			return createErr
		}
	}

	enabled, err := initializer.dependencies.Repository.ListEnabled(ctx)
	if err != nil {
		return err
	}
	for _, locale := range enabled {
		if locale.Default {
			return nil
		}
	}
	defaultLocale, err := initializer.dependencies.Repository.FindByTag(
		ctx,
		defaultTag,
	)
	if err != nil ||
		!defaultLocale.Enabled ||
		defaultLocale.FallbackID != "" ||
		defaultLocale.Version < 1 {
		return ErrInvalidLocaleManagement
	}
	_, err = initializer.dependencies.Repository.SetDefault(
		ctx,
		defaultLocale.ID,
		defaultLocale.Version,
		now,
	)
	return err
}

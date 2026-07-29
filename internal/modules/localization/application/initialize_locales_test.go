package application

import (
	"context"
	"testing"
	"time"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/stretchr/testify/require"
)

func TestInitializeLocalesCreatesConfiguredRegistryOnce(t *testing.T) {
	repository := &initialLocaleRepositoryStub{
		localeManagementRepositoryStub: localeManagementRepositoryStub{
			byTag: map[string]localizationdomain.Locale{},
		},
	}
	nextID := 0
	initializer, err := NewInitializeLocales(InitializeLocaleDependencies{
		Repository: repository,
		NewID: func() (string, error) {
			nextID++
			return []string{"en-id", "th-id"}[nextID-1], nil
		},
		Now: func() time.Time {
			return time.Date(2026, 7, 30, 14, 0, 0, 0, time.UTC)
		},
	})
	require.NoError(t, err)

	err = initializer.Execute(
		context.Background(),
		[]string{"en", "th"},
		"en",
	)

	require.NoError(t, err)
	require.Len(t, repository.createdLocales, 2)
	require.Equal(t, "en-id", repository.defaultLocaleID)

	err = initializer.Execute(
		context.Background(),
		[]string{"en", "th"},
		"en",
	)
	require.NoError(t, err)
	require.Len(t, repository.createdLocales, 2)
}

func TestInitializeLocalesPreservesExistingSystemDefault(t *testing.T) {
	repository := &initialLocaleRepositoryStub{
		localeManagementRepositoryStub: localeManagementRepositoryStub{
			byTag: map[string]localizationdomain.Locale{
				"en": {
					ID: "en-id", Tag: "en", Enabled: true,
					UserSelectable: true, Version: 2,
				},
				"th": {
					ID: "th-id", Tag: "th", Enabled: true,
					UserSelectable: true, Default: true, Version: 5,
				},
			},
		},
	}
	initializer, err := NewInitializeLocales(InitializeLocaleDependencies{
		Repository: repository,
		NewID:      func() (string, error) { return "unused", nil },
		Now:        time.Now,
	})
	require.NoError(t, err)

	err = initializer.Execute(
		context.Background(),
		[]string{"en", "th"},
		"en",
	)

	require.NoError(t, err)
	require.Empty(t, repository.defaultLocaleID)
	require.Empty(t, repository.createdLocales)
}

type initialLocaleRepositoryStub struct {
	localeManagementRepositoryStub
	createdLocales  []localizationdomain.Locale
	defaultLocaleID string
}

func (stub *initialLocaleRepositoryStub) Create(
	_ context.Context,
	command ports.CreateLocaleCommand,
) error {
	stub.createdLocales = append(stub.createdLocales, command.Locale)
	stub.byTag[command.Locale.Tag] = command.Locale
	return nil
}

func (stub *initialLocaleRepositoryStub) ListEnabled(
	context.Context,
) ([]localizationdomain.Locale, error) {
	result := make([]localizationdomain.Locale, 0, len(stub.byTag))
	for _, locale := range stub.byTag {
		if locale.Enabled {
			result = append(result, locale)
		}
	}
	return result, nil
}

func (stub *initialLocaleRepositoryStub) SetDefault(
	_ context.Context,
	localeID string,
	expectedVersion int64,
	_ time.Time,
) (ports.DefaultLocaleResult, error) {
	stub.defaultLocaleID = localeID
	for tag, locale := range stub.byTag {
		if locale.ID == localeID {
			locale.Default = true
			locale.Version = expectedVersion + 1
			stub.byTag[tag] = locale
		}
	}
	return ports.DefaultLocaleResult{Version: expectedVersion + 1}, nil
}

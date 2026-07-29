package preferences

import (
	"context"
	"testing"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/stretchr/testify/require"
)

func TestUpdateUsesAuthenticatedUserAndValidatesSelectableSubset(t *testing.T) {
	preferences := &preferenceRepositoryStub{}
	useCase, err := NewUpdate(
		preferences,
		selectableLocaleRepositoryStub{locales: []localizationdomain.Locale{
			{ID: "th", Enabled: true, UserSelectable: true},
			{ID: "en", Enabled: true, UserSelectable: true},
			{ID: "ja", Enabled: true, UserSelectable: false},
		}},
	)
	require.NoError(t, err)

	result, err := useCase.Execute(context.Background(), "authenticated-user", UpdateCommand{
		LocaleIDs:       []string{"th", "en"},
		ExpectedVersion: 2,
	})

	require.NoError(t, err)
	require.Equal(t, "authenticated-user", preferences.userID)
	require.Equal(t, []string{"en", "th"}, preferences.preference.LocaleIDs)
	require.Equal(t, int64(3), result.Version)
}

func TestReadMissingPreferenceDynamicallyReturnsAllSelectableLocales(
	t *testing.T,
) {
	reader, err := NewRead(
		&preferenceRepositoryStub{loadErr: ports.ErrLocalePreferenceNotFound},
		selectableLocaleRepositoryStub{locales: []localizationdomain.Locale{
			{ID: "th", Enabled: true, UserSelectable: true},
			{ID: "en", Enabled: true, UserSelectable: true},
			{ID: "ja", Enabled: true, UserSelectable: false},
		}},
	)
	require.NoError(t, err)

	result, err := reader.Execute(context.Background(), "authenticated-user")

	require.NoError(t, err)
	require.True(t, result.AllLanguages)
	require.Equal(t, []string{"th", "en"}, result.LocaleIDs)
	require.Zero(t, result.Version)
}

type preferenceRepositoryStub struct {
	userID     string
	preference localizationdomain.LocalePreference
	loadErr    error
}

func (stub *preferenceRepositoryStub) Load(
	context.Context,
	string,
) (localizationdomain.LocalePreference, error) {
	return stub.preference, stub.loadErr
}

func (stub *preferenceRepositoryStub) Save(
	_ context.Context,
	userID string,
	expectedVersion int64,
	preference localizationdomain.LocalePreference,
) (localizationdomain.LocalePreference, error) {
	stub.userID = userID
	stub.preference = preference
	preference.Version = expectedVersion + 1
	return preference, nil
}

type selectableLocaleRepositoryStub struct {
	locales []localizationdomain.Locale
	err     error
}

func (stub selectableLocaleRepositoryStub) ListSelectable(
	context.Context,
) ([]localizationdomain.Locale, error) {
	return stub.locales, stub.err
}

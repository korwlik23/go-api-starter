package application

import (
	"context"
	"testing"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/stretchr/testify/require"
)

func TestListLocalesSupportsMoreThanTwoLanguagesAndOneDefault(t *testing.T) {
	repository := localeRepositoryStub{locales: []localizationdomain.Locale{
		{ID: "th-id", Tag: "th", Enabled: true, UserSelectable: true, Default: true, Version: 1},
		{ID: "en-id", Tag: "en", Enabled: true, UserSelectable: true, Version: 1},
		{ID: "ja-id", Tag: "ja", Enabled: true, UserSelectable: false, Version: 1},
		{ID: "zh-id", Tag: "zh-hant", Enabled: true, UserSelectable: true, Version: 1},
	}}
	useCase, err := NewListLocales(repository)
	require.NoError(t, err)

	locales, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, locales, 4)
	require.False(t, locales[2].UserSelectable)
}

type localeRepositoryStub struct {
	locales []localizationdomain.Locale
	err     error
}

func (stub localeRepositoryStub) ListEnabled(
	context.Context,
) ([]localizationdomain.Locale, error) {
	return stub.locales, stub.err
}

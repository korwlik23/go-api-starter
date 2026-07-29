package catalog

import (
	"context"
	"testing"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/stretchr/testify/require"
)

func TestCatalogUpdateRequiresSystemPermissionAndFixesEditorProvenance(
	t *testing.T,
) {
	repository := &catalogUpdateRepositoryStub{}
	useCase, err := NewUpdate(
		systemAuthorizerStub{allowed: true},
		catalogRepositoryStub{byTag: localizationdomain.Locale{
			ID: "th-id", Tag: "th", Enabled: true,
		}},
		repository,
	)
	require.NoError(t, err)

	version, err := useCase.Execute(
		context.Background(),
		"system-user",
		UpdateCommand{
			Locale: "th", Category: "common",
			Entries:         map[string]string{"actions.save": "จัดเก็บ"},
			ExpectedVersion: 2,
		},
	)

	require.NoError(t, err)
	require.Equal(t, int64(3), version)
	require.Equal(t, "editor", repository.source)
	require.Equal(t, "th-id", repository.localeID)
}

type systemAuthorizerStub struct {
	allowed bool
	err     error
}

func (stub systemAuthorizerStub) Allowed(
	context.Context,
	string,
	authorizationdomain.PermissionKey,
) (bool, error) {
	return stub.allowed, stub.err
}

type catalogUpdateRepositoryStub struct {
	localeID string
	source   string
}

func (stub *catalogUpdateRepositoryStub) ReplaceCategory(
	_ context.Context,
	localeID string,
	_ string,
	_ map[string]string,
	expectedVersion int64,
	source string,
) (int64, error) {
	stub.localeID = localeID
	stub.source = source
	return expectedVersion + 1, nil
}

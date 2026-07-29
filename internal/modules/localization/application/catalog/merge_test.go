package catalog

import (
	"context"
	"testing"

	localizationdomain "github.com/korwlik23/go-api-starter/internal/modules/localization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/localization/ports"
	"github.com/stretchr/testify/require"
)

func TestMergeOrdersDatabaseOverrideAboveBundleAndFallback(t *testing.T) {
	repository := catalogRepositoryStub{
		byTag: localizationdomain.Locale{
			ID: "th-id", Tag: "th", Enabled: true, FallbackID: "en-id",
		},
		byID: map[string]localizationdomain.Locale{
			"en-id": {ID: "en-id", Tag: "en", Enabled: true},
		},
		overrides: map[string][]ports.CatalogOverride{
			"th-id": {{Key: "actions.save", Value: "จัดเก็บ", Version: 4}},
		},
	}
	bundles := bundleRepositoryStub{categories: map[string]map[string]string{
		"en/common": {
			"actions.save":   "Save",
			"actions.cancel": "Cancel",
		},
		"th/common": {
			"actions.save": "บันทึก",
		},
	}}
	useCase, err := NewMerge(repository, bundles)
	require.NoError(t, err)

	result, err := useCase.Execute(context.Background(), "th", "common")

	require.NoError(t, err)
	require.Equal(t, "จัดเก็บ", result.Entries["actions.save"])
	require.Equal(t, "Cancel", result.Entries["actions.cancel"])
	require.Equal(t, int64(4), result.Version)
}

type catalogRepositoryStub struct {
	byTag     localizationdomain.Locale
	byID      map[string]localizationdomain.Locale
	overrides map[string][]ports.CatalogOverride
}

func (stub catalogRepositoryStub) FindEnabledLocaleByTag(
	context.Context,
	string,
) (localizationdomain.Locale, error) {
	return stub.byTag, nil
}

func (stub catalogRepositoryStub) FindEnabledLocaleByID(
	_ context.Context,
	id string,
) (localizationdomain.Locale, error) {
	return stub.byID[id], nil
}

func (stub catalogRepositoryStub) ListActiveOverrides(
	_ context.Context,
	localeID string,
	_ string,
) ([]ports.CatalogOverride, error) {
	return stub.overrides[localeID], nil
}

type bundleRepositoryStub struct {
	categories map[string]map[string]string
}

func (stub bundleRepositoryStub) Category(
	locale string,
	category string,
) (map[string]string, error) {
	return stub.categories[locale+"/"+category], nil
}

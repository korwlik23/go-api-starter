package contract_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestLocaleListContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/locales")
	require.NotNil(t, pathItem)
	operation := pathItem.Get
	require.NotNil(t, operation)
	require.Equal(t, "listLocales", operation.OperationID)
	require.Equal(t, []interface{}{"admin", "public", "site-server"}, operation.Extensions["x-surfaces"])
	require.Empty(t, *operation.Security)

	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	item := response.Value.Content.Get("application/json").Schema.Value.Properties["items"].Value.Items.Value
	require.ElementsMatch(t, []string{"id", "code", "name", "direction", "enabled", "selectable", "default"}, item.Required)
	require.Equal(t, []interface{}{"ltr", "rtl"}, item.Properties["direction"].Value.Enum)
}

func TestLocaleCreateContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	operation := document.Paths.Find("/locales").Post
	require.NotNil(t, operation)
	require.Equal(t, "createLocale", operation.OperationID)
	require.Equal(t, "localization.locales.create", operation.Extensions["x-required-permission"])
	require.NotContains(t, operation.Extensions, "x-required-role")
	require.Contains(t, (*operation.Security)[0], "CSRFToken")

	request := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"code", "name", "direction"}, request.Required)
	require.NotNil(t, operation.Responses.Value("201"))
	require.NotNil(t, operation.Responses.Value("409"))
}

func TestLocaleUpdateContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/locales/{locale_id}")
	require.NotNil(t, pathItem)
	operation := pathItem.Patch
	require.NotNil(t, operation)
	require.Equal(t, "updateLocale", operation.OperationID)
	require.Equal(t, "localization.locales.update", operation.Extensions["x-required-permission"])

	request := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"name", "direction", "expected_version"}, request.Required)
	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"id", "name", "direction", "version"}, schema.Required)
	require.NotNil(t, operation.Responses.Value("409"))
}

func TestLocaleStatusContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/locales/{locale_id}/status")
	require.NotNil(t, pathItem)
	operation := pathItem.Patch
	require.NotNil(t, operation)
	require.Equal(t, "updateLocaleStatus", operation.OperationID)
	request := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"enabled", "selectable", "expected_version"}, request.Required)
	require.NotNil(t, operation.Responses.Value("409"), "default/selectable locale invariants must return a stable conflict")
}

func TestLocaleDefaultContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/locales/{locale_id}/default")
	require.NotNil(t, pathItem)
	operation := pathItem.Put
	require.NotNil(t, operation)
	require.Equal(t, "setDefaultLocale", operation.OperationID)
	request := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"expected_version"}, request.Required)
	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"locale_id", "previous_default_locale_id", "version"}, schema.Required)
	require.NotNil(t, operation.Responses.Value("409"))
}

func TestLocalePreferenceContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/locales/preferences")
	require.NotNil(t, pathItem)
	operation := pathItem.Put
	require.NotNil(t, operation)
	require.Equal(t, "updateLocalePreferences", operation.OperationID)
	require.Equal(t, float64(32768), operation.Extensions["x-max-body-bytes"])

	request := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"all_languages", "locale_ids", "expected_version"}, request.Required)
	expectedVersion := request.Properties["expected_version"].Value
	require.Equal(t, float64(0), *expectedVersion.Min)
	localeIDs := request.Properties["locale_ids"].Value
	require.Equal(t, uint64(256), *localeIDs.MaxItems)
	require.True(t, localeIDs.UniqueItems)
	require.NotNil(t, operation.Responses.Value("409"))
}

func TestCatalogReadContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/localization/catalog")
	require.NotNil(t, pathItem)
	operation := pathItem.Get
	require.NotNil(t, operation)
	require.Equal(t, "getTranslationCatalog", operation.OperationID)
	require.Equal(t, []interface{}{"admin", "public", "site-server"}, operation.Extensions["x-surfaces"])

	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"locale", "category", "version", "entries"}, schema.Required)
	require.True(t, schema.Properties["entries"].Value.Type.Is("object"))
}

func TestCatalogUpdateContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	operation := document.Paths.Find("/localization/catalog").Put
	require.NotNil(t, operation)
	require.Equal(t, "updateTranslationCatalog", operation.OperationID)
	require.Equal(t, "localization.catalog.update", operation.Extensions["x-required-permission"])
	require.NotContains(t, operation.Extensions, "x-required-role")

	request := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"locale", "category", "entries", "expected_version"}, request.Required)
	response := operation.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"locale", "category", "version"}, schema.Required)
	require.NotNil(t, operation.Responses.Value("409"))
}

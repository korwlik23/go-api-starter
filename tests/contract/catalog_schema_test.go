package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBundledLocaleCatalogsHaveCategoryAndKeyParity(t *testing.T) {
	categories := []string{"common", "navigation", "auth", "validation"}
	keyPattern := regexp.MustCompile(
		`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`,
	)
	for _, category := range categories {
		en := readCatalogCategory(t, "en", category)
		th := readCatalogCategory(t, "th", category)
		enKeys := make([]string, 0, len(en))
		thKeys := make([]string, 0, len(th))
		for key, value := range en {
			require.True(t, keyPattern.MatchString(key), key)
			require.NotEmpty(t, value, key)
			enKeys = append(enKeys, key)
		}
		for key, value := range th {
			require.True(t, keyPattern.MatchString(key), key)
			require.NotEmpty(t, value, key)
			thKeys = append(thKeys, key)
		}
		slices.Sort(enKeys)
		slices.Sort(thKeys)
		require.Equal(t, enKeys, thKeys, category)
	}
}

func readCatalogCategory(
	t *testing.T,
	locale string,
	category string,
) map[string]string {
	t.Helper()
	path := filepath.Join(
		"..",
		"..",
		"locales",
		locale,
		category+".json",
	)
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	var catalog map[string]string
	require.NoError(t, json.Unmarshal(contents, &catalog))
	require.NotEmpty(t, catalog)
	return catalog
}

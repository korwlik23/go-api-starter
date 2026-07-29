package openapitools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterDocumentKeepsOnlySelectedSurfaceOperations(t *testing.T) {
	document := map[string]any{
		"openapi": "3.0.3",
		"paths": map[string]any{
			"/shared": map[string]any{
				"get": map[string]any{
					"operationId": "readShared",
					"x-surfaces":  []any{"admin", "public"},
				},
				"post": map[string]any{
					"operationId": "writeShared",
					"x-surfaces":  []any{"admin"},
				},
			},
			"/internal": map[string]any{
				"get": map[string]any{
					"operationId": "readInternal",
					"x-surfaces":  []any{"site-server"},
				},
			},
		},
	}

	filtered, err := FilterDocument(document, "public")
	require.NoError(t, err)
	paths := filtered["paths"].(map[string]any)
	require.NotContains(t, paths, "/internal")
	shared := paths["/shared"].(map[string]any)
	require.Contains(t, shared, "get")
	require.NotContains(t, shared, "post")

	originalShared := document["paths"].(map[string]any)["/shared"].(map[string]any)
	require.Contains(t, originalShared, "post", "filtering must not mutate the root bundle")
}

func TestFilterDocumentRejectsUnknownAndMissingSurfaceMetadata(t *testing.T) {
	document := map[string]any{
		"paths": map[string]any{
			"/unsafe": map[string]any{
				"get": map[string]any{"operationId": "unsafe"},
			},
		},
	}

	_, err := FilterDocument(document, "public")
	require.ErrorContains(t, err, "x-surfaces")
	_, err = FilterDocument(document, "unknown")
	require.ErrorContains(t, err, "unsupported surface")
}

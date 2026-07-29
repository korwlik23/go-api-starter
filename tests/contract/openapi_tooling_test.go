package contract_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type generatorConfig struct {
	Generate map[string]bool `yaml:"generate"`
	Output   string          `yaml:"output"`
	Package  string          `yaml:"package"`
}

func TestGeneratorConfigsDefineIsolatedSurfaces(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	testCases := []struct {
		name     string
		file     string
		pkg      string
		output   string
		generate map[string]bool
	}{
		{
			name:   "admin",
			file:   "admin.yaml",
			pkg:    "adminapi",
			output: "openapi.gen.go",
			generate: map[string]bool{
				"models": true,
				"client": true,
			},
		},
		{
			name:   "public",
			file:   "public.yaml",
			pkg:    "publicapi",
			output: "openapi.gen.go",
			generate: map[string]bool{
				"models": true,
				"client": true,
			},
		},
		{
			name:   "site server",
			file:   "site-server.yaml",
			pkg:    "siteapi",
			output: "openapi.gen.go",
			generate: map[string]bool{
				"models":        true,
				"gin-server":    true,
				"strict-server": true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			contents, err := os.ReadFile(filepath.Join(root, "openapi", "codegen", testCase.file))
			require.NoError(t, err)

			var config generatorConfig
			require.NoError(t, yaml.Unmarshal(contents, &config))
			require.Equal(t, testCase.pkg, config.Package)
			require.Equal(t, testCase.output, config.Output)
			require.Equal(t, testCase.generate, config.Generate)
		})
	}
}

func TestSchemaNamesDoNotCollideWithGeneratedResponseTypes(t *testing.T) {
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile(filepath.Join(
		"..", "..", "openapi", "dist", "admin.openapi.yaml",
	))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	for _, pathItem := range document.Paths.Map() {
		operations := []*openapi3.Operation{
			pathItem.Get, pathItem.Post, pathItem.Put, pathItem.Patch, pathItem.Delete,
		}
		for _, operation := range operations {
			if operation == nil || operation.OperationID == "" {
				continue
			}
			responseType := strings.ToUpper(operation.OperationID[:1]) + operation.OperationID[1:] + "Response"
			_, collision := document.Components.Schemas[responseType]
			require.Falsef(
				t,
				collision,
				"schema %q collides with the generated response wrapper for operation %q",
				responseType,
				operation.OperationID,
			)
		}
	}
}

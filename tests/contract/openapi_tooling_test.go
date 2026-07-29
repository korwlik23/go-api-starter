package contract_test

import (
	"os"
	"path/filepath"
	"testing"

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
			output: "../../internal/generated/admin/openapi.gen.go",
			generate: map[string]bool{
				"models": true,
				"client": true,
			},
		},
		{
			name:   "public",
			file:   "public.yaml",
			pkg:    "publicapi",
			output: "../../internal/generated/public/openapi.gen.go",
			generate: map[string]bool{
				"models": true,
				"client": true,
			},
		},
		{
			name:   "site server",
			file:   "site-server.yaml",
			pkg:    "siteapi",
			output: "../../internal/generated/site/openapi.gen.go",
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

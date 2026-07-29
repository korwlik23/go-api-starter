package config

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBundledExampleEnvironmentLoadsAndValidates(t *testing.T) {
	file, err := os.Open("../../../.env.example")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		require.True(t, ok, "invalid example environment entry")
		values[key] = value
	}
	require.NoError(t, scanner.Err())

	cfg, err := Load(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	require.NoError(t, err)
	require.NoError(t, Validate(cfg))
}

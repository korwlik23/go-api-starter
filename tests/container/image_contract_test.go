package container_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeImageIncludesOperationalCommands(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	dockerfile := string(contents)
	for _, expected := range []string{
		"-o /out/bootstrap ./cmd/bootstrap",
		"/out/bootstrap /app/bootstrap",
	} {
		if !strings.Contains(dockerfile, expected) {
			t.Errorf("Dockerfile is missing %q", expected)
		}
	}
}

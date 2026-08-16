package seed

import (
	"strings"
	"testing"
)

func TestPermissionMatrixUsesExplicitKeysAndCoversEightDemoUsers(t *testing.T) {
	if len(demoRoles) != 8 {
		t.Fatalf("demo users = %d, want 8", len(demoRoles))
	}
	for _, fixture := range demoRoles {
		if strings.ContainsAny(fixture.Key, "*?") || strings.ContainsAny(fixture.Name, "*?") {
			t.Fatalf("role %q contains wildcard semantics", fixture.Key)
		}
		for _, key := range append(append([]string{}, fixture.SystemKeys...), fixture.AccountKeys...) {
			if strings.ContainsAny(key, "*?") || !strings.Contains(key, ".") {
				t.Fatalf("permission %q is not an explicit key", key)
			}
		}
	}
}

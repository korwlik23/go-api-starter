package migrations_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM14AssignmentIDsMigrationExistsForEveryDialect(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	apiRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	for _, dialect := range []string{"postgres", "mariadb", "mysql"} {
		path := filepath.Join(
			apiRoot,
			"internal", "modules", "authorization", "migrations", dialect,
			"000006_add_assignment_ids.sql",
		)
		contents, err := os.ReadFile(path)
		require.NoError(t, err, dialect)
		require.Contains(t, string(contents), "assignment_id", dialect)
		require.Contains(t, string(contents), "UNIQUE", dialect)
		if dialect == "mariadb" || dialect == "mysql" {
			require.Contains(t, string(contents), "DROP INDEX", dialect)
		} else {
			require.Contains(t, string(contents), "DROP CONSTRAINT", dialect)
		}
	}
}

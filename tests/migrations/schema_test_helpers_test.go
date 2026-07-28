package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

type migrationColumn struct {
	dataType      string
	nullable      string
	defaultValue  sql.NullString
	maximumLength sql.NullInt64
}

func migrationColumns(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	table string,
) map[string]migrationColumn {
	t.Helper()

	schema := migrationCurrentSchema(t, ctx, db, cfg)
	rows, err := db.QueryContext(
		ctx,
		"SELECT column_name, data_type, is_nullable, column_default, "+
			"character_maximum_length FROM information_schema.columns "+
			"WHERE table_schema = "+migrationPlaceholder(cfg, 1)+
			" AND table_name = "+migrationPlaceholder(cfg, 2),
		schema,
		table,
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	columns := make(map[string]migrationColumn)
	for rows.Next() {
		var name string
		var column migrationColumn
		require.NoError(t, rows.Scan(
			&name,
			&column.dataType,
			&column.nullable,
			&column.defaultValue,
			&column.maximumLength,
		))
		column.dataType = strings.ToLower(column.dataType)
		columns[name] = column
	}
	require.NoError(t, rows.Err())
	return columns
}

func migrationHasExactConstraintColumns(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	table string,
	constraintType string,
	expectedColumns []string,
) bool {
	t.Helper()

	schema := migrationCurrentSchema(t, ctx, db, cfg)
	query := "SELECT tc.constraint_name, kcu.column_name " +
		"FROM information_schema.table_constraints tc " +
		"JOIN information_schema.key_column_usage kcu ON " +
		"tc.constraint_schema = kcu.constraint_schema AND " +
		"tc.table_name = kcu.table_name AND " +
		"tc.constraint_name = kcu.constraint_name " +
		"WHERE tc.table_schema = " + migrationPlaceholder(cfg, 1) +
		" AND tc.table_name = " + migrationPlaceholder(cfg, 2) +
		" AND tc.constraint_type = " + migrationPlaceholder(cfg, 3) +
		" ORDER BY tc.constraint_name, kcu.ordinal_position"
	rows, err := db.QueryContext(
		ctx,
		query,
		schema,
		table,
		constraintType,
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	constraintColumns := make(map[string][]string)
	for rows.Next() {
		var constraintName string
		var columnName string
		require.NoError(t, rows.Scan(&constraintName, &columnName))
		constraintColumns[constraintName] = append(
			constraintColumns[constraintName],
			columnName,
		)
	}
	require.NoError(t, rows.Err())
	for _, columns := range constraintColumns {
		if slices.Equal(columns, expectedColumns) {
			return true
		}
	}
	return false
}

func migrationTableExists(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	table string,
) bool {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = "+
			migrationPlaceholder(cfg, 1)+" AND table_name = "+migrationPlaceholder(cfg, 2),
		migrationCurrentSchema(t, ctx, db, cfg),
		table,
	).Scan(&count))
	return count == 1
}

func migrationCurrentSchema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
) string {
	t.Helper()

	query := "SELECT DATABASE()"
	if cfg.Driver == "postgres" {
		query = "SELECT current_schema()"
	}
	var schema string
	require.NoError(t, db.QueryRowContext(ctx, query).Scan(&schema))
	return schema
}

func migrationPlaceholders(cfg config.DatabaseConfig, count int) string {
	placeholders := make([]string, count)
	for index := range placeholders {
		placeholders[index] = migrationPlaceholder(cfg, index+1)
	}
	return strings.Join(placeholders, ", ")
}

func migrationPlaceholder(cfg config.DatabaseConfig, position int) string {
	if cfg.Driver == "postgres" {
		return fmt.Sprintf("$%d", position)
	}
	return "?"
}

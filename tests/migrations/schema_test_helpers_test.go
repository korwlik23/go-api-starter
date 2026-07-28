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

type migrationForeignKey struct {
	localColumns      []string
	referencedTable   string
	referencedColumns []string
	updateRule        string
	deleteRule        string
}

type migrationIndex struct {
	columns []string
	unique  bool
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

func migrationHasExactForeignKey(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	table string,
	expected migrationForeignKey,
) bool {
	t.Helper()

	foreignKeys := migrationForeignKeys(t, ctx, db, cfg, table)
	for _, foreignKey := range foreignKeys {
		if slices.Equal(foreignKey.localColumns, expected.localColumns) &&
			foreignKey.referencedTable == expected.referencedTable &&
			slices.Equal(foreignKey.referencedColumns, expected.referencedColumns) &&
			foreignKey.updateRule == expected.updateRule &&
			foreignKey.deleteRule == expected.deleteRule {
			return true
		}
	}
	return false
}

func migrationForeignKeys(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	table string,
) map[string]migrationForeignKey {
	t.Helper()

	schema := migrationCurrentSchema(t, ctx, db, cfg)
	query := "SELECT kcu.constraint_name, kcu.column_name, " +
		"kcu.referenced_table_name, kcu.referenced_column_name, " +
		"rc.update_rule, rc.delete_rule " +
		"FROM information_schema.key_column_usage kcu " +
		"JOIN information_schema.referential_constraints rc ON " +
		"rc.constraint_schema = kcu.constraint_schema AND " +
		"rc.constraint_name = kcu.constraint_name AND " +
		"rc.table_name = kcu.table_name " +
		"WHERE kcu.table_schema = " + migrationPlaceholder(cfg, 1) +
		" AND kcu.table_name = " + migrationPlaceholder(cfg, 2) +
		" AND kcu.referenced_table_name IS NOT NULL " +
		"ORDER BY kcu.constraint_name, kcu.ordinal_position"
	if cfg.Driver == "postgres" {
		query = "SELECT constraint_record.conname, local_attribute.attname, " +
			"foreign_table.relname, foreign_attribute.attname, " +
			"CASE constraint_record.confupdtype " +
			"WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' " +
			"WHEN 'c' THEN 'CASCADE' WHEN 'n' THEN 'SET NULL' " +
			"WHEN 'd' THEN 'SET DEFAULT' END, " +
			"CASE constraint_record.confdeltype " +
			"WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' " +
			"WHEN 'c' THEN 'CASCADE' WHEN 'n' THEN 'SET NULL' " +
			"WHEN 'd' THEN 'SET DEFAULT' END " +
			"FROM pg_catalog.pg_constraint constraint_record " +
			"JOIN pg_catalog.pg_class local_table ON " +
			"local_table.oid = constraint_record.conrelid " +
			"JOIN pg_catalog.pg_namespace local_namespace ON " +
			"local_namespace.oid = local_table.relnamespace " +
			"JOIN pg_catalog.pg_class foreign_table ON " +
			"foreign_table.oid = constraint_record.confrelid " +
			"JOIN LATERAL unnest(constraint_record.conkey) WITH ORDINALITY " +
			"AS local_key(attnum, position) ON TRUE " +
			"JOIN LATERAL unnest(constraint_record.confkey) WITH ORDINALITY " +
			"AS foreign_key(attnum, position) ON " +
			"foreign_key.position = local_key.position " +
			"JOIN pg_catalog.pg_attribute local_attribute ON " +
			"local_attribute.attrelid = local_table.oid AND " +
			"local_attribute.attnum = local_key.attnum " +
			"JOIN pg_catalog.pg_attribute foreign_attribute ON " +
			"foreign_attribute.attrelid = foreign_table.oid AND " +
			"foreign_attribute.attnum = foreign_key.attnum " +
			"WHERE constraint_record.contype = 'f' AND " +
			"local_namespace.nspname = " + migrationPlaceholder(cfg, 1) +
			" AND local_table.relname = " + migrationPlaceholder(cfg, 2) +
			" ORDER BY constraint_record.conname, local_key.position"
	}
	rows, err := db.QueryContext(ctx, query, schema, table)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	foreignKeys := make(map[string]migrationForeignKey)
	for rows.Next() {
		var constraintName string
		var localColumn string
		var referencedTable string
		var referencedColumn string
		var updateRule string
		var deleteRule string
		require.NoError(t, rows.Scan(
			&constraintName,
			&localColumn,
			&referencedTable,
			&referencedColumn,
			&updateRule,
			&deleteRule,
		))
		foreignKey := foreignKeys[constraintName]
		foreignKey.localColumns = append(foreignKey.localColumns, localColumn)
		foreignKey.referencedTable = referencedTable
		foreignKey.referencedColumns = append(
			foreignKey.referencedColumns,
			referencedColumn,
		)
		foreignKey.updateRule = strings.ToUpper(updateRule)
		foreignKey.deleteRule = strings.ToUpper(deleteRule)
		foreignKeys[constraintName] = foreignKey
	}
	require.NoError(t, rows.Err())
	return foreignKeys
}

func migrationHasExactIndexColumns(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	table string,
	expectedUnique bool,
	expectedColumns []string,
) bool {
	t.Helper()

	indexes := migrationIndexes(t, ctx, db, cfg, table)
	for _, index := range indexes {
		if index.unique == expectedUnique && slices.Equal(index.columns, expectedColumns) {
			return true
		}
	}
	return false
}

func migrationIndexes(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	table string,
) map[string]migrationIndex {
	t.Helper()

	schema := migrationCurrentSchema(t, ctx, db, cfg)
	query := "SELECT index_name, non_unique, column_name " +
		"FROM information_schema.statistics WHERE table_schema = " +
		migrationPlaceholder(cfg, 1) + " AND table_name = " +
		migrationPlaceholder(cfg, 2) + " ORDER BY index_name, seq_in_index"
	if cfg.Driver == "postgres" {
		query = "SELECT index_table.relname, index_record.indisunique, " +
			"column_record.attname FROM pg_catalog.pg_class table_record " +
			"JOIN pg_catalog.pg_namespace namespace_record ON " +
			"namespace_record.oid = table_record.relnamespace " +
			"JOIN pg_catalog.pg_index index_record ON " +
			"index_record.indrelid = table_record.oid " +
			"JOIN pg_catalog.pg_class index_table ON " +
			"index_table.oid = index_record.indexrelid " +
			"JOIN LATERAL unnest(index_record.indkey) WITH ORDINALITY " +
			"AS index_key(attnum, position) ON index_key.attnum > 0 " +
			"JOIN pg_catalog.pg_attribute column_record ON " +
			"column_record.attrelid = table_record.oid AND " +
			"column_record.attnum = index_key.attnum " +
			"WHERE namespace_record.nspname = " + migrationPlaceholder(cfg, 1) +
			" AND table_record.relname = " + migrationPlaceholder(cfg, 2) +
			" ORDER BY index_table.relname, index_key.position"
	}
	rows, err := db.QueryContext(ctx, query, schema, table)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	indexes := make(map[string]migrationIndex)
	for rows.Next() {
		var indexName string
		var unique bool
		var columnName string
		if cfg.Driver == "postgres" {
			require.NoError(t, rows.Scan(&indexName, &unique, &columnName))
		} else {
			var nonUnique int
			require.NoError(t, rows.Scan(&indexName, &nonUnique, &columnName))
			unique = nonUnique == 0
		}
		index := indexes[indexName]
		index.unique = unique
		index.columns = append(index.columns, columnName)
		indexes[indexName] = index
	}
	require.NoError(t, rows.Err())
	return indexes
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

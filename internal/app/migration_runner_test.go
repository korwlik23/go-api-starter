package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationStagesStopBeforeLocaleInitializationOnSchemaFailure(t *testing.T) {
	t.Parallel()

	schemaFailure := errors.New("schema failed")
	order := make([]string, 0, 2)
	err := runMigrationStages(
		context.Background(),
		func(context.Context) error {
			order = append(order, "schema")
			return schemaFailure
		},
		func(context.Context) error {
			order = append(order, "locales")
			return nil
		},
		func(context.Context) error {
			order = append(order, "modules")
			return nil
		},
	)

	require.ErrorIs(t, err, schemaFailure)
	require.Equal(t, []string{"schema"}, order)
}

func TestMigrationStagesInitializeLocalesAfterSchema(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 2)
	err := runMigrationStages(
		context.Background(),
		func(context.Context) error {
			order = append(order, "schema")
			return nil
		},
		func(context.Context) error {
			order = append(order, "locales")
			return nil
		},
		func(context.Context) error {
			order = append(order, "modules")
			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, []string{"schema", "locales", "modules"}, order)
}

func TestMigrationStagesStopBeforeModuleReconcileOnLocaleFailure(t *testing.T) {
	t.Parallel()

	localeFailure := errors.New("locale initialization failed")
	order := make([]string, 0, 3)
	err := runMigrationStages(
		context.Background(),
		func(context.Context) error {
			order = append(order, "schema")
			return nil
		},
		func(context.Context) error {
			order = append(order, "locales")
			return localeFailure
		},
		func(context.Context) error {
			order = append(order, "modules")
			return nil
		},
	)

	require.ErrorIs(t, err, localeFailure)
	require.Equal(t, []string{"schema", "locales"}, order)
}

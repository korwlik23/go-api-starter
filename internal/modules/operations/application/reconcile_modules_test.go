package application

import (
	"context"
	"errors"
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
	"github.com/stretchr/testify/require"
)

func TestReconcileModulesBuildsExactSecurityProjection(t *testing.T) {
	t.Parallel()

	repository := &reconcileRepositoryFake{}
	reconciler := newReconcilerForTest(t, repository)
	registry := reconcileRegistry(t, true)

	result, err := reconciler.Execute(context.Background(), ReconcileRequest{
		Registry: registry,
		Enabled:  map[string]bool{"identity": true, "publishing": false},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.Revision)
	require.Equal(t, int64(1), result.CatalogEpoch)
	require.Len(t, repository.applied.Modules, 2)
	require.Len(t, repository.applied.Permissions, 1)
	require.False(t, repository.applied.Permissions[0].Active)
	for _, module := range repository.applied.Modules {
		require.Len(t, module.ModuleChecksum, 32)
		require.Len(t, module.ConfigChecksum, 32)
		require.Len(t, module.CatalogChecksum, 32)
	}
}

func TestReconcileModulesPreservesRemovedPermissionAsInactive(t *testing.T) {
	t.Parallel()

	repository := &reconcileRepositoryFake{
		snapshot: ports.ReconcileSnapshot{
			Revision:     4,
			CatalogEpoch: 8,
			Permissions: []ports.PermissionRecord{{
				ID:        "permission-existing",
				Key:       "publishing.posts.read.any",
				ModuleID:  "publishing",
				Scope:     "any",
				Tier:      "optional",
				Delegable: true,
				Active:    true,
			}},
		},
	}
	reconciler := newReconcilerForTest(t, repository)
	registry := reconcileRegistry(t, false)

	result, err := reconciler.Execute(context.Background(), ReconcileRequest{
		Registry:         registry,
		Enabled:          map[string]bool{"identity": true, "publishing": false},
		ExpectedRevision: 4,
	})

	require.NoError(t, err)
	require.Equal(t, int64(5), result.Revision)
	require.Equal(t, int64(9), result.CatalogEpoch)
	require.Len(t, repository.applied.Permissions, 1)
	require.Equal(t, "permission-existing", repository.applied.Permissions[0].ID)
	require.False(t, repository.applied.Permissions[0].Active)
}

func TestReconcileModulesKeepsCatalogEpochOnIdempotentRerun(t *testing.T) {
	t.Parallel()

	firstRepository := &reconcileRepositoryFake{}
	reconciler := newReconcilerForTest(t, firstRepository)
	registry := reconcileRegistry(t, true)
	enabled := map[string]bool{"identity": true, "publishing": true}

	first, err := reconciler.Execute(context.Background(), ReconcileRequest{
		Registry: registry,
		Enabled:  enabled,
	})
	require.NoError(t, err)

	secondRepository := &reconcileRepositoryFake{
		snapshot: ports.ReconcileSnapshot{
			Revision:     firstRepository.applied.Revision,
			CatalogEpoch: firstRepository.applied.CatalogEpoch,
			Modules:      firstRepository.applied.Modules,
			Permissions:  firstRepository.applied.Permissions,
		},
	}
	reconciler = newReconcilerForTest(t, secondRepository)
	second, err := reconciler.Execute(context.Background(), ReconcileRequest{
		Registry:         registry,
		Enabled:          enabled,
		ExpectedRevision: first.Revision,
	})

	require.NoError(t, err)
	require.Equal(t, first.CatalogEpoch, second.CatalogEpoch)
	require.Equal(t, first.Revision+1, second.Revision)
}

func TestReconcileModulesRejectsStaleRevisionAndOwnerCollision(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		snapshot ports.ReconcileSnapshot
		expected int64
		errorIs  error
	}{
		"stale revision": {
			snapshot: ports.ReconcileSnapshot{Revision: 2, CatalogEpoch: 1},
			expected: 1,
			errorIs:  ErrStaleReconcile,
		},
		"owner collision": {
			snapshot: ports.ReconcileSnapshot{
				Revision:     1,
				CatalogEpoch: 1,
				Permissions: []ports.PermissionRecord{{
					ID:       "permission-existing",
					Key:      "publishing.posts.read.any",
					ModuleID: "identity",
					Scope:    "any",
					Tier:     "optional",
				}},
			},
			expected: 1,
			errorIs:  ErrPermissionOwnerCollision,
		},
	}

	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repository := &reconcileRepositoryFake{snapshot: test.snapshot}
			reconciler := newReconcilerForTest(t, repository)
			_, err := reconciler.Execute(context.Background(), ReconcileRequest{
				Registry:         reconcileRegistry(t, true),
				Enabled:          map[string]bool{"identity": true, "publishing": true},
				ExpectedRevision: test.expected,
			})
			require.ErrorIs(t, err, test.errorIs)
			require.False(t, repository.applyCalled)
		})
	}
}

func TestReconcileModulesRollsBackRepositoryFailure(t *testing.T) {
	t.Parallel()

	privateFailure := errors.New("private database failure")
	repository := &reconcileRepositoryFake{applyError: privateFailure}
	reconciler := newReconcilerForTest(t, repository)

	_, err := reconciler.Execute(context.Background(), ReconcileRequest{
		Registry: reconcileRegistry(t, true),
		Enabled:  map[string]bool{"identity": true, "publishing": true},
	})

	require.ErrorIs(t, err, ErrReconcileFailed)
	require.NotErrorIs(t, err, privateFailure)
	require.True(t, repository.rolledBack)
}

type reconcileRepositoryFake struct {
	snapshot    ports.ReconcileSnapshot
	applied     ports.ReconcileProjection
	applyError  error
	applyCalled bool
	rolledBack  bool
}

func (repository *reconcileRepositoryFake) WithinReconcile(
	ctx context.Context,
	operation func(context.Context, ports.ReconcileSession) error,
) error {
	session := &reconcileSessionFake{repository: repository}
	err := operation(ctx, session)
	if err != nil {
		repository.rolledBack = true
	}
	return err
}

type reconcileSessionFake struct {
	repository *reconcileRepositoryFake
}

func (session *reconcileSessionFake) Snapshot(
	context.Context,
) (ports.ReconcileSnapshot, error) {
	return session.repository.snapshot, nil
}

func (session *reconcileSessionFake) Apply(
	_ context.Context,
	projection ports.ReconcileProjection,
) error {
	session.repository.applyCalled = true
	session.repository.applied = projection
	return session.repository.applyError
}

func newReconcilerForTest(
	t *testing.T,
	repository ports.ModuleStateRepository,
) *ReconcileModules {
	t.Helper()

	sequence := 0
	reconciler, err := NewReconcileModules(ReconcileDependencies{
		Repository: repository,
		NewID: func() (string, error) {
			sequence++
			return "permission-new-" + string(rune('0'+sequence)), nil
		},
	})
	require.NoError(t, err)
	return reconciler
}

func reconcileRegistry(t *testing.T, includePermission bool) *modular.Registry {
	t.Helper()

	permissions := []modular.Permission(nil)
	routes := []modular.Route(nil)
	if includePermission {
		permissions = []modular.Permission{{
			Key:       "publishing.posts.read.any",
			Scope:     modular.ScopeAny,
			Tier:      modular.TierOptional,
			Delegable: true,
		}}
		routes = []modular.Route{{
			Method:     "GET",
			Path:       "/api/v1/publishing/posts",
			Permission: permissions[0].Key,
		}}
	}
	registry, err := modular.NewRegistry([]modular.Manifest{
		{
			ID:         "identity",
			Tier:       modular.TierCore,
			Migrations: modular.SQLMigrations(),
		},
		{
			ID:           "publishing",
			Tier:         modular.TierOptional,
			Disableable:  true,
			Dependencies: []string{"identity"},
			Permissions:  permissions,
			Routes:       routes,
			Migrations:   modular.SQLMigrations(),
		},
	})
	require.NoError(t, err)
	return registry
}

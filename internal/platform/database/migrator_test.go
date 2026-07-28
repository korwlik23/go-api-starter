package database

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestD3MigrationOrderAndLockScope(t *testing.T) {
	t.Parallel()

	events := &recordedMigrationEvents{}
	locker := &recordingMigrationLocker{events: events}
	runner := &recordingMigrationRunner{events: events}
	migrator, err := newMigrator(
		MigrationDialectPostgres,
		[]MigrationModule{
			d3MigrationModule("authorization", "accounts", "identity"),
			d3MigrationModule("audit", "identity"),
			d3MigrationModule("accounts", "identity"),
			d3MigrationModule("identity"),
		},
		locker,
		runner,
	)
	require.NoError(t, err)

	require.NoError(t, migrator.Up(context.Background()))
	require.Equal(
		t,
		[]string{
			"lock",
			"run:identity",
			"run:accounts",
			"run:audit",
			"run:authorization",
			"unlock",
		},
		events.Values(),
	)
}

func TestD3ConcurrentMigrationRunsRemainSerialized(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	releaseRunner := make(chan struct{})
	var releaseRunnerOnce sync.Once
	t.Cleanup(func() {
		releaseRunnerOnce.Do(func() {
			close(releaseRunner)
		})
	})
	locker := &serialMigrationLocker{
		arrived: make(chan struct{}, 2),
	}
	runner := &concurrentMigrationRunner{
		entered: make(chan struct{}),
		release: releaseRunner,
	}
	migrator, err := newMigrator(
		MigrationDialectPostgres,
		[]MigrationModule{d3MigrationModule("identity")},
		locker,
		runner,
	)
	require.NoError(t, err)

	results := make(chan error, 2)
	go func() {
		results <- migrator.Up(ctx)
	}()
	select {
	case <-runner.entered:
	case <-ctx.Done():
		t.Fatal("first migration run did not enter the runner")
	}
	select {
	case <-locker.arrived:
	case <-ctx.Done():
		t.Fatal("first migration run did not reach the locker")
	}

	go func() {
		results <- migrator.Up(ctx)
	}()
	select {
	case <-locker.arrived:
	case <-ctx.Done():
		t.Fatal("second migration run did not contend for the locker")
	}

	releaseRunnerOnce.Do(func() {
		close(releaseRunner)
	})

	for range 2 {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("serialized migration runs did not finish")
		}
	}
	require.Equal(t, int32(1), runner.maximumActive.Load())
	require.Equal(t, int32(2), runner.calls.Load())
}

func TestD3RejectsInvalidMigrationPlans(t *testing.T) {
	t.Parallel()

	validLocker := &recordingMigrationLocker{}
	validRunner := &recordingMigrationRunner{}
	tests := []struct {
		name    string
		dialect MigrationDialect
		modules []MigrationModule
		locker  migrationLocker
		runner  migrationModuleRunner
	}{
		{
			name:    "empty module plan",
			dialect: MigrationDialectPostgres,
			locker:  validLocker,
			runner:  validRunner,
		},
		{
			name:    "missing locker",
			dialect: MigrationDialectPostgres,
			modules: []MigrationModule{d3MigrationModule("identity")},
			runner:  validRunner,
		},
		{
			name:    "missing runner",
			dialect: MigrationDialectPostgres,
			modules: []MigrationModule{d3MigrationModule("identity")},
			locker:  validLocker,
		},
		{
			name:    "duplicate module",
			dialect: MigrationDialectPostgres,
			modules: []MigrationModule{
				d3MigrationModule("identity"),
				d3MigrationModule("identity"),
			},
			locker: validLocker,
			runner: validRunner,
		},
		{
			name:    "invalid module identifier",
			dialect: MigrationDialectPostgres,
			modules: []MigrationModule{d3MigrationModule("Identity/Admin")},
			locker:  validLocker,
			runner:  validRunner,
		},
		{
			name:    "module identifier exceeds table-name budget",
			dialect: MigrationDialectPostgres,
			modules: []MigrationModule{
				d3MigrationModule("a" + strings.Repeat("b", 40)),
			},
			locker: validLocker,
			runner: validRunner,
		},
		{
			name:    "missing selected migration bundle",
			dialect: MigrationDialectPostgres,
			modules: []MigrationModule{
				{
					ID: "identity",
					Bundles: map[MigrationDialect]fs.FS{
						MigrationDialectMariaDB: d3MigrationFS(),
					},
				},
			},
			locker: validLocker,
			runner: validRunner,
		},
		{
			name:    "unknown dependency",
			dialect: MigrationDialectPostgres,
			modules: []MigrationModule{
				d3MigrationModule("identity", "accounts"),
			},
			locker: validLocker,
			runner: validRunner,
		},
		{
			name:    "duplicate dependency",
			dialect: MigrationDialectPostgres,
			modules: []MigrationModule{
				d3MigrationModule("identity"),
				d3MigrationModule("accounts", "identity", "identity"),
			},
			locker: validLocker,
			runner: validRunner,
		},
		{
			name:    "self dependency",
			dialect: MigrationDialectPostgres,
			modules: []MigrationModule{
				d3MigrationModule("identity", "identity"),
			},
			locker: validLocker,
			runner: validRunner,
		},
		{
			name:    "dependency cycle",
			dialect: MigrationDialectPostgres,
			modules: []MigrationModule{
				d3MigrationModule("identity", "accounts"),
				d3MigrationModule("accounts", "identity"),
			},
			locker: validLocker,
			runner: validRunner,
		},
		{
			name:    "unsupported migration dialect",
			dialect: MigrationDialect("sqlite"),
			modules: []MigrationModule{d3MigrationModule("identity")},
			locker:  validLocker,
			runner:  validRunner,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			migrator, err := newMigrator(
				test.dialect,
				test.modules,
				test.locker,
				test.runner,
			)

			require.Nil(t, migrator)
			require.ErrorIs(t, err, ErrInvalidMigrationPlan)
		})
	}
}

func TestD3StopsAfterRedactedModuleFailureAndReleasesLock(t *testing.T) {
	t.Parallel()

	const privateMarker = "private-migration-driver-error"
	events := &recordedMigrationEvents{}
	locker := &recordingMigrationLocker{events: events}
	runner := &recordingMigrationRunner{
		events:     events,
		failModule: "accounts",
		failError:  errors.New(privateMarker),
	}
	migrator, err := newMigrator(
		MigrationDialectPostgres,
		[]MigrationModule{
			d3MigrationModule("authorization", "accounts"),
			d3MigrationModule("accounts"),
		},
		locker,
		runner,
	)
	require.NoError(t, err)

	err = migrator.Up(context.Background())

	require.ErrorIs(t, err, ErrMigrationFailed)
	require.NotContains(t, err.Error(), privateMarker)
	require.Contains(t, err.Error(), "accounts")
	require.Equal(
		t,
		[]string{"lock", "run:accounts", "unlock"},
		events.Values(),
	)
}

func TestD3RedactsLockFailureBeforeRunningModules(t *testing.T) {
	t.Parallel()

	const privateMarker = "private-lock-driver-error"
	runner := &recordingMigrationRunner{}
	migrator, err := newMigrator(
		MigrationDialectPostgres,
		[]MigrationModule{d3MigrationModule("identity")},
		&recordingMigrationLocker{lockError: errors.New(privateMarker)},
		runner,
	)
	require.NoError(t, err)

	err = migrator.Up(context.Background())

	require.ErrorIs(t, err, ErrMigrationLockUnavailable)
	require.NotContains(t, err.Error(), privateMarker)
	require.Empty(t, runner.events.Values())
}

func TestD3MigrationVersionTableIsModuleOwned(t *testing.T) {
	t.Parallel()

	require.Equal(t, "goose_identity_version", migrationVersionTable("identity"))
	require.Equal(
		t,
		"goose_feature_flags_version",
		migrationVersionTable("feature_flags"),
	)
}

func TestD3NewMigratorRejectsSingleConnectionPool(t *testing.T) {
	t.Parallel()

	cfg := d1DatabaseConfig("postgres", "")
	cfg.MaxOpenConnections = 1
	cfg.MaxIdleConnections = 1

	migrator, err := NewMigrator(
		&sql.DB{},
		cfg,
		[]MigrationModule{d3MigrationModule("identity")},
	)

	require.Nil(t, migrator)
	require.ErrorIs(t, err, ErrInvalidMigrationPlan)
}

func TestD3PreservesContextErrorsAndStopsMigrationSequence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		runnerError   error
		expectedError error
	}{
		{
			name:          "canceled",
			runnerError:   context.Canceled,
			expectedError: context.Canceled,
		},
		{
			name:          "deadline exceeded",
			runnerError:   context.DeadlineExceeded,
			expectedError: context.DeadlineExceeded,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			events := &recordedMigrationEvents{}
			migrator, err := newMigrator(
				MigrationDialectPostgres,
				[]MigrationModule{
					d3MigrationModule("accounts", "identity"),
					d3MigrationModule("identity"),
				},
				&recordingMigrationLocker{events: events},
				&recordingMigrationRunner{
					events:     events,
					failModule: "identity",
					failError:  test.runnerError,
				},
			)
			require.NoError(t, err)

			err = migrator.Up(context.Background())

			require.ErrorIs(t, err, test.expectedError)
			require.Equal(
				t,
				[]string{"lock", "run:identity", "unlock"},
				events.Values(),
			)
		})
	}
}

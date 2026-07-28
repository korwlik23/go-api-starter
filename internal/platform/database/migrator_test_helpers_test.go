package database

import (
	"context"
	"io/fs"
	"sync"
	"sync/atomic"
	"testing/fstest"
)

func d3MigrationModule(id string, dependencies ...string) MigrationModule {
	migrations := d3MigrationFS()
	return MigrationModule{
		ID:           id,
		Dependencies: dependencies,
		Bundles: map[MigrationDialect]fs.FS{
			MigrationDialectPostgres: migrations,
			MigrationDialectMariaDB:  migrations,
			MigrationDialectMySQL:    migrations,
		},
	}
}

func d3MigrationFS() fstest.MapFS {
	return fstest.MapFS{
		"000001_probe.sql": {
			Data: []byte("-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n"),
		},
	}
}

type recordedMigrationEvents struct {
	mu     sync.Mutex
	values []string
}

func (events *recordedMigrationEvents) Add(value string) {
	if events == nil {
		return
	}
	events.mu.Lock()
	defer events.mu.Unlock()
	events.values = append(events.values, value)
}

func (events *recordedMigrationEvents) Values() []string {
	if events == nil {
		return nil
	}
	events.mu.Lock()
	defer events.mu.Unlock()
	return append([]string(nil), events.values...)
}

type recordingMigrationLocker struct {
	events    *recordedMigrationEvents
	lockError error
}

func (locker *recordingMigrationLocker) WithLock(
	ctx context.Context,
	run func(context.Context) error,
) error {
	if locker.lockError != nil {
		return locker.lockError
	}
	locker.events.Add("lock")
	defer locker.events.Add("unlock")
	return run(ctx)
}

type recordingMigrationRunner struct {
	events     *recordedMigrationEvents
	failModule string
	failError  error
}

func (runner *recordingMigrationRunner) Up(
	_ context.Context,
	module MigrationModule,
) error {
	runner.events.Add("run:" + module.ID)
	if module.ID == runner.failModule {
		return runner.failError
	}
	return nil
}

type serialMigrationLocker struct {
	mu      sync.Mutex
	arrived chan struct{}
}

func (locker *serialMigrationLocker) WithLock(
	ctx context.Context,
	run func(context.Context) error,
) error {
	locker.arrived <- struct{}{}
	locker.mu.Lock()
	defer locker.mu.Unlock()
	return run(ctx)
}

type concurrentMigrationRunner struct {
	active        atomic.Int32
	maximumActive atomic.Int32
	calls         atomic.Int32
	entered       chan struct{}
	enterOnce     sync.Once
	release       <-chan struct{}
}

func (runner *concurrentMigrationRunner) Up(
	_ context.Context,
	_ MigrationModule,
) error {
	active := runner.active.Add(1)
	defer runner.active.Add(-1)
	runner.calls.Add(1)
	for {
		maximum := runner.maximumActive.Load()
		if active <= maximum ||
			runner.maximumActive.CompareAndSwap(maximum, active) {
			break
		}
	}
	runner.enterOnce.Do(func() {
		close(runner.entered)
	})
	<-runner.release
	return nil
}

var _ fs.FS = fstest.MapFS{}

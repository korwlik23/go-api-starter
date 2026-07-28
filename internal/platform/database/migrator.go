package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/pressly/goose/v3"
)

const maximumMigrationModuleIDLength = 40

var (
	ErrInvalidMigrationPlan     = errors.New("database: invalid migration plan")
	ErrMigrationLockUnavailable = errors.New("database: migration lock unavailable")
	ErrMigrationFailed          = errors.New("database: migration failed")

	migrationModuleIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// MigrationDialect identifies one physical SQL migration bundle.
type MigrationDialect string

const (
	MigrationDialectPostgres MigrationDialect = "postgres"
	MigrationDialectMariaDB  MigrationDialect = "mariadb"
	MigrationDialectMySQL    MigrationDialect = "mysql"
)

// MigrationModule owns one version table and one bundle for every supported
// database profile. Dependencies are module IDs, not migration versions.
type MigrationModule struct {
	ID           string
	Dependencies []string
	Bundles      map[MigrationDialect]fs.FS
}

type migrationLocker interface {
	WithLock(context.Context, func(context.Context) error) error
}

type migrationModuleRunner interface {
	Up(context.Context, MigrationModule) error
}

// Migrator applies module-owned bundles in deterministic dependency order
// while holding one database-session advisory lock for the complete run.
type Migrator struct {
	modules []MigrationModule
	locker  migrationLocker
	runner  migrationModuleRunner
}

// NewMigrator validates the database profile and migration graph before
// constructing the production Goose runner and advisory locker.
func NewMigrator(
	db *sql.DB,
	cfg config.DatabaseConfig,
	modules []MigrationModule,
) (*Migrator, error) {
	if db == nil {
		return nil, ErrInvalidMigrationPlan
	}
	if err := config.ValidateDatabase(cfg); err != nil {
		return nil, ErrInvalidMigrationPlan
	}
	if cfg.MaxOpenConnections < 2 ||
		db.Stats().MaxOpenConnections == 1 {
		return nil, ErrInvalidMigrationPlan
	}

	dialect, gooseDialect, err := migrationDialectForConfig(cfg)
	if err != nil {
		return nil, err
	}
	locker, err := newMigrationLocker(db, cfg)
	if err != nil {
		return nil, err
	}
	runner := &gooseMigrationRunner{
		db:           db,
		dialect:      dialect,
		gooseDialect: gooseDialect,
	}
	return newMigrator(dialect, modules, locker, runner)
}

func newMigrator(
	dialect MigrationDialect,
	modules []MigrationModule,
	locker migrationLocker,
	runner migrationModuleRunner,
) (*Migrator, error) {
	if !supportedMigrationDialect(dialect) ||
		len(modules) == 0 ||
		locker == nil ||
		runner == nil {
		return nil, ErrInvalidMigrationPlan
	}

	orderedModules, err := orderMigrationModules(dialect, modules)
	if err != nil {
		return nil, err
	}
	return &Migrator{
		modules: orderedModules,
		locker:  locker,
		runner:  runner,
	}, nil
}

// Up applies every pending module migration. Goose errors are deliberately
// mapped to a stable public error so driver details and credentials cannot
// escape through API responses or logs.
func (migrator *Migrator) Up(ctx context.Context) error {
	if ctx == nil {
		return ErrMigrationFailed
	}

	err := migrator.locker.WithLock(ctx, func(lockContext context.Context) error {
		for _, module := range migrator.modules {
			if err := migrator.runner.Up(lockContext, module); err != nil {
				if contextError := lockContext.Err(); contextError != nil {
					return contextError
				}
				if errors.Is(err, context.Canceled) {
					return context.Canceled
				}
				if errors.Is(err, context.DeadlineExceeded) {
					return context.DeadlineExceeded
				}
				return &migrationModuleError{moduleID: module.ID}
			}
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}

	migrationFailed := errors.Is(err, ErrMigrationFailed)
	lockUnavailable := errors.Is(err, ErrMigrationLockUnavailable)
	var moduleError *migrationModuleError
	if errors.As(err, &moduleError) {
		safeModuleError := fmt.Errorf(
			"%w: module %s",
			ErrMigrationFailed,
			moduleError.moduleID,
		)
		if lockUnavailable {
			return errors.Join(safeModuleError, ErrMigrationLockUnavailable)
		}
		return safeModuleError
	}
	switch {
	case migrationFailed && lockUnavailable:
		return errors.Join(ErrMigrationFailed, ErrMigrationLockUnavailable)
	case migrationFailed:
		return ErrMigrationFailed
	default:
		return ErrMigrationLockUnavailable
	}
}

type migrationModuleError struct {
	moduleID string
}

func (migrationError *migrationModuleError) Error() string {
	return fmt.Sprintf(
		"%s: module %s",
		ErrMigrationFailed,
		migrationError.moduleID,
	)
}

func (*migrationModuleError) Unwrap() error {
	return ErrMigrationFailed
}

func orderMigrationModules(
	dialect MigrationDialect,
	modules []MigrationModule,
) ([]MigrationModule, error) {
	byID := make(map[string]MigrationModule, len(modules))
	for _, module := range modules {
		if !ValidMigrationModuleID(module.ID) {
			return nil, ErrInvalidMigrationPlan
		}
		if _, exists := byID[module.ID]; exists {
			return nil, ErrInvalidMigrationPlan
		}
		if module.Bundles == nil || module.Bundles[dialect] == nil {
			return nil, ErrInvalidMigrationPlan
		}

		dependencies := append([]string(nil), module.Dependencies...)
		seenDependencies := make(map[string]struct{}, len(dependencies))
		for _, dependency := range dependencies {
			if !ValidMigrationModuleID(dependency) ||
				dependency == module.ID {
				return nil, ErrInvalidMigrationPlan
			}
			if _, exists := seenDependencies[dependency]; exists {
				return nil, ErrInvalidMigrationPlan
			}
			seenDependencies[dependency] = struct{}{}
		}

		bundles := make(map[MigrationDialect]fs.FS, len(module.Bundles))
		for bundleDialect, bundle := range module.Bundles {
			bundles[bundleDialect] = bundle
		}
		byID[module.ID] = MigrationModule{
			ID:           module.ID,
			Dependencies: dependencies,
			Bundles:      bundles,
		}
	}

	indegree := make(map[string]int, len(byID))
	dependents := make(map[string][]string, len(byID))
	for id, module := range byID {
		indegree[id] = len(module.Dependencies)
		for _, dependency := range module.Dependencies {
			if _, exists := byID[dependency]; !exists {
				return nil, ErrInvalidMigrationPlan
			}
			dependents[dependency] = append(dependents[dependency], id)
		}
	}
	for dependency := range dependents {
		sort.Strings(dependents[dependency])
	}

	ready := make([]string, 0, len(byID))
	for id, dependencyCount := range indegree {
		if dependencyCount == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)

	ordered := make([]MigrationModule, 0, len(byID))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byID[id])

		for _, dependent := range dependents[id] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sort.Strings(ready)
	}
	if len(ordered) != len(byID) {
		return nil, ErrInvalidMigrationPlan
	}
	return ordered, nil
}

// ValidMigrationModuleID reports whether id is safe for module-owned SQL
// identifiers and logical migration paths.
func ValidMigrationModuleID(id string) bool {
	return len(id) <= maximumMigrationModuleIDLength &&
		migrationModuleIDPattern.MatchString(id)
}

func supportedMigrationDialect(dialect MigrationDialect) bool {
	switch dialect {
	case MigrationDialectPostgres,
		MigrationDialectMariaDB,
		MigrationDialectMySQL:
		return true
	default:
		return false
	}
}

func migrationVersionTable(moduleID string) string {
	return "goose_" + moduleID + "_version"
}

func migrationDialectForConfig(
	cfg config.DatabaseConfig,
) (MigrationDialect, goose.Dialect, error) {
	switch {
	case cfg.Driver == "postgres" && cfg.Flavor == "":
		return MigrationDialectPostgres, goose.DialectPostgres, nil
	case cfg.Driver == "mysql" && cfg.Flavor == "mariadb":
		return MigrationDialectMariaDB, goose.DialectMySQL, nil
	case cfg.Driver == "mysql" && cfg.Flavor == "mysql":
		return MigrationDialectMySQL, goose.DialectMySQL, nil
	default:
		return "", "", ErrInvalidMigrationPlan
	}
}

type gooseMigrationRunner struct {
	db           *sql.DB
	dialect      MigrationDialect
	gooseDialect goose.Dialect
}

func (runner *gooseMigrationRunner) Up(
	ctx context.Context,
	module MigrationModule,
) error {
	provider, err := goose.NewProvider(
		runner.gooseDialect,
		runner.db,
		module.Bundles[runner.dialect],
		goose.WithTableName(migrationVersionTable(module.ID)),
		goose.WithDisableGlobalRegistry(true),
		goose.WithLogger(goose.NopLogger()),
	)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

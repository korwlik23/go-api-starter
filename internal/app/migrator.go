package app

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"io/fs"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"

	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

const migrationCatalogChecksumDomain = "go-api-starter:migration-catalog:v1"

var (
	// ErrInvalidMigrationCatalog is returned when module migration bundles
	// cannot be proven to have the same logical versions and basenames.
	ErrInvalidMigrationCatalog = errors.New("app: invalid migration catalog")

	canonicalMigrationFilename = regexp.MustCompile(
		`^([0-9]{6})_[a-z][a-z0-9_]*\.sql$`,
	)
	safeMigrationPathSegment  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	requiredMigrationDialects = [...]database.MigrationDialect{
		database.MigrationDialectPostgres,
		database.MigrationDialectMariaDB,
		database.MigrationDialectMySQL,
	}
)

// ValidateMigrationDialectParity performs a deterministic, database-free
// preflight of every module migration bundle.
func ValidateMigrationDialectParity(
	modules []database.MigrationModule,
) error {
	_, err := inspectMigrationCatalog(modules)
	return err
}

// MigrationCatalogChecksum returns a deterministic SHA-256 checksum over the
// validated module IDs, dialects, canonical filenames, and exact file bytes.
func MigrationCatalogChecksum(
	modules []database.MigrationModule,
) (string, error) {
	catalog, err := inspectMigrationCatalog(modules)
	if err != nil {
		return "", err
	}

	checksum := sha256.New()
	writeMigrationChecksumFrame(
		checksum,
		[]byte(migrationCatalogChecksumDomain),
	)
	for _, module := range catalog {
		writeMigrationChecksumFrame(checksum, []byte(module.id))
		for _, bundle := range module.bundles {
			writeMigrationChecksumFrame(checksum, []byte(bundle.dialect))
			for _, file := range bundle.files {
				writeMigrationChecksumFrame(checksum, []byte(file.name))
				writeMigrationChecksumFrame(checksum, file.digest[:])
			}
		}
	}
	return hex.EncodeToString(checksum.Sum(nil)), nil
}

type inspectedMigrationModule struct {
	id      string
	bundles []inspectedMigrationBundle
}

type inspectedMigrationBundle struct {
	dialect database.MigrationDialect
	files   []inspectedMigrationFile
}

type inspectedMigrationFile struct {
	name    string
	version int64
	digest  [sha256.Size]byte
}

func inspectMigrationCatalog(
	modules []database.MigrationModule,
) ([]inspectedMigrationModule, error) {
	if len(modules) == 0 {
		return nil, newMigrationCatalogError(
			"",
			"at least one migration module is required",
		)
	}

	orderedModules := append([]database.MigrationModule(nil), modules...)
	for _, module := range orderedModules {
		if !database.ValidMigrationModuleID(module.ID) {
			return nil, newMigrationCatalogError(
				"",
				"module identifier is invalid",
			)
		}
	}
	sort.Slice(orderedModules, func(left, right int) bool {
		return orderedModules[left].ID < orderedModules[right].ID
	})

	catalog := make([]inspectedMigrationModule, 0, len(orderedModules))
	previousModuleID := ""
	for _, module := range orderedModules {
		modulePath := migrationModuleLogicalPath(module.ID)
		if module.ID == previousModuleID {
			return nil, newMigrationCatalogError(
				modulePath,
				"module identifier is duplicated",
			)
		}
		previousModuleID = module.ID
		if err := validateMigrationBundleKeys(module, modulePath); err != nil {
			return nil, err
		}

		inspectedModule := inspectedMigrationModule{
			id:      module.ID,
			bundles: make([]inspectedMigrationBundle, 0, len(requiredMigrationDialects)),
		}
		for _, dialect := range requiredMigrationDialects {
			bundlePath := migrationBundleLogicalPath(module.ID, dialect)
			bundle := module.Bundles[dialect]
			if nilMigrationFilesystem(bundle) {
				return nil, newMigrationCatalogError(
					bundlePath,
					"migration bundle is missing",
				)
			}
			inspectedBundle, err := inspectMigrationBundle(
				bundle,
				dialect,
				bundlePath,
			)
			if err != nil {
				return nil, err
			}
			inspectedModule.bundles = append(
				inspectedModule.bundles,
				inspectedBundle,
			)
		}
		if err := validateInspectedMigrationParity(inspectedModule); err != nil {
			return nil, err
		}
		catalog = append(catalog, inspectedModule)
	}
	return catalog, nil
}

func validateMigrationBundleKeys(
	module database.MigrationModule,
	modulePath string,
) error {
	for dialect := range module.Bundles {
		if !requiredMigrationDialect(dialect) {
			return newMigrationCatalogError(
				modulePath,
				"unsupported migration dialect bundle",
			)
		}
	}
	return nil
}

func inspectMigrationBundle(
	bundle fs.FS,
	dialect database.MigrationDialect,
	bundlePath string,
) (inspectedMigrationBundle, error) {
	entries, err := fs.ReadDir(bundle, ".")
	if err != nil {
		return inspectedMigrationBundle{}, newMigrationCatalogError(
			bundlePath,
			"migration bundle cannot be read",
		)
	}
	if len(entries) == 0 {
		return inspectedMigrationBundle{}, newMigrationCatalogError(
			bundlePath,
			"migration bundle is empty",
		)
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})

	inspectedBundle := inspectedMigrationBundle{
		dialect: dialect,
		files:   make([]inspectedMigrationFile, 0, len(entries)),
	}
	versions := make(map[int64]struct{}, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		entryPath := bundlePath
		if safeMigrationLogicalPathSegment(name) {
			entryPath = path.Join(bundlePath, name)
		}
		entryInfo, infoError := entry.Info()
		if infoError != nil {
			return inspectedMigrationBundle{}, newMigrationCatalogError(
				entryPath,
				"migration entry cannot be inspected",
			)
		}
		if !entryInfo.Mode().IsRegular() {
			return inspectedMigrationBundle{}, newMigrationCatalogError(
				entryPath,
				"migration bundle contains a non-regular entry",
			)
		}

		matches := canonicalMigrationFilename.FindStringSubmatch(name)
		if len(matches) != 2 {
			return inspectedMigrationBundle{}, newMigrationCatalogError(
				entryPath,
				"migration filename is not canonical",
			)
		}
		version, parseError := strconv.ParseInt(matches[1], 10, 64)
		if parseError != nil || version < 1 {
			return inspectedMigrationBundle{}, newMigrationCatalogError(
				entryPath,
				"migration version is invalid",
			)
		}
		if _, duplicate := versions[version]; duplicate {
			return inspectedMigrationBundle{}, newMigrationCatalogError(
				entryPath,
				"migration version is duplicated",
			)
		}
		versions[version] = struct{}{}

		content, readError := fs.ReadFile(bundle, name)
		if readError != nil {
			return inspectedMigrationBundle{}, newMigrationCatalogError(
				entryPath,
				"migration file cannot be read",
			)
		}
		if len(content) == 0 {
			return inspectedMigrationBundle{}, newMigrationCatalogError(
				entryPath,
				"migration file is empty",
			)
		}
		inspectedBundle.files = append(
			inspectedBundle.files,
			inspectedMigrationFile{
				name:    name,
				version: version,
				digest:  sha256.Sum256(content),
			},
		)
	}
	return inspectedBundle, nil
}

func validateInspectedMigrationParity(
	module inspectedMigrationModule,
) error {
	reference := module.bundles[0]
	referenceByVersion := migrationFilesByVersion(reference.files)
	for _, candidate := range module.bundles[1:] {
		candidateByVersion := migrationFilesByVersion(candidate.files)
		for _, referenceFile := range reference.files {
			candidateFile, exists := candidateByVersion[referenceFile.version]
			if !exists {
				return newMigrationCatalogError(
					path.Join(
						migrationBundleLogicalPath(module.id, candidate.dialect),
						referenceFile.name,
					),
					"migration file is missing",
				)
			}
			if candidateFile.name != referenceFile.name {
				return newMigrationCatalogError(
					path.Join(
						migrationBundleLogicalPath(module.id, candidate.dialect),
						candidateFile.name,
					),
					"migration basename differs from PostgreSQL",
				)
			}
		}
		for _, candidateFile := range candidate.files {
			if _, exists := referenceByVersion[candidateFile.version]; !exists {
				return newMigrationCatalogError(
					path.Join(
						migrationBundleLogicalPath(module.id, candidate.dialect),
						candidateFile.name,
					),
					"migration file is extra",
				)
			}
		}
	}
	return nil
}

func migrationFilesByVersion(
	files []inspectedMigrationFile,
) map[int64]inspectedMigrationFile {
	byVersion := make(map[int64]inspectedMigrationFile, len(files))
	for _, file := range files {
		byVersion[file.version] = file
	}
	return byVersion
}

func requiredMigrationDialect(dialect database.MigrationDialect) bool {
	for _, required := range requiredMigrationDialects {
		if dialect == required {
			return true
		}
	}
	return false
}

func safeMigrationLogicalPathSegment(value string) bool {
	return value != "." &&
		value != ".." &&
		safeMigrationPathSegment.MatchString(value)
}

func nilMigrationFilesystem(filesystem fs.FS) bool {
	if filesystem == nil {
		return true
	}
	value := reflect.ValueOf(filesystem)
	switch value.Kind() {
	case reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Pointer,
		reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func migrationModuleLogicalPath(moduleID string) string {
	return path.Join("internal", "modules", moduleID, "migrations")
}

func migrationBundleLogicalPath(
	moduleID string,
	dialect database.MigrationDialect,
) string {
	return path.Join(
		migrationModuleLogicalPath(moduleID),
		string(dialect),
	)
}

type migrationCatalogError struct {
	logicalPath string
	reason      string
}

func newMigrationCatalogError(
	logicalPath string,
	reason string,
) error {
	return &migrationCatalogError{
		logicalPath: logicalPath,
		reason:      reason,
	}
}

func (catalogError *migrationCatalogError) Error() string {
	if catalogError.logicalPath == "" {
		return ErrInvalidMigrationCatalog.Error() + ": " + catalogError.reason
	}
	return ErrInvalidMigrationCatalog.Error() +
		": " + catalogError.logicalPath +
		": " + catalogError.reason
}

func (*migrationCatalogError) Unwrap() error {
	return ErrInvalidMigrationCatalog
}

func writeMigrationChecksumFrame(checksum hash.Hash, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = checksum.Write(length[:])
	_, _ = checksum.Write(value)
}

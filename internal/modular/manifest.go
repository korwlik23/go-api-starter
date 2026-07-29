package modular

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Tier string

const (
	TierCore     Tier = "core"
	TierDefault  Tier = "default"
	TierOptional Tier = "optional"
)

type PermissionScope string

const (
	ScopeOwn    PermissionScope = "own"
	ScopeAny    PermissionScope = "any"
	ScopeSystem PermissionScope = "system"
)

var ErrInvalidManifest = errors.New("invalid module manifest")

var (
	moduleIDPattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	canonicalNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(?:[.][a-z][a-z0-9_]*)*$`)
	permissionKeyPattern = regexp.MustCompile(
		`^[a-z][a-z0-9_]*[.][a-z][a-z0-9_]*[.][a-z][a-z0-9_]*[.](own|any|system)$`,
	)
	methodPattern = regexp.MustCompile(`^[A-Z]+$`)
)

type Manifest struct {
	ID                   string
	Tier                 Tier
	Disableable          bool
	Dependencies         []string
	RequiredCapabilities []string
	ProvidedCapabilities []string
	Permissions          []Permission
	Routes               []Route
	Jobs                 []Job
	Migrations           []Migration
	Health               []HealthContributor
}

type Permission struct {
	Key       string
	Scope     PermissionScope
	Tier      Tier
	Delegable bool
}

type Route struct {
	Method     string
	Path       string
	Permission string
}

type Job struct {
	ID string
}

type Migration struct {
	Dialect string
}

type HealthContributor struct {
	ID       string
	Critical bool
}

func SQLMigrations() []Migration {
	return []Migration{
		{Dialect: "postgres"},
		{Dialect: "mariadb"},
		{Dialect: "mysql"},
	}
}

func ValidateManifest(manifest Manifest) error {
	if !moduleIDPattern.MatchString(manifest.ID) {
		return invalidManifest("module ID is not canonical")
	}
	if !validTier(manifest.Tier) {
		return invalidManifest("module tier is invalid")
	}
	if manifest.Tier == TierCore && manifest.Disableable {
		return invalidManifest("core module cannot be disableable")
	}
	if err := validateDependencies(manifest); err != nil {
		return err
	}
	if err := validateCapabilities(manifest); err != nil {
		return err
	}
	permissionKeys, err := validatePermissions(manifest)
	if err != nil {
		return err
	}
	if err := validateRoutes(manifest.Routes, permissionKeys); err != nil {
		return err
	}
	if err := validateNamedSet("job", jobIDs(manifest.Jobs)); err != nil {
		return err
	}
	if err := validateMigrations(manifest.Migrations); err != nil {
		return err
	}
	return validateNamedSet("health contributor", healthIDs(manifest.Health))
}

func validateDependencies(manifest Manifest) error {
	seen := make(map[string]struct{}, len(manifest.Dependencies))
	for _, dependency := range manifest.Dependencies {
		if !moduleIDPattern.MatchString(dependency) || dependency == manifest.ID {
			return invalidManifest("module dependency is invalid")
		}
		if _, exists := seen[dependency]; exists {
			return invalidManifest("module dependency is duplicated")
		}
		seen[dependency] = struct{}{}
	}
	return nil
}

func validateCapabilities(manifest Manifest) error {
	if err := validateNamedSet("required capability", manifest.RequiredCapabilities); err != nil {
		return err
	}
	return validateNamedSet("provided capability", manifest.ProvidedCapabilities)
}

func validatePermissions(manifest Manifest) (map[string]struct{}, error) {
	seen := make(map[string]struct{}, len(manifest.Permissions))
	for _, permission := range manifest.Permissions {
		if !permissionKeyPattern.MatchString(permission.Key) {
			return nil, invalidManifest("permission key is not canonical")
		}
		segments := strings.Split(permission.Key, ".")
		if segments[0] != manifest.ID || PermissionScope(segments[3]) != permission.Scope {
			return nil, invalidManifest("permission owner or scope does not match")
		}
		if permission.Tier != manifest.Tier || !validScope(permission.Scope) {
			return nil, invalidManifest("permission tier or scope is invalid")
		}
		if _, exists := seen[permission.Key]; exists {
			return nil, invalidManifest("permission key is duplicated")
		}
		seen[permission.Key] = struct{}{}
	}
	return seen, nil
}

func validateRoutes(routes []Route, permissionKeys map[string]struct{}) error {
	seen := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		if !methodPattern.MatchString(route.Method) ||
			!strings.HasPrefix(route.Path, "/") ||
			strings.ContainsAny(route.Path, " \t\r\n") {
			return invalidManifest("route is invalid")
		}
		if route.Permission != "" {
			if _, declared := permissionKeys[route.Permission]; !declared {
				return invalidManifest("route permission is not declared by its module")
			}
		}
		key := route.Method + " " + route.Path
		if _, exists := seen[key]; exists {
			return invalidManifest("route is duplicated")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateMigrations(migrations []Migration) error {
	seen := make(map[string]struct{}, len(migrations))
	for _, migration := range migrations {
		switch migration.Dialect {
		case "postgres", "mariadb", "mysql":
		default:
			return invalidManifest("migration dialect is invalid")
		}
		if _, exists := seen[migration.Dialect]; exists {
			return invalidManifest("migration dialect is duplicated")
		}
		seen[migration.Dialect] = struct{}{}
	}
	return nil
}

func validateNamedSet(kind string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !canonicalNamePattern.MatchString(value) {
			return invalidManifest(kind + " is not canonical")
		}
		if _, exists := seen[value]; exists {
			return invalidManifest(kind + " is duplicated")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func jobIDs(jobs []Job) []string {
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		ids = append(ids, job.ID)
	}
	return ids
}

func healthIDs(contributors []HealthContributor) []string {
	ids := make([]string, 0, len(contributors))
	for _, contributor := range contributors {
		ids = append(ids, contributor.ID)
	}
	return ids
}

func validTier(tier Tier) bool {
	return tier == TierCore || tier == TierDefault || tier == TierOptional
}

func validScope(scope PermissionScope) bool {
	return scope == ScopeOwn || scope == ScopeAny || scope == ScopeSystem
}

func invalidManifest(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidManifest, reason)
}

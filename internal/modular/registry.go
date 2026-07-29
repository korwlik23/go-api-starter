package modular

import (
	"errors"
	"fmt"
)

var ErrInvalidRegistry = errors.New("invalid module registry")

type Registry struct {
	manifests   map[string]Manifest
	permissions map[string]Permission
	order       []string
}

func NewRegistry(input []Manifest) (*Registry, error) {
	manifests := make(map[string]Manifest, len(input))
	permissions := make(map[string]Permission)
	for _, candidate := range input {
		manifest := cloneManifest(candidate)
		if err := ValidateManifest(manifest); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRegistry, err)
		}
		if _, exists := manifests[manifest.ID]; exists {
			return nil, fmt.Errorf(
				"%w: duplicate module %s",
				ErrInvalidRegistry,
				manifest.ID,
			)
		}
		for _, permission := range manifest.Permissions {
			if _, exists := permissions[permission.Key]; exists {
				return nil, fmt.Errorf(
					"%w: duplicate permission %s",
					ErrInvalidRegistry,
					permission.Key,
				)
			}
			permissions[permission.Key] = permission
		}
		manifests[manifest.ID] = manifest
	}

	order, err := dependencyOrder(manifests)
	if err != nil {
		return nil, err
	}
	return &Registry{
		manifests:   manifests,
		permissions: permissions,
		order:       order,
	}, nil
}

func (registry *Registry) Ordered() []Manifest {
	if registry == nil {
		return nil
	}
	result := make([]Manifest, 0, len(registry.order))
	for _, id := range registry.order {
		result = append(result, cloneManifest(registry.manifests[id]))
	}
	return result
}

func (registry *Registry) Manifest(id string) (Manifest, bool) {
	if registry == nil {
		return Manifest{}, false
	}
	manifest, exists := registry.manifests[id]
	return cloneManifest(manifest), exists
}

func (registry *Registry) Permission(key string) (Permission, bool) {
	if registry == nil {
		return Permission{}, false
	}
	permission, exists := registry.permissions[key]
	return permission, exists
}

func cloneManifest(manifest Manifest) Manifest {
	manifest.Dependencies = append([]string(nil), manifest.Dependencies...)
	manifest.RequiredCapabilities = append(
		[]string(nil),
		manifest.RequiredCapabilities...,
	)
	manifest.ProvidedCapabilities = append(
		[]string(nil),
		manifest.ProvidedCapabilities...,
	)
	manifest.Permissions = append([]Permission(nil), manifest.Permissions...)
	manifest.Routes = append([]Route(nil), manifest.Routes...)
	manifest.Jobs = append([]Job(nil), manifest.Jobs...)
	manifest.Migrations = append([]Migration(nil), manifest.Migrations...)
	manifest.Health = append([]HealthContributor(nil), manifest.Health...)
	return manifest
}

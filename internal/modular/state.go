package modular

import "fmt"

type ProjectedPermission struct {
	Permission
	Active bool
}

type ModuleState struct {
	ID          string
	Tier        Tier
	Enabled     bool
	Permissions []ProjectedPermission
	Routes      []Route
	Jobs        []Job
	Health      []HealthContributor
}

type StateProjection struct {
	modules map[string]ModuleState
	order   []string
}

func ProjectState(
	registry *Registry,
	enabled map[string]bool,
) (*StateProjection, error) {
	if registry == nil {
		return nil, fmt.Errorf("%w: registry is required", ErrInvalidRegistry)
	}
	for id := range enabled {
		if _, exists := registry.manifests[id]; !exists {
			return nil, fmt.Errorf("%w: unknown module state %s", ErrInvalidRegistry, id)
		}
	}

	modules := make(map[string]ModuleState, len(registry.order))
	for _, manifest := range registry.Ordered() {
		isEnabled, configured := enabled[manifest.ID]
		if !configured {
			return nil, fmt.Errorf(
				"%w: module state is missing for %s",
				ErrInvalidRegistry,
				manifest.ID,
			)
		}
		if !isEnabled && !manifest.Disableable {
			return nil, fmt.Errorf(
				"%w: module %s cannot be disabled",
				ErrInvalidRegistry,
				manifest.ID,
			)
		}
		if isEnabled {
			for _, dependency := range manifest.Dependencies {
				if !enabled[dependency] {
					return nil, fmt.Errorf(
						"%w: enabled module %s requires enabled dependency %s",
						ErrInvalidRegistry,
						manifest.ID,
						dependency,
					)
				}
			}
		}
		modules[manifest.ID] = projectModule(manifest, isEnabled)
	}
	return &StateProjection{
		modules: modules,
		order:   append([]string(nil), registry.order...),
	}, nil
}

func (projection *StateProjection) Module(id string) (ModuleState, bool) {
	if projection == nil {
		return ModuleState{}, false
	}
	module, exists := projection.modules[id]
	return cloneModuleState(module), exists
}

func (projection *StateProjection) Ordered() []ModuleState {
	if projection == nil {
		return nil
	}
	result := make([]ModuleState, 0, len(projection.order))
	for _, id := range projection.order {
		result = append(result, cloneModuleState(projection.modules[id]))
	}
	return result
}

func projectModule(manifest Manifest, enabled bool) ModuleState {
	permissions := make([]ProjectedPermission, 0, len(manifest.Permissions))
	for _, permission := range manifest.Permissions {
		permissions = append(permissions, ProjectedPermission{
			Permission: permission,
			Active:     enabled,
		})
	}
	state := ModuleState{
		ID:          manifest.ID,
		Tier:        manifest.Tier,
		Enabled:     enabled,
		Permissions: permissions,
	}
	if enabled {
		state.Routes = append([]Route(nil), manifest.Routes...)
		state.Jobs = append([]Job(nil), manifest.Jobs...)
		state.Health = append([]HealthContributor(nil), manifest.Health...)
	}
	return state
}

func cloneModuleState(state ModuleState) ModuleState {
	state.Permissions = append([]ProjectedPermission(nil), state.Permissions...)
	state.Routes = append([]Route(nil), state.Routes...)
	state.Jobs = append([]Job(nil), state.Jobs...)
	state.Health = append([]HealthContributor(nil), state.Health...)
	return state
}

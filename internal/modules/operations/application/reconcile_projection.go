package application

import (
	"bytes"
	"math"
	"sort"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
)

func buildReconcileProjection(
	request ReconcileRequest,
	snapshot ports.ReconcileSnapshot,
	newID func() (string, error),
) (ports.ReconcileProjection, error) {
	state, err := modular.ProjectState(request.Registry, request.Enabled)
	if err != nil || snapshot.Revision == math.MaxInt64 {
		return ports.ReconcileProjection{}, ErrInvalidReconcile
	}

	permissions, err := projectPermissions(state, snapshot.Permissions, newID)
	if err != nil {
		return ports.ReconcileProjection{}, err
	}
	configChecksum, err := checksumConfig(state)
	if err != nil {
		return ports.ReconcileProjection{}, ErrReconcileFailed
	}
	catalogChecksum, err := checksumCatalog(state, permissions)
	if err != nil {
		return ports.ReconcileProjection{}, ErrReconcileFailed
	}
	epoch, err := nextCatalogEpoch(snapshot, catalogChecksum)
	if err != nil {
		return ports.ReconcileProjection{}, err
	}
	revision := snapshot.Revision + 1
	modules, err := projectModules(
		request.Registry,
		state,
		configChecksum,
		catalogChecksum,
		epoch,
		revision,
	)
	if err != nil {
		return ports.ReconcileProjection{}, err
	}
	return ports.ReconcileProjection{
		Revision:     revision,
		CatalogEpoch: epoch,
		Modules:      modules,
		Permissions:  permissions,
	}, nil
}

func projectPermissions(
	state *modular.StateProjection,
	existing []ports.PermissionRecord,
	newID func() (string, error),
) ([]ports.PermissionRecord, error) {
	byKey := make(map[string]ports.PermissionRecord, len(existing))
	for _, permission := range existing {
		if permission.Key == "" || permission.ModuleID == "" {
			return nil, ErrReconcileFailed
		}
		if _, duplicate := byKey[permission.Key]; duplicate {
			return nil, ErrReconcileFailed
		}
		permission.Active = false
		byKey[permission.Key] = permission
	}
	for _, module := range state.Ordered() {
		for _, projected := range module.Permissions {
			record, exists := byKey[projected.Key]
			if exists && record.ModuleID != module.ID {
				return nil, ErrPermissionOwnerCollision
			}
			if !exists {
				id, err := newID()
				if err != nil || id == "" {
					return nil, ErrReconcileFailed
				}
				record.ID = id
			}
			record.Key = projected.Key
			record.ModuleID = module.ID
			record.Scope = string(projected.Scope)
			record.Tier = string(projected.Tier)
			record.Delegable = projected.Delegable
			record.Active = projected.Active
			byKey[projected.Key] = record
		}
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	permissions := make([]ports.PermissionRecord, 0, len(keys))
	for _, key := range keys {
		permissions = append(permissions, byKey[key])
	}
	return permissions, nil
}

func projectModules(
	registry *modular.Registry,
	state *modular.StateProjection,
	configChecksum []byte,
	catalogChecksum []byte,
	epoch int64,
	revision int64,
) ([]ports.ModuleRecord, error) {
	modules := make([]ports.ModuleRecord, 0, len(state.Ordered()))
	for _, module := range state.Ordered() {
		manifest, exists := registry.Manifest(module.ID)
		if !exists {
			return nil, ErrInvalidReconcile
		}
		moduleChecksum, err := checksumValue(manifest)
		if err != nil {
			return nil, ErrReconcileFailed
		}
		modules = append(modules, ports.ModuleRecord{
			ID:                module.ID,
			Enabled:           module.Enabled,
			ModuleChecksum:    moduleChecksum,
			ConfigChecksum:    append([]byte(nil), configChecksum...),
			CatalogChecksum:   append([]byte(nil), catalogChecksum...),
			CatalogEpoch:      epoch,
			ReconcileRevision: revision,
		})
	}
	return modules, nil
}

func nextCatalogEpoch(
	snapshot ports.ReconcileSnapshot,
	catalogChecksum []byte,
) (int64, error) {
	if snapshot.CatalogEpoch < 0 || snapshot.CatalogEpoch == math.MaxInt64 {
		return 0, ErrInvalidReconcile
	}
	if snapshot.CatalogEpoch == 0 {
		return 1, nil
	}
	for _, module := range snapshot.Modules {
		if len(module.CatalogChecksum) == 32 {
			if bytes.Equal(module.CatalogChecksum, catalogChecksum) {
				return snapshot.CatalogEpoch, nil
			}
			break
		}
	}
	return snapshot.CatalogEpoch + 1, nil
}

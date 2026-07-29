package application

import (
	"crypto/sha256"
	"encoding/json"

	"github.com/korwlik23/go-api-starter/internal/modular"
	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
)

type configChecksumEntry struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type catalogChecksumEntry struct {
	Key       string `json:"key"`
	ModuleID  string `json:"module_id"`
	Scope     string `json:"scope"`
	Tier      string `json:"tier"`
	Delegable bool   `json:"delegable"`
	Active    bool   `json:"active"`
}

type catalogChecksumProjection struct {
	Modules     []configChecksumEntry  `json:"modules"`
	Permissions []catalogChecksumEntry `json:"permissions"`
}

func checksumConfig(state *modular.StateProjection) ([]byte, error) {
	entries := make([]configChecksumEntry, 0, len(state.Ordered()))
	for _, module := range state.Ordered() {
		entries = append(entries, configChecksumEntry{
			ID:      module.ID,
			Enabled: module.Enabled,
		})
	}
	return checksumValue(entries)
}

func checksumCatalog(
	state *modular.StateProjection,
	permissions []ports.PermissionRecord,
) ([]byte, error) {
	projection := catalogChecksumProjection{
		Modules: make([]configChecksumEntry, 0, len(state.Ordered())),
		Permissions: make(
			[]catalogChecksumEntry,
			0,
			len(permissions),
		),
	}
	for _, module := range state.Ordered() {
		projection.Modules = append(projection.Modules, configChecksumEntry{
			ID:      module.ID,
			Enabled: module.Enabled,
		})
	}
	for _, permission := range permissions {
		projection.Permissions = append(
			projection.Permissions,
			catalogChecksumEntry{
				Key:       permission.Key,
				ModuleID:  permission.ModuleID,
				Scope:     permission.Scope,
				Tier:      permission.Tier,
				Delegable: permission.Delegable,
				Active:    permission.Active,
			},
		)
	}
	return checksumValue(projection)
}

func checksumValue(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(encoded)
	return append([]byte(nil), sum[:]...), nil
}

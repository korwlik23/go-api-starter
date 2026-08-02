package media

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionAssetsReadOwn   = "media.assets.read.own"
	PermissionAssetsWriteOwn  = "media.assets.write.own"
	PermissionUploadsWriteOwn = "media.uploads.write.own"
)

func Permissions() []modular.Permission {
	return []modular.Permission{
		{Key: PermissionAssetsReadOwn, Scope: modular.ScopeOwn, Tier: modular.TierDefault, Delegable: false},
		{Key: PermissionAssetsWriteOwn, Scope: modular.ScopeOwn, Tier: modular.TierDefault, Delegable: false},
		{Key: PermissionUploadsWriteOwn, Scope: modular.ScopeOwn, Tier: modular.TierDefault, Delegable: false},
	}
}

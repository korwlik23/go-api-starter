package discoverability

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionRedirectsReadOwn  = "discoverability.redirects.read.own"
	PermissionRedirectsWriteOwn = "discoverability.redirects.write.own"
	PermissionSEOReadOwn        = "discoverability.seo.read.own"
	PermissionSEOWriteOwn       = "discoverability.seo.write.own"
)

func Permissions() []modular.Permission {
	return []modular.Permission{
		{Key: PermissionRedirectsReadOwn, Scope: modular.ScopeOwn, Tier: modular.TierDefault, Delegable: false},
		{Key: PermissionRedirectsWriteOwn, Scope: modular.ScopeOwn, Tier: modular.TierDefault, Delegable: false},
		{Key: PermissionSEOReadOwn, Scope: modular.ScopeOwn, Tier: modular.TierDefault, Delegable: false},
		{Key: PermissionSEOWriteOwn, Scope: modular.ScopeOwn, Tier: modular.TierDefault, Delegable: false},
	}
}

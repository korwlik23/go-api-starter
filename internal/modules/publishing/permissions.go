package publishing

import "github.com/korwlik23/go-api-starter/internal/modular"

const (
	PermissionPagesReadOwn         = "publishing.pages.read.own"
	PermissionPagesWriteOwn        = "publishing.pages.write.own"
	PermissionPagesReviewAny       = "publishing.pages.review.any"
	PermissionPagesPublishAny      = "publishing.pages.publish.any"
	PermissionRevisionsReadOwn     = "publishing.revisions.read.own"
	PermissionRevisionsRollbackOwn = "publishing.revisions.rollback.own"
)

func Permissions() []modular.Permission {
	return []modular.Permission{
		{
			Key:       PermissionPagesReadOwn,
			Scope:     modular.ScopeOwn,
			Tier:      modular.TierDefault,
			Delegable: false,
		},
		{
			Key:       PermissionPagesWriteOwn,
			Scope:     modular.ScopeOwn,
			Tier:      modular.TierDefault,
			Delegable: false,
		},
		{
			Key:       PermissionPagesReviewAny,
			Scope:     modular.ScopeAny,
			Tier:      modular.TierDefault,
			Delegable: false,
		},
		{
			Key:       PermissionPagesPublishAny,
			Scope:     modular.ScopeAny,
			Tier:      modular.TierDefault,
			Delegable: false,
		},
		{
			Key:       PermissionRevisionsReadOwn,
			Scope:     modular.ScopeOwn,
			Tier:      modular.TierDefault,
			Delegable: false,
		},
		{
			Key:       PermissionRevisionsRollbackOwn,
			Scope:     modular.ScopeOwn,
			Tier:      modular.TierDefault,
			Delegable: false,
		},
	}
}

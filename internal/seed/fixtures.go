package seed

type roleFixture struct {
	Key         string
	Name        string
	Email       string
	Status      string
	SystemKeys  []string
	AccountKeys []string
	DisplayName string
	Locale      string
}

var demoRoles = []roleFixture{
	{
		Key: "owner", Name: "Owner", Email: "owner@example.com", Status: "active",
		DisplayName: "Local Owner", Locale: "th",
	},
	{
		Key: "author", Name: "Author", Email: "author@example.com", Status: "active",
		AccountKeys: []string{
			"publishing.pages.read.own", "publishing.pages.write.own", "publishing.revisions.read.own", "publishing.revisions.rollback.own",
			"media.assets.read.own", "media.assets.write.own", "media.uploads.write.own", "navigation.menus.read.own", "navigation.menus.write.own",
			"discoverability.redirects.read.own", "discoverability.redirects.write.own", "discoverability.seo.read.own", "discoverability.seo.write.own", "discoverability.audit.read.own", "discoverability.audit.run.own",
			"identity.profile.read.own", "identity.profile.manage.own", "identity.password.manage.own", "accounts.current.read.own", "notifications.inbox.read.own", "notifications.inbox.manage.own",
		},
		DisplayName: "Local Author", Locale: "en",
	},
	{
		Key: "reviewer", Name: "Reviewer", Email: "reviewer@example.com", Status: "active",
		AccountKeys: []string{"publishing.pages.read.own", "publishing.pages.review.any", "publishing.revisions.read.own", "identity.profile.read.own", "identity.password.manage.own", "accounts.current.read.own", "notifications.inbox.read.own"},
		DisplayName: "Local Reviewer", Locale: "th",
	},
	{
		Key: "publisher", Name: "Publisher", Email: "publisher@example.com", Status: "active",
		AccountKeys: []string{"publishing.pages.read.own", "publishing.pages.review.any", "publishing.pages.publish.any", "publishing.revisions.read.own", "navigation.menus.read.own", "navigation.menus.publish.any", "identity.profile.read.own", "identity.password.manage.own", "accounts.current.read.own", "notifications.inbox.read.own"},
		DisplayName: "Local Publisher", Locale: "en",
	},
	{
		Key: "translator", Name: "Translator", Email: "translator@example.com", Status: "active",
		SystemKeys:  []string{"localization.translations.read.system", "localization.locales.manage.system", "localization.translations.update.system"},
		AccountKeys: []string{"accounts.current.read.own", "identity.profile.read.own", "identity.password.manage.own", "notifications.inbox.read.own"},
		DisplayName: "Local Translator", Locale: "ja",
	},
	{
		Key: "ops", Name: "Operations", Email: "ops@example.com", Status: "active",
		SystemKeys:  []string{"operations.foundation.read.system", "operations.jobs.read.system", "operations.jobs.manage.system", "settings.global.read.system", "settings.global.manage.system", "audit.events.read.system"},
		AccountKeys: []string{"accounts.current.read.own", "identity.profile.read.own", "identity.password.manage.own", "notifications.inbox.read.own"},
		DisplayName: "Local Operations", Locale: "en",
	},
	{
		Key: "viewer", Name: "Viewer", Email: "viewer@example.com", Status: "active",
		AccountKeys: []string{"accounts.current.read.own", "publishing.pages.read.own", "media.assets.read.own", "navigation.menus.read.own", "discoverability.redirects.read.own", "discoverability.seo.read.own", "identity.profile.read.own", "identity.password.manage.own", "notifications.inbox.read.own"},
		DisplayName: "Local Viewer", Locale: "th",
	},
	{
		Key: "disabled", Name: "Disabled", Email: "disabled@example.com", Status: "disabled",
		DisplayName: "Disabled Demo User", Locale: "th",
	},
}

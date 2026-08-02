package contract_test

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestOpenAPISurfaceIsolation(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	surfaces := map[string][]string{}
	for _, pathItem := range document.Paths.Map() {
		operations := []*openapi3.Operation{
			pathItem.Get, pathItem.Post, pathItem.Put, pathItem.Patch, pathItem.Delete,
		}
		for _, operation := range operations {
			if operation == nil {
				continue
			}
			raw, ok := operation.Extensions["x-surfaces"].([]interface{})
			require.True(t, ok, "operation %s must declare x-surfaces", operation.OperationID)
			require.NotEmpty(t, raw, "operation %s must belong to at least one surface", operation.OperationID)
			for _, value := range raw {
				surface, ok := value.(string)
				require.True(t, ok)
				surfaces[surface] = append(surfaces[surface], operation.OperationID)
			}
		}
	}

	for surface := range surfaces {
		sort.Strings(surfaces[surface])
	}
	require.ElementsMatch(t, []string{
		"assignRole", "confirmMFAEnrollment", "createLocale", "enrollMFA", "getCSRFToken",
		"getCurrentAccount", "getCurrentSession", "getEffectivePermissions",
		"getLocalePreferences", "getMFAChallenge", "getTranslationCatalog",
		"grantRolePermission", "listAuthorizationRoles", "listEnabledModules",
		"getAdminUser", "listAdminUsers", "listAccountMemberships", "listAuditEvents",
		"listLocales", "listMediaAssets", "listRoleAssignments", "listRolePermissions", "listSessions", "login", "logout", "requestPasswordRecovery",
		"listDiscoverabilityRedirects", "createDiscoverabilityRedirect", "getDiscoverabilityRedirect", "updateDiscoverabilityRedirect", "deleteDiscoverabilityRedirect", "getDiscoverabilitySEO", "updateDiscoverabilitySEO",
		"startMediaUpload", "finalizeMediaUpload", "deleteMediaAsset",
		"listNavigationMenus", "createNavigationMenu", "getNavigationMenu", "updateNavigationMenu", "deleteNavigationMenu", "reorderNavigationMenu",
		"resetPassword", "revokeRoleAssignment", "revokeSession", "setDefaultLocale",
		"updateAuthorizationRole", "updateLocale", "updateLocalePreferences", "updateLocaleStatus",
		"createAuthorizationRole",
		"updateTranslationCatalog", "updateAdminUserStatus", "updateMembershipStatus", "verifyMFA",
	}, surfaces["admin"])
	require.ElementsMatch(t, []string{"getTranslationCatalog", "listLocales"}, surfaces["public"])
	require.ElementsMatch(t, []string{
		"getLiveness", "getReadiness", "getTranslationCatalog", "listLocales",
	}, surfaces["site-server"])
}

func TestGeneratedSurfaceBundlesContainOnlyTheirOperations(t *testing.T) {
	expected := map[string][]string{
		"admin": {
			"assignRole", "confirmMFAEnrollment", "createLocale", "enrollMFA", "getCSRFToken",
			"getCurrentAccount", "getCurrentSession", "getEffectivePermissions",
			"getLocalePreferences", "getMFAChallenge", "getTranslationCatalog",
			"grantRolePermission", "listAuthorizationRoles", "listEnabledModules",
			"getAdminUser", "listAdminUsers", "listAccountMemberships", "listAuditEvents",
			"listLocales", "listMediaAssets", "listRoleAssignments", "listRolePermissions", "listSessions", "login", "logout", "requestPasswordRecovery",
			"listDiscoverabilityRedirects", "createDiscoverabilityRedirect", "getDiscoverabilityRedirect", "updateDiscoverabilityRedirect", "deleteDiscoverabilityRedirect", "getDiscoverabilitySEO", "updateDiscoverabilitySEO",
			"startMediaUpload", "finalizeMediaUpload", "deleteMediaAsset",
			"listNavigationMenus", "createNavigationMenu", "getNavigationMenu", "updateNavigationMenu", "deleteNavigationMenu", "reorderNavigationMenu",
			"resetPassword", "revokeRoleAssignment", "revokeSession", "setDefaultLocale",
			"updateAuthorizationRole", "updateLocale", "updateLocalePreferences", "updateLocaleStatus",
			"createAuthorizationRole",
			"updateTranslationCatalog", "updateAdminUserStatus", "updateMembershipStatus", "verifyMFA",
		},
		"public": {"getTranslationCatalog", "listLocales"},
		"site-server": {
			"getLiveness", "getReadiness", "getTranslationCatalog", "listLocales",
		},
	}

	for surface, operationIDs := range expected {
		t.Run(surface, func(t *testing.T) {
			loader := openapi3.NewLoader()
			document, err := loader.LoadFromFile(filepath.Join(
				"..", "..", "openapi", "dist", surface+".openapi.yaml",
			))
			require.NoError(t, err)
			require.NoError(t, document.Validate(context.Background()))

			actual := make([]string, 0, len(operationIDs))
			for _, pathItem := range document.Paths.Map() {
				operations := []*openapi3.Operation{
					pathItem.Get, pathItem.Post, pathItem.Put, pathItem.Patch, pathItem.Delete,
				}
				for _, operation := range operations {
					if operation != nil {
						actual = append(actual, operation.OperationID)
					}
				}
			}

			require.ElementsMatch(t, operationIDs, actual)
		})
	}
}

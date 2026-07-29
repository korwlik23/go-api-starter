package contract_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type openAPIKeyScheme struct {
	In   string `yaml:"in"`
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

type securityComponentsDocument struct {
	Components struct {
		SecuritySchemes map[string]openAPIKeyScheme `yaml:"securitySchemes"`
	} `yaml:"components"`
}

func TestCommonOpenAPIContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))
	require.Equal(t, "3.0.3", document.OpenAPI)
	require.Len(t, document.Servers, 1)
	require.Equal(t, "{scheme}://{host}/api/v1", document.Servers[0].URL)

	problem := document.Components.Schemas["Problem"]
	require.NotNil(t, problem)
	require.NotNil(t, problem.Value)
	require.ElementsMatch(t, []string{"code", "message", "fields", "request_id"}, problem.Value.Required)
	require.True(t, problem.Value.Type.Is("object"))
	require.True(t, problem.Value.Properties["request_id"].Value.Type.Is("string"))
	require.True(t, problem.Value.Properties["fields"].Value.Type.Is("object"))

	errorEnvelope := document.Components.Schemas["ErrorEnvelope"]
	require.NotNil(t, errorEnvelope)
	require.NotNil(t, errorEnvelope.Value)
	require.Equal(t, []string{"error"}, errorEnvelope.Value.Required)

	securityContents, err := os.ReadFile(filepath.Join("..", "..", "openapi", "common", "security.yaml"))
	require.NoError(t, err)
	var securityDocument securityComponentsDocument
	require.NoError(t, yaml.Unmarshal(securityContents, &securityDocument))

	sessionCookie := securityDocument.Components.SecuritySchemes["SessionCookie"]
	require.Equal(t, "apiKey", sessionCookie.Type)
	require.Equal(t, "cookie", sessionCookie.In)
	require.Equal(t, "go_lang_starter_session", sessionCookie.Name)

	csrfToken := securityDocument.Components.SecuritySchemes["CSRFToken"]
	require.Equal(t, "apiKey", csrfToken.Type)
	require.Equal(t, "header", csrfToken.In)
	require.Equal(t, "X-CSRF-Token", csrfToken.Name)

	pageInfo := document.Components.Schemas["PageInfo"]
	require.NotNil(t, pageInfo)
	require.NotNil(t, pageInfo.Value)
	require.ElementsMatch(t, []string{"page", "per_page", "total_items", "total_pages"}, pageInfo.Value.Required)
	require.True(t, pageInfo.Value.Properties["page"].Value.Type.Is("integer"))
	require.Equal(t, float64(100), *document.Components.Parameters["PerPage"].Value.Schema.Value.Max)
}

func TestPreAuthCSRFContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/auth/csrf")
	require.NotNil(t, pathItem)
	require.NotNil(t, pathItem.Get)
	require.Equal(t, "getCSRFToken", pathItem.Get.OperationID)

	response := pathItem.Get.Responses.Value("200")
	require.NotNil(t, response)
	require.NotNil(t, response.Value)
	require.Equal(t, []interface{}{"no-store"}, response.Value.Headers["Cache-Control"].Value.Schema.Value.Enum)
	require.NotNil(t, response.Value.Headers["Set-Cookie"])

	schema := response.Value.Content.Get("application/json").Schema
	require.NotNil(t, schema)
	require.NotNil(t, schema.Value)
	require.ElementsMatch(t, []string{"csrf_token"}, schema.Value.Required)
	require.True(t, schema.Value.Properties["csrf_token"].Value.Type.Is("string"))
}

func TestLoginContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/auth/login")
	require.NotNil(t, pathItem)
	require.NotNil(t, pathItem.Post)
	require.Equal(t, "login", pathItem.Post.OperationID)
	require.Len(t, *pathItem.Post.Security, 1)
	require.Contains(t, (*pathItem.Post.Security)[0], "SessionCookie")
	require.Contains(t, (*pathItem.Post.Security)[0], "CSRFToken")

	requestSchema := pathItem.Post.RequestBody.Value.Content.Get("application/json").Schema
	require.ElementsMatch(t, []string{"email", "password"}, requestSchema.Value.Required)
	require.True(t, requestSchema.Value.Properties["email"].Value.Type.Is("string"))
	require.True(t, requestSchema.Value.Properties["password"].Value.WriteOnly)

	for _, status := range []string{"200", "202"} {
		response := pathItem.Post.Responses.Value(status)
		require.NotNil(t, response)
		require.Equal(t, []interface{}{"no-store"}, response.Value.Headers["Cache-Control"].Value.Schema.Value.Enum)
		require.NotNil(t, response.Value.Headers["Set-Cookie"])
	}

	authenticated := pathItem.Post.Responses.Value("200").Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"status", "csrf_token"}, authenticated.Required)
	require.Equal(t, []interface{}{"authenticated"}, authenticated.Properties["status"].Value.Enum)

	mfaPending := pathItem.Post.Responses.Value("202").Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"status", "csrf_token", "challenge_id"}, mfaPending.Required)
	require.Equal(t, []interface{}{"mfa_pending"}, mfaPending.Properties["status"].Value.Enum)
	require.NotContains(t, mfaPending.Properties, "totp_secret")
	require.NotContains(t, mfaPending.Properties, "recovery_code")
}

func TestMFAEnrollmentContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/auth/mfa/enrollment")
	require.NotNil(t, pathItem)
	require.NotNil(t, pathItem.Post)
	require.Equal(t, "enrollMFA", pathItem.Post.OperationID)
	require.Contains(t, (*pathItem.Post.Security)[0], "SessionCookie")
	require.Contains(t, (*pathItem.Post.Security)[0], "CSRFToken")

	response := pathItem.Post.Responses.Value("201")
	require.NotNil(t, response)
	require.Equal(t, []interface{}{"no-store"}, response.Value.Headers["Cache-Control"].Value.Schema.Value.Enum)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"enrollment_id", "provisioning_uri"}, schema.Required)
	require.NotContains(t, schema.Properties, "encrypted_secret")
	require.NotContains(t, schema.Properties, "secret_ciphertext")
}

func TestMFAChallengeContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/auth/mfa/challenge")
	require.NotNil(t, pathItem)
	require.NotNil(t, pathItem.Get)
	require.Equal(t, "getMFAChallenge", pathItem.Get.OperationID)
	require.Contains(t, (*pathItem.Get.Security)[0], "SessionCookie")

	response := pathItem.Get.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"status", "challenge_id", "methods"}, schema.Required)
	require.Equal(t, []interface{}{"mfa_pending"}, schema.Properties["status"].Value.Enum)
	require.NotContains(t, schema.Properties, "totp_secret")
	require.NotContains(t, schema.Properties, "verification_code")
}

func TestMFAVerificationContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/auth/mfa/verify")
	require.NotNil(t, pathItem)
	require.NotNil(t, pathItem.Post)
	require.Equal(t, "verifyMFA", pathItem.Post.OperationID)
	require.Contains(t, (*pathItem.Post.Security)[0], "SessionCookie")
	require.Contains(t, (*pathItem.Post.Security)[0], "CSRFToken")

	requestSchema := pathItem.Post.RequestBody.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"method", "code"}, requestSchema.Required)
	require.Equal(t, []interface{}{"totp", "recovery_code"}, requestSchema.Properties["method"].Value.Enum)
	require.True(t, requestSchema.Properties["code"].Value.WriteOnly)

	response := pathItem.Post.Responses.Value("200")
	require.NotNil(t, response)
	require.Equal(t, []interface{}{"no-store"}, response.Value.Headers["Cache-Control"].Value.Schema.Value.Enum)
	require.NotNil(t, response.Value.Headers["Set-Cookie"])
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"status", "csrf_token"}, schema.Required)
	require.Equal(t, []interface{}{"authenticated"}, schema.Properties["status"].Value.Enum)
	require.NotContains(t, schema.Properties, "recovery_code")
}

func TestCurrentSessionContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/auth/session")
	require.NotNil(t, pathItem)
	require.NotNil(t, pathItem.Get)
	require.Equal(t, "getCurrentSession", pathItem.Get.OperationID)
	require.Contains(t, (*pathItem.Get.Security)[0], "SessionCookie")

	response := pathItem.Get.Responses.Value("200")
	require.NotNil(t, response)
	require.Equal(t, []interface{}{"no-store"}, response.Value.Headers["Cache-Control"].Value.Schema.Value.Enum)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"session_id", "principal", "state"}, schema.Required)
	require.NotContains(t, schema.Properties, "token")
	require.NotContains(t, schema.Properties, "cookie")
	require.NotContains(t, schema.Properties, "fingerprint")
}

func TestSessionListContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/auth/sessions")
	require.NotNil(t, pathItem)
	require.NotNil(t, pathItem.Get)
	require.Equal(t, "listSessions", pathItem.Get.OperationID)
	require.Contains(t, (*pathItem.Get.Security)[0], "SessionCookie")

	response := pathItem.Get.Responses.Value("200")
	require.NotNil(t, response)
	require.Equal(t, []interface{}{"no-store"}, response.Value.Headers["Cache-Control"].Value.Schema.Value.Enum)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"items"}, schema.Required)
	item := schema.Properties["items"].Value.Items.Value
	require.ElementsMatch(t, []string{"id", "created_at", "last_seen_at", "current"}, item.Required)
	require.NotContains(t, item.Properties, "token")
	require.NotContains(t, item.Properties, "fingerprint")
}

func TestSessionRevokeContract(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "openapi", "root.yaml"))
	require.NoError(t, err)
	require.NoError(t, document.Validate(context.Background()))

	pathItem := document.Paths.Find("/auth/sessions/{session_id}")
	require.NotNil(t, pathItem)
	require.NotNil(t, pathItem.Delete)
	require.Equal(t, "revokeSession", pathItem.Delete.OperationID)
	require.Contains(t, (*pathItem.Delete.Security)[0], "SessionCookie")
	require.Contains(t, (*pathItem.Delete.Security)[0], "CSRFToken")

	response := pathItem.Delete.Responses.Value("200")
	require.NotNil(t, response)
	schema := response.Value.Content.Get("application/json").Schema.Value
	require.ElementsMatch(t, []string{"target_session_id", "revoked"}, schema.Required)
	require.NotNil(t, pathItem.Delete.Responses.Value("409"))
}

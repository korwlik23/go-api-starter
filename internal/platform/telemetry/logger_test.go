package telemetry

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestA5(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := NewLogger(&output, LoggerOptions{
		Service:     "go-api-starter",
		Environment: "test",
		Level:       slog.LevelInfo,
	})

	logger.Info(
		"request completed",
		slog.String("request_id", "request-123"),
		slog.String("trace_id", "trace-456"),
		slog.String("route", "/api/v1/users/:id"),
		slog.Int("status", 200),
		slog.Duration("duration", 25*time.Millisecond),
	)

	var record map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &record))
	require.NotEmpty(t, record["timestamp"])
	require.Equal(t, "INFO", record["level"])
	require.Equal(t, "request completed", record["message"])
	require.Equal(t, "go-api-starter", record["service"])
	require.Equal(t, "test", record["environment"])
	require.Equal(t, "request-123", record["request_id"])
	require.Equal(t, "trace-456", record["trace_id"])
	require.Equal(t, "/api/v1/users/:id", record["route"])
	require.Equal(t, float64(200), record["status"])
	require.Contains(t, record, "duration")

	output.Reset()
	passwordValue := sensitiveFixture("password")
	cookieValue := sensitiveFixture("cookie")
	tokenValue := sensitiveFixture("token")
	secretValue := sensitiveFixture("secret")
	sessionValue := sensitiveFixture("session")
	authorizationValue := sensitiveFixture("authorization")

	logger.Info(
		"sensitive fields",
		slog.String("password", passwordValue),
		slog.String("raw_cookie", cookieValue),
		slog.String("access_token", tokenValue),
		slog.String("client_secret", secretValue),
		slog.String("session_id", sessionValue),
		slog.Group("headers", slog.String("authorization", authorizationValue)),
	)

	rawRecord := output.String()
	for _, sensitiveValue := range []string{
		passwordValue,
		cookieValue,
		tokenValue,
		secretValue,
		sessionValue,
		authorizationValue,
	} {
		require.NotContains(t, rawRecord, sensitiveValue)
	}

	record = nil
	require.NoError(t, json.Unmarshal(output.Bytes(), &record))
	require.Equal(t, "[REDACTED]", record["password"])
	require.Equal(t, "[REDACTED]", record["raw_cookie"])
	require.Equal(t, "[REDACTED]", record["access_token"])
	require.Equal(t, "[REDACTED]", record["client_secret"])
	require.Equal(t, "[REDACTED]", record["session_id"])
	headers, ok := record["headers"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", headers["authorization"])
}

func sensitiveFixture(kind string) string {
	return "sensitive-" + kind + "-fixture"
}

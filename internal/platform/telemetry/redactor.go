package telemetry

import (
	"log/slog"
	"strings"
)

const redactedValue = "[REDACTED]"

func redactAttribute(attribute slog.Attr) slog.Attr {
	if isSensitiveKey(attribute.Key) {
		attribute.Value = slog.StringValue(redactedValue)
	}
	return attribute
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(key)

	for _, fragment := range []string{
		"password",
		"passwd",
		"secret",
		"token",
		"cookie",
		"authorization",
		"api_key",
		"apikey",
		"private_key",
		"privatekey",
		"session_id",
		"sessionid",
		"session_key",
		"sessionkey",
	} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}

	return false
}

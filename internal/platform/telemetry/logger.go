package telemetry

import (
	"io"
	"log/slog"
)

type LoggerOptions struct {
	Service     string
	Environment string
	Level       slog.Level
}

// NewLogger creates the structured application logger used by the API.
func NewLogger(output io.Writer, options LoggerOptions) *slog.Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level:       options.Level,
		ReplaceAttr: normalizeAttribute,
	})

	return slog.New(handler).With(
		slog.String("service", options.Service),
		slog.String("environment", options.Environment),
	)
}

func normalizeAttribute(_ []string, attribute slog.Attr) slog.Attr {
	switch attribute.Key {
	case slog.TimeKey:
		attribute.Key = "timestamp"
	case slog.MessageKey:
		attribute.Key = "message"
	}
	return redactAttribute(attribute)
}

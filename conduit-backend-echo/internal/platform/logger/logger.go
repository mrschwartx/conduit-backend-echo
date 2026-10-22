// Package logger builds the structured slog logger and carries request-scoped
// loggers / request ids through context.
package logger

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

// New returns a JSON logger writing to w at the given level (default info).
func New(level string, w io.Writer) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.TrimSpace(level))); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl}))
}

type ctxKey int

const (
	loggerKey ctxKey = iota
	requestIDKey
)

// WithContext stores a request-scoped logger and the request id in ctx.
func WithContext(ctx context.Context, l *slog.Logger, requestID string) context.Context {
	ctx = context.WithValue(ctx, loggerKey, l)
	return context.WithValue(ctx, requestIDKey, requestID)
}

// FromContext returns the request-scoped logger or the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// RequestID returns the request id stored in ctx ("" if none).
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

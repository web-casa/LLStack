package logging

import (
	"io"
	"log/slog"
)

// Logger is the subset used across the Phase 1 skeleton.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// NewDefault returns a text logger suitable for CLI and local debugging.
func NewDefault(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

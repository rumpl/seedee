package server

import (
	"log/slog"
	"strings"
	"time"
)

// Config holds server configuration.
type Config struct {
	// Addr is the address to listen on (e.g., ":8080", "0.0.0.0:9090").
	Addr string

	// LogLevel is the minimum log level (debug, info, warn, error).
	LogLevel string

	// PruneInterval is how often to clean up old pipeline runs.
	PruneInterval time.Duration

	// PruneMaxAge is the max age of completed pipeline runs before pruning.
	PruneMaxAge time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Addr:          ":8080",
		LogLevel:      "info",
		PruneInterval: 5 * time.Minute,
		PruneMaxAge:   1 * time.Hour,
	}
}

// ParseLogLevel converts a log level string to a slog.Level.
// It is case-insensitive and defaults to slog.LevelInfo for unknown values.
func ParseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

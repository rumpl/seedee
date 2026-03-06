package server_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/rumpl/seedee/internal/server"
)

func TestDefaultConfig(t *testing.T) {
	cfg := server.DefaultConfig()

	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":8080")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.PruneInterval != 5*time.Minute {
		t.Errorf("PruneInterval = %v, want %v", cfg.PruneInterval, 5*time.Minute)
	}
	if cfg.PruneMaxAge != 1*time.Hour {
		t.Errorf("PruneMaxAge = %v, want %v", cfg.PruneMaxAge, 1*time.Hour)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"Warn", slog.LevelWarn},
		{"Error", slog.LevelError},
		{"garbage", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := server.ParseLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestServer_CustomAddr(t *testing.T) {
	cfg := server.Config{Addr: ":0"}
	if cfg.Addr != ":0" {
		t.Errorf("Config.Addr = %q, want %q", cfg.Addr, ":0")
	}
}

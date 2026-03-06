// Package main is the entry point for the seedeed server.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rumpl/seedee/internal/server"
)

func main() {
	cfg := server.DefaultConfig()

	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "listen address")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "log level (debug, info, warn, error)")
	flag.DurationVar(&cfg.PruneInterval, "prune-interval", cfg.PruneInterval, "how often to prune old runs")
	flag.DurationVar(&cfg.PruneMaxAge, "prune-max-age", cfg.PruneMaxAge, "max age of completed runs before pruning")
	flag.Parse()

	// Environment variables override flags
	if addr := os.Getenv("SEEDEE_ADDR"); addr != "" {
		cfg.Addr = addr
	}
	if level := os.Getenv("SEEDEE_LOG_LEVEL"); level != "" {
		cfg.LogLevel = level
	}

	logLevel := server.ParseLogLevel(cfg.LogLevel)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)

	srv := server.NewServer(cfg, logger)
	if err := srv.Start(ctx); err != nil {
		cancel()
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
	cancel()
}

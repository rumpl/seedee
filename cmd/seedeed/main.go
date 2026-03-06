// Package main is the entry point for the seedeed server.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rumpl/seedee/internal/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	addr := ":8080"
	if a := os.Getenv("SEEDEE_ADDR"); a != "" {
		addr = a
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	srv := server.NewServer(addr, logger)
	if err := srv.Start(ctx); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

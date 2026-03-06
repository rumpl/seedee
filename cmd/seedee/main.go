// Package main is the entry point for the seedee CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rumpl/seedee/internal/cli"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		_, _ = fmt.Fprintf(os.Stderr, "\nInterrupted. Shutting down gracefully... (Ctrl+C again to force)\n")
		cancel()
		<-sigCh
		_, _ = fmt.Fprintf(os.Stderr, "\nForce quit.\n")
		os.Exit(130)
	}()

	root := cli.NewRootCmd()
	root.SetContext(ctx)

	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

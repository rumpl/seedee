// Package server contains the ConnectRPC server handlers.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/rumpl/seedee/gen/seedee/v1/seedeev1connect"
)

const (
	// pruneInterval is how often the server checks for old pipeline runs.
	pruneInterval = 5 * time.Minute
	// pruneMaxAge is the maximum age of a completed pipeline run before it is pruned.
	pruneMaxAge = 1 * time.Hour
)

// Server is an HTTP/2 server hosting ConnectRPC handlers.
type Server struct {
	cfg     Config
	handler *CIServiceHandler
	logger  *slog.Logger
}

// NewServer creates a new Server with the given configuration and logger.
func NewServer(cfg Config, logger *slog.Logger) *Server {
	return &Server{
		cfg:     cfg,
		handler: NewCIServiceHandler(logger),
		logger:  logger,
	}
}

// Start starts the server and blocks until the context is canceled or
// a fatal error occurs. It performs a graceful shutdown when the context
// is done.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	path, handler := seedeev1connect.NewCIServiceHandler(s.handler)
	mux.Handle(path, handler)

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:      s.cfg.Addr,
		Handler:   mux,
		Protocols: protocols,
	}

	// Start background goroutine to prune old completed pipeline runs.
	go func() {
		ticker := time.NewTicker(pruneInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.handler.PruneOldRuns(pruneMaxAge)
			}
		}
	}()

	go func() {
		<-ctx.Done()
		s.logger.Info("shutting down server")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("server shutdown error", "error", err)
		}
	}()

	s.logger.Info("starting server", "addr", s.cfg.Addr)

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}

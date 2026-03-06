// Package server contains the ConnectRPC server handlers.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/rumpl/seedee/gen/seedee/v1/seedeev1connect"
)

var (
	errRunPipelineNotImplemented       = errors.New("seedee.v1.CIService.RunPipeline is not implemented")
	errGetPipelineStatusNotImplemented = errors.New("seedee.v1.CIService.GetPipelineStatus is not implemented")
	errCancelPipelineNotImplemented    = errors.New("seedee.v1.CIService.CancelPipeline is not implemented")
)

// Server is an HTTP/2 server hosting ConnectRPC handlers.
type Server struct {
	addr    string
	handler *CIServiceHandler
	logger  *slog.Logger
}

// NewServer creates a new Server that will listen on the given address.
func NewServer(addr string, logger *slog.Logger) *Server {
	return &Server{
		addr:    addr,
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
		fmt.Fprintln(w, "ok")
	})

	h2cHandler := h2c.NewHandler(mux, &http2.Server{})

	srv := &http.Server{
		Addr:    s.addr,
		Handler: h2cHandler,
	}

	go func() {
		<-ctx.Done()
		s.logger.Info("shutting down server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("server shutdown error", "error", err)
		}
	}()

	s.logger.Info("starting server", "addr", s.addr)

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

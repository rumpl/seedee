package server

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"

	seedeev1 "github.com/rumpl/seedee/gen/seedee/v1"
)

// CIServiceHandler implements the seedee.v1.CIService ConnectRPC service.
type CIServiceHandler struct {
	logger *slog.Logger
}

// NewCIServiceHandler creates a new CIServiceHandler with the given logger.
func NewCIServiceHandler(logger *slog.Logger) *CIServiceHandler {
	return &CIServiceHandler{
		logger: logger,
	}
}

// RunPipeline is not yet implemented.
func (h *CIServiceHandler) RunPipeline(
	_ context.Context,
	_ *connect.Request[seedeev1.RunPipelineRequest],
	_ *connect.ServerStream[seedeev1.RunPipelineEvent],
) error {
	return connect.NewError(connect.CodeUnimplemented, errRunPipelineNotImplemented)
}

// GetPipelineStatus is not yet implemented.
func (h *CIServiceHandler) GetPipelineStatus(
	_ context.Context,
	_ *connect.Request[seedeev1.GetPipelineStatusRequest],
) (*connect.Response[seedeev1.GetPipelineStatusResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errGetPipelineStatusNotImplemented)
}

// CancelPipeline is not yet implemented.
func (h *CIServiceHandler) CancelPipeline(
	_ context.Context,
	_ *connect.Request[seedeev1.CancelPipelineRequest],
) (*connect.Response[seedeev1.CancelPipelineResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errCancelPipelineNotImplemented)
}

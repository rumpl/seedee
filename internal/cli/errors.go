package cli

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
)

// wrapConnectError wraps a ConnectRPC error with a user-friendly message
// that includes the server address and actionable guidance.
func wrapConnectError(err error, addr string) error {
	if err == nil {
		return nil
	}

	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		switch connectErr.Code() {
		case connect.CodeUnavailable:
			return fmt.Errorf("could not connect to server at %s: %w\n\nIs the seedee server running? Start it with: seedeed --addr %s", addr, err, addr)
		case connect.CodeInvalidArgument:
			return fmt.Errorf("server rejected the pipeline: %w", err)
		case connect.CodeCanceled:
			return fmt.Errorf("pipeline execution was canceled: %w", err)
		case connect.CodeDeadlineExceeded:
			return fmt.Errorf("request to %s timed out: %w", addr, err)
		case connect.CodeUnimplemented:
			return fmt.Errorf("server at %s does not support this operation: %w", addr, err)
		default:
			return fmt.Errorf("server error from %s: %w", addr, err)
		}
	}

	// Non-connect error (network, DNS, etc.)
	return fmt.Errorf("failed to communicate with server at %s: %w\n\nCheck that the address is correct and the server is reachable.", addr, err)
}

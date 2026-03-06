package cli

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
)

func TestWrapConnectError_Nil(t *testing.T) {
	if err := wrapConnectError(nil, "localhost:8080"); err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestWrapConnectError_Unavailable(t *testing.T) {
	err := connect.NewError(connect.CodeUnavailable, errors.New("connection refused"))
	wrapped := wrapConnectError(err, "localhost:8080")
	if wrapped == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(wrapped.Error(), "could not connect") {
		t.Errorf("expected 'could not connect' in message, got: %s", wrapped.Error())
	}
	if !strings.Contains(wrapped.Error(), "localhost:8080") {
		t.Errorf("expected address in message, got: %s", wrapped.Error())
	}
}

func TestWrapConnectError_InvalidArgument(t *testing.T) {
	err := connect.NewError(connect.CodeInvalidArgument, errors.New("bad pipeline"))
	wrapped := wrapConnectError(err, "localhost:8080")
	if !strings.Contains(wrapped.Error(), "rejected") {
		t.Errorf("expected 'rejected' in message, got: %s", wrapped.Error())
	}
}

func TestWrapConnectError_Canceled(t *testing.T) {
	err := connect.NewError(connect.CodeCanceled, errors.New("canceled"))
	wrapped := wrapConnectError(err, "localhost:8080")
	if !strings.Contains(wrapped.Error(), "canceled") {
		t.Errorf("expected 'canceled' in message, got: %s", wrapped.Error())
	}
}

func TestWrapConnectError_DeadlineExceeded(t *testing.T) {
	err := connect.NewError(connect.CodeDeadlineExceeded, errors.New("timeout"))
	wrapped := wrapConnectError(err, "localhost:8080")
	if !strings.Contains(wrapped.Error(), "timed out") {
		t.Errorf("expected 'timed out' in message, got: %s", wrapped.Error())
	}
}

func TestWrapConnectError_Unimplemented(t *testing.T) {
	err := connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
	wrapped := wrapConnectError(err, "localhost:8080")
	if !strings.Contains(wrapped.Error(), "does not support") {
		t.Errorf("expected 'does not support' in message, got: %s", wrapped.Error())
	}
}

func TestWrapConnectError_OtherCode(t *testing.T) {
	err := connect.NewError(connect.CodeInternal, errors.New("internal"))
	wrapped := wrapConnectError(err, "localhost:8080")
	if !strings.Contains(wrapped.Error(), "server error") {
		t.Errorf("expected 'server error' in message, got: %s", wrapped.Error())
	}
}

func TestWrapConnectError_NonConnect(t *testing.T) {
	err := errors.New("DNS resolution failed")
	wrapped := wrapConnectError(err, "badhost:8080")
	if !strings.Contains(wrapped.Error(), "failed to communicate") {
		t.Errorf("expected 'failed to communicate' in message, got: %s", wrapped.Error())
	}
	if !strings.Contains(wrapped.Error(), "badhost:8080") {
		t.Errorf("expected address in message, got: %s", wrapped.Error())
	}
}

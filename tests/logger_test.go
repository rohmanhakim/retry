package retrier_test

import (
	"context"
	"errors"
	"testing"
	"time"

	retrier "github.com/rohmanhakim/retrier"
)

// TestNoOpLogger_Enabled tests that NoOpLogger.Enabled returns false.
func TestNoOpLogger_Enabled(t *testing.T) {
	logger := retrier.NewNoOpLogger()
	if logger.Enabled() {
		t.Error("NoOpLogger.Enabled() should return false")
	}
}

// TestNoOpLogger_LogRetry tests that NoOpLogger.LogRetry is a no-op.
func TestNoOpLogger_LogRetry(t *testing.T) {
	logger := retrier.NewNoOpLogger()

	// LogRetry should not panic and should do nothing
	// This test verifies it can be called with various parameters without error
	logger.LogRetry(context.Background(), 1, 3, 100*time.Millisecond, errors.New("test error"))
	logger.LogRetry(context.Background(), 2, 3, 200*time.Millisecond, nil)
	logger.LogRetry(context.TODO(), 0, 0, 0, nil) // context.TODO and zero values
}

// TestNoOpLogger_NewNoOpLogger tests that NewNoOpLogger returns a valid instance.
func TestNoOpLogger_NewNoOpLogger(t *testing.T) {
	logger := retrier.NewNoOpLogger()
	if logger == nil {
		t.Error("NewNoOpLogger() should not return nil")
	}
}

// TestNoOpLogger_ImplementsDebugLogger verifies NoOpLogger implements DebugLogger interface.
func TestNoOpLogger_ImplementsDebugLogger(t *testing.T) {
	// This test ensures NoOpLogger properly implements the DebugLogger interface
	var _ retrier.DebugLogger = retrier.NewNoOpLogger()
}

// TestWithLogAttrs_AttrsPassedToLogger verifies that attrs passed via WithLogAttrs
// are correctly forwarded to the logger's LogRetry method.
func TestWithLogAttrs_AttrsPassedToLogger(t *testing.T) {
	mock := newMockLogger(true)

	// Create a function that succeeds on first try
	fn := func() (string, error) {
		return "success", nil
	}

	ctx := context.Background()
	result := retrier.Retry(ctx, mock, fn,
		retrier.WithLogAttrs("operation", "test_op", "request_id", "abc123"),
	)

	// Verify success
	if !result.IsSuccess() {
		t.Errorf("expected success, got error: %v", result.Err())
	}

	// Verify logger was called with attrs
	if len(mock.logRetryCalls) != 1 {
		t.Fatalf("expected 1 log call, got %d", len(mock.logRetryCalls))
	}

	call := mock.logRetryCalls[0]
	expectedAttrs := []any{"operation", "test_op", "request_id", "abc123"}

	if len(call.attrs) != len(expectedAttrs) {
		t.Errorf("expected %d attrs, got %d", len(expectedAttrs), len(call.attrs))
	}

	for i, attr := range expectedAttrs {
		if call.attrs[i] != attr {
			t.Errorf("expected attr[%d] = %v, got %v", i, attr, call.attrs[i])
		}
	}
}

// TestWithLogAttrs_EmptyAttrs verifies that retry works without WithLogAttrs.
func TestWithLogAttrs_EmptyAttrs(t *testing.T) {
	mock := newMockLogger(true)

	fn := func() (string, error) {
		return "success", nil
	}

	ctx := context.Background()
	result := retrier.Retry(ctx, mock, fn)

	if !result.IsSuccess() {
		t.Errorf("expected success, got error: %v", result.Err())
	}

	if len(mock.logRetryCalls) != 1 {
		t.Fatalf("expected 1 log call, got %d", len(mock.logRetryCalls))
	}

	// attrs should be empty
	if len(mock.logRetryCalls[0].attrs) != 0 {
		t.Errorf("expected 0 attrs, got %d", len(mock.logRetryCalls[0].attrs))
	}
}

package retry

import (
	"context"
	"time"
)

// DebugLogger provides structured debug logging capabilities for the retry package.
// Users can implement this interface to provide custom logging behavior.
// When debug mode is disabled, use NoOpLogger for zero overhead.
type DebugLogger interface {
	// Enabled returns true if debug logging is enabled.
	// When false, the retry handler will skip logging entirely for efficiency.
	Enabled() bool

	// LogRetry logs retry attempts with backoff information.
	// Parameters:
	//   - ctx: context for the retry operation
	//   - attempt: current attempt number (1-based)
	//   - maxAttempts: maximum number of attempts allowed
	//   - backoff: the delay before the next retry (0 for success/exhausted)
	//   - err: the error that triggered the retry (nil on success)
	LogRetry(ctx context.Context, attempt int, maxAttempts int, backoff time.Duration, err error)
}

// NoOpLogger is a no-operation implementation of DebugLogger.
// It provides zero overhead when debug mode is disabled.
// All methods are empty and Enabled() always returns false.
type NoOpLogger struct{}

// NewNoOpLogger creates a new NoOpLogger instance.
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

// Enabled returns false - debug logging is disabled.
func (n *NoOpLogger) Enabled() bool { return false }

// LogRetry is a no-op.
func (n *NoOpLogger) LogRetry(_ context.Context, _ int, _ int, _ time.Duration, _ error) {
}

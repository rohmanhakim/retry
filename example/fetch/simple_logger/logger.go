// Package simple_logger provides a simple implementation of the retrier.DebugLogger interface
// that prints retry information to stdout.
package simple_logger

import (
	"context"
	"fmt"
	"time"

	"github.com/rohmanhakim/retrier"
)

// SimpleLogger implements retrier.DebugLogger interface.
// It prints retry attempts to stdout with timestamps.
type SimpleLogger struct{}

// NewSimpleLogger creates a new SimpleLogger instance.
func NewSimpleLogger() *SimpleLogger {
	return &SimpleLogger{}
}

// Enabled returns true to enable debug logging.
func (l *SimpleLogger) Enabled() bool {
	return true
}

// LogRetry prints retry information to stdout.
// It shows the attempt number, max attempts, backoff duration, error (if any),
// and any additional attributes passed via WithLogAttrs.
func (l *SimpleLogger) LogRetry(_ context.Context, attempt, maxAttempts int, backoff time.Duration, err error, attrs ...any) {
	timestamp := time.Now().Format("15:04:05.000")

	// Format attrs as key=value pairs
	var attrStr string
	if len(attrs) > 0 {
		attrStr = " "
		for i := 0; i < len(attrs)-1; i += 2 {
			if key, ok := attrs[i].(string); ok {
				attrStr += fmt.Sprintf("%s=%v ", key, attrs[i+1])
			}
		}
	}

	if err == nil {
		// Success case
		fmt.Printf("[%s] ✅ Success on attempt %d/%d%s\n", timestamp, attempt, maxAttempts, attrStr)
	} else if backoff > 0 {
		// Retry case - will retry after backoff
		fmt.Printf("[%s] ❌ Attempt %d/%d failed:%s%v\n", timestamp, attempt, maxAttempts, attrStr, err)
		fmt.Printf("[%s]    ↳ Retrying in %v...\n", timestamp, backoff)
	} else {
		// Exhausted case - no more retries
		fmt.Printf("[%s] 🛑 Attempt %d/%d failed (exhausted):%s%v\n", timestamp, attempt, maxAttempts, attrStr, err)
	}
}

// Interface assertion to ensure SimpleLogger implements DebugLogger
var _ retrier.DebugLogger = (*SimpleLogger)(nil)

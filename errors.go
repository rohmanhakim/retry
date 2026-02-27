// Package retrier provides a generic retry mechanism with exponential backoff and jitter.
package retrier

import "fmt"

// RetryPolicy defines automatic retry behavior.
// This controls whether Retry() will attempt exponential backoff.
type RetryPolicy int

const (
	// RetryPolicyAuto indicates the error should be retried automatically
	// with exponential backoff.
	RetryPolicyAuto RetryPolicy = iota

	// RetryPolicyManual indicates the error should not be auto-retried,
	// but is eligible for manual retry.
	RetryPolicyManual

	// RetryPolicyNever indicates a permanent failure that should not be retried.
	RetryPolicyNever
)

// RetryableError is an interface that errors must implement to be handled
// by the retry mechanism. Users should implement this interface on their
// custom error types to control retry behavior.
type RetryableError interface {
	error

	// RetryPolicy controls automatic retry behavior.
	// Used by the retry handler to decide whether to retry.
	RetryPolicy() RetryPolicy
}

// RetryErrorCause represents the cause of a retry error.
type RetryErrorCause string

const (
	// ErrZeroAttempt indicates that MaxAttempts was set to 0.
	ErrZeroAttempt RetryErrorCause = "zero attempt"

	// ErrExhaustedAttempts indicates that all retry attempts were exhausted.
	ErrExhaustedAttempts RetryErrorCause = "exhausted attempt"

	// ErrContextCancelled indicates that the context was cancelled during retry.
	ErrContextCancelled RetryErrorCause = "context cancelled"
)

// RetryError represents an error that occurred during retry attempts.
// It stores the original error for debugging and implements the RetryableError interface.
type RetryError struct {
	Message string
	Cause   RetryErrorCause
	wrapped error       // Original error that caused the retry failure
	policy  RetryPolicy // Cached policy for interface method
}

// NewRetryError creates a new RetryError with explicit classification.
// Parameters:
//   - cause: The error cause (ErrZeroAttempt or ErrExhaustedAttempts)
//   - message: Human-readable error message
//   - policy: RetryPolicyAuto, RetryPolicyManual, or RetryPolicyNever
//   - wrapped: The original error that caused the retry failure (may be nil)
func NewRetryError(cause RetryErrorCause, message string, policy RetryPolicy, wrapped error) *RetryError {
	return &RetryError{
		Message: message,
		Cause:   cause,
		wrapped: wrapped,
		policy:  policy,
	}
}

// Error returns the error message implementing the error interface.
func (e *RetryError) Error() string {
	if e.wrapped != nil {
		return fmt.Sprintf("retry error: %s, %s: %v", e.Cause, e.Message, e.wrapped)
	}
	return fmt.Sprintf("retry error: %s, %s", e.Cause, e.Message)
}

// Unwrap returns the wrapped error for error chain support.
func (e *RetryError) Unwrap() error {
	return e.wrapped
}

// RetryPolicy returns the automatic retry behavior for this error.
// When RetryError is returned (exhausted attempts), it returns the cached policy.
func (e *RetryError) RetryPolicy() RetryPolicy {
	return e.policy
}

// Is allows errors.Is to match RetryError types.
func (e *RetryError) Is(target error) bool {
	_, ok := target.(*RetryError)
	return ok
}

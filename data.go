package retrier

import (
	"fmt"
	"time"
)

// retryConfig holds the internal configuration for retry logic.
// It is populated via functional options.
type retryConfig struct {
	jitter             time.Duration
	maxAttempts        int
	backoff            backoffConfig
	defaultRetryPolicy RetryPolicy
}

// backoffConfig holds the internal configuration for exponential backoff.
type backoffConfig struct {
	initialDuration time.Duration
	multiplier      float64
	maxDuration     time.Duration
}

// defaults returns a retryConfig with sensible default values.
func defaults() retryConfig {
	return retryConfig{
		maxAttempts:        3,
		jitter:             0,
		defaultRetryPolicy: RetryPolicyAuto,
		backoff: backoffConfig{
			initialDuration: 1 * time.Second,
			multiplier:      2.0,
			maxDuration:     1 * time.Minute,
		},
	}
}

// RetryOption is a functional option for configuring retry behavior.
type RetryOption func(*retryConfig)

// WithMaxAttempts sets the maximum number of retry attempts.
// Default is 3.
func WithMaxAttempts(n int) RetryOption {
	return func(c *retryConfig) {
		c.maxAttempts = n
	}
}

// WithJitter sets the maximum random duration added to backoff delays.
// This helps avoid thundering herd problems. Default is 0 (no jitter).
func WithJitter(d time.Duration) RetryOption {
	return func(c *retryConfig) {
		c.jitter = d
	}
}

// WithInitialDuration sets the initial backoff duration.
// Default is 1 second.
func WithInitialDuration(d time.Duration) RetryOption {
	return func(c *retryConfig) {
		c.backoff.initialDuration = d
	}
}

// WithMultiplier sets the backoff multiplier.
// Each subsequent delay is multiplied by this value. Default is 2.0.
func WithMultiplier(m float64) RetryOption {
	return func(c *retryConfig) {
		c.backoff.multiplier = m
	}
}

// WithMaxDuration sets the maximum backoff duration.
// Default is 1 minute.
func WithMaxDuration(d time.Duration) RetryOption {
	return func(c *retryConfig) {
		c.backoff.maxDuration = d
	}
}

// WithRetryPolicy sets the default retry policy for standard errors.
// Default is RetryPolicyAuto (standard errors are retried automatically).
func WithRetryPolicy(p RetryPolicy) RetryOption {
	return func(c *retryConfig) {
		c.defaultRetryPolicy = p
	}
}

// Result encapsulates the immutable outcome of a retry operation.
// It holds either a successful value or an error, along with metadata about the execution.
type Result[T any] struct {
	value    T
	err      error
	attempts int
}

// NewSuccessResult creates a Result representing a successful retry operation.
func NewSuccessResult[T any](value T, attempts int) Result[T] {
	return Result[T]{value: value, attempts: attempts}
}

// NewFailureResult creates a Result representing a failed retry operation.
func NewFailureResult[T any](err error, attempts int) Result[T] {
	var zero T
	return Result[T]{value: zero, err: err, attempts: attempts}
}

// Value returns the successful result value.
// Returns zero value of T if the operation failed.
func (r Result[T]) Value() T {
	return r.value
}

// Err returns the error if the operation failed, or nil if successful.
func (r Result[T]) Err() error {
	return r.err
}

// Attempts returns the number of attempts made before success or failure.
func (r Result[T]) Attempts() int {
	return r.attempts
}

// IsSuccess returns true if the operation succeeded (no error).
func (r Result[T]) IsSuccess() bool {
	return r.err == nil
}

// IsFailure returns true if the operation failed (has error).
func (r Result[T]) IsFailure() bool {
	return r.err != nil
}

// Decompose returns the result as a tuple (value, attempts, error).
// This provides idiomatic Go error handling for traditionalists:
//
//	value, attempts, err := retrier.Retry(ctx, logger, fn).Decompose()
//	if err != nil {
//	    // handle error
//	}
func (r Result[T]) Decompose() (T, int, error) {
	return r.value, r.attempts, r.err
}

// UnwrapOr returns the successful value, or the provided default if failed.
// Perfect for fallback configurations:
//
//	cacheTTL := retrier.Retry(ctx, logger, fetchRemoteConfig).UnwrapOr(defaultTTL)
func (r Result[T]) UnwrapOr(defaultValue T) T {
	if r.err != nil {
		return defaultValue
	}
	return r.value
}

// Unwrap returns the successful value or panics if failed.
// Use only when failure is impossible or should crash.
func (r Result[T]) Unwrap() T {
	if r.err != nil {
		panic(fmt.Sprintf("unwrap called on failed result: %v", r.err))
	}
	return r.value
}

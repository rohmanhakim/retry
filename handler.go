package retrier

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Retry executes the provided function with retry logic.
// It will retry the function up to MaxAttempts times, applying exponential backoff
// with jitter between attempts.
//
// Type parameter T represents the return type of the function being retried.
// Returns a Result containing the value (if successful), error (if failed),
// and the number of attempts made.
//
// The ctx parameter allows cancellation of the retry operation. If the context
// is cancelled during a backoff delay, the function returns immediately with
// ErrContextCancelled.
//
// The logger parameter provides debug logging capabilities. When debug mode is
// disabled (NoOpLogger), there is zero overhead from logging.
//
// opts are functional options to configure retry behavior:
//   - WithMaxAttempts(n int): Maximum retry attempts (default: 3)
//   - WithJitter(d time.Duration): Random delay added to backoff (default: 0)
//   - WithInitialDuration(d time.Duration): Initial backoff duration (default: 1s)
//   - WithMultiplier(m float64): Backoff multiplier (default: 2.0)
//   - WithMaxDuration(d time.Duration): Maximum backoff duration (default: 1m)
//   - WithRetryPolicy(p RetryPolicy): Default retry policy for standard errors (default: RetryPolicyAuto)
//
// Error handling:
//   - If the error implements RetryableError, its RetryPolicy() is used
//   - Standard errors use the configured DefaultRetryPolicy (defaults to RetryPolicyAuto)
//
// Example:
//
//	result := retrier.Retry(ctx, logger, fn,
//	    retrier.WithMaxAttempts(5),
//	    retrier.WithJitter(100*time.Millisecond),
//	    retrier.WithInitialDuration(1*time.Second),
//	)
func Retry[T any](ctx context.Context, logger DebugLogger, fn func() (T, error), opts ...RetryOption) Result[T] {
	// Apply defaults and options
	config := defaults()
	for _, opt := range opts {
		opt(&config)
	}

	var lastErr error
	var zero T

	if config.maxAttempts < 1 {
		return Result[T]{
			value: zero,
			err: NewRetryError(
				ErrZeroAttempt,
				"max attempt cannot be 0",
				RetryPolicyNever, // Zero attempt is a configuration error
				nil,
			),
			attempts: 0,
		}
	}

	for attempt := 1; attempt <= config.maxAttempts; attempt++ {
		result, err := fn()

		// Success case: no error
		if err == nil {
			// Log successful retry if debug enabled
			if logger.Enabled() {
				logger.LogRetry(ctx, attempt, config.maxAttempts, 0, nil)
			}
			return NewSuccessResult(result, attempt)
		}

		lastErr = err

		// Check if the error should be auto-retried based on RetryPolicy
		// RetryableError with explicit policy takes precedence
		// Standard errors use DefaultRetryPolicy
		if !shouldAutoRetry(err, config.defaultRetryPolicy) {
			return Result[T]{
				value:    zero,
				err:      err,
				attempts: attempt,
			}
		}

		// If this was the last attempt, break and return exhausted error
		if attempt == config.maxAttempts {
			break
		}

		// Compute delay for the next retry using exponential backoff with jitter
		backoffDelay := exponentialBackoffDelay(
			attempt,
			config.jitter,
			config.backoff,
		)

		// Log retry attempt if debug enabled
		if logger.Enabled() {
			logger.LogRetry(ctx, attempt, config.maxAttempts, backoffDelay, err)
		}

		// Wait for backoff delay or context cancellation
		select {
		case <-ctx.Done():
			return Result[T]{
				value: zero,
				err: NewRetryError(
					ErrContextCancelled,
					fmt.Sprintf("context cancelled after %d attempts", attempt),
					RetryPolicyNever,
					ctx.Err(),
				),
				attempts: attempt,
			}
		case <-time.After(backoffDelay):
		}
	}

	// Log exhausted attempts if debug enabled
	if logger.Enabled() {
		logger.LogRetry(ctx, config.maxAttempts, config.maxAttempts, 0, lastErr)
	}

	// Return failure result when max attempts are exhausted
	return Result[T]{
		value: zero,
		err: NewRetryError(
			ErrExhaustedAttempts,
			fmt.Sprintf("exhausted %d attempts. Last error: %v", config.maxAttempts, lastErr),
			RetryPolicyManual, // Exhausted auto-retry → manual retry eligible
			lastErr,           // Preserve original error
		),
		attempts: config.maxAttempts,
	}
}

// shouldAutoRetry determines whether an error should trigger automatic retry.
// If the error implements RetryableError, its RetryPolicy() is used.
// Otherwise, the defaultPolicy is applied.
func shouldAutoRetry(err error, defaultPolicy RetryPolicy) bool {
	var retryErr RetryableError
	if errors.As(err, &retryErr) {
		return retryErr.RetryPolicy() == RetryPolicyAuto
	}
	// Standard error: use default policy
	return defaultPolicy == RetryPolicyAuto
}

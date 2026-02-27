package retrier

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// Retry executes the provided function with retry logic.
// It will retry the function up to MaxAttempts times, applying exponential backoff
// with jitter between attempts. Only retryable errors will trigger a retry.
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
func Retry[T any](ctx context.Context, retryParam RetryParam, logger DebugLogger, fn func() (T, RetryableError)) Result[T] {
	var lastErr RetryableError
	var zero T

	if retryParam.MaxAttempts < 1 {
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

	// Initialize random number generator with the provided seed
	rng := rand.New(rand.NewSource(retryParam.RandomSeed))

	for attempt := 1; attempt <= retryParam.MaxAttempts; attempt++ {
		result, err := fn()

		// Success case: no error
		if err == nil {
			// Log successful retry if debug enabled
			if logger.Enabled() {
				logger.LogRetry(ctx, attempt, retryParam.MaxAttempts, 0, nil)
			}
			return NewSuccessResult(result, attempt)
		}

		lastErr = err

		// Check if the error should be auto-retried based on RetryPolicy
		// Only RetryPolicyAuto errors trigger automatic retry with exponential backoff
		if !shouldAutoRetry(err) {
			return Result[T]{
				value:    zero,
				err:      err,
				attempts: attempt,
			}
		}

		// If this was the last attempt, break and return exhausted error
		if attempt == retryParam.MaxAttempts {
			break
		}

		// Compute delay for the next retry using exponential backoff with jitter
		backoffDelay := exponentialBackoffDelay(
			attempt,
			retryParam.Jitter,
			*rng,
			retryParam.BackoffParam,
		)

		// Log retry attempt if debug enabled
		if logger.Enabled() {
			logger.LogRetry(ctx, attempt, retryParam.MaxAttempts, backoffDelay, err)
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
		logger.LogRetry(ctx, retryParam.MaxAttempts, retryParam.MaxAttempts, 0, lastErr)
	}

	// Return failure result when max attempts are exhausted
	return Result[T]{
		value: zero,
		err: NewRetryError(
			ErrExhaustedAttempts,
			fmt.Sprintf("exhausted %d attempts. Last error: %v", retryParam.MaxAttempts, lastErr),
			RetryPolicyManual, // Exhausted auto-retry → manual retry eligible
			lastErr,           // Preserve original error
		),
		attempts: retryParam.MaxAttempts,
	}
}

// shouldAutoRetry determines whether an error should trigger automatic retry.
// It checks the error's RetryPolicy: only RetryPolicyAuto triggers automatic retry.
// This is the primary method for retry decision-making in the retry handler.
func shouldAutoRetry(err RetryableError) bool {
	return err.RetryPolicy() == RetryPolicyAuto
}

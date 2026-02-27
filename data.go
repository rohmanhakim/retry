package retrier

import "time"

// RetryParam holds the parameters for retry logic.
// These parameters are passed from outside (e.g., config) and should not
// be known by the retry handler internally.
type RetryParam struct {
	Jitter       time.Duration
	MaxAttempts  int
	BackoffParam BackoffParam
}

// NewRetryParam creates a new RetryParam with the given settings.
func NewRetryParam(
	jitter time.Duration,
	maxAttempts int,
	backoffParam BackoffParam,
) RetryParam {
	return RetryParam{
		Jitter:       jitter,
		MaxAttempts:  maxAttempts,
		BackoffParam: backoffParam,
	}
}

// Result encapsulates the immutable outcome of a retry operation.
// It holds either a successful value or an error, along with metadata about the execution.
type Result[T any] struct {
	value    T
	err      RetryableError
	attempts int
}

// NewSuccessResult creates a Result representing a successful retry operation.
func NewSuccessResult[T any](value T, attempts int) Result[T] {
	return Result[T]{value: value, attempts: attempts}
}

// NewFailureResult creates a Result representing a failed retry operation.
func NewFailureResult[T any](err RetryableError, attempts int) Result[T] {
	var zero T
	return Result[T]{value: zero, err: err, attempts: attempts}
}

// Value returns the successful result value.
// Returns zero value of T if the operation failed.
func (r Result[T]) Value() T {
	return r.value
}

// Err returns the error if the operation failed, or nil if successful.
func (r Result[T]) Err() RetryableError {
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

# retry

[![codecov](https://codecov.io/github/rohmanhakim/retry/branch/master/graph/badge.svg?token=eYWKQEsSzz)](https://codecov.io/github/rohmanhakim/retry)
[![Go Reference](https://pkg.go.dev/badge/github.com/rohmanhakim/retry.svg)](https://pkg.go.dev/github.com/rohmanhakim/retry)

A simple, standalone Go package for retrying operations with exponential backoff and jitter support. Zero external dependencies.

## Features

- **Generic API**: Works with any return type using Go generics
- **Exponential Backoff**: Configurable backoff with multiplier and max duration
- **Jitter Support**: Add randomness to backoff delays to avoid thundering herd
- **Custom Retry Policies**: Control which errors should be retried automatically
- **Debug Logging**: Optional logging interface for observability
- **Zero Dependencies**: Uses only Go standard library

## Installation

```bash
go get github.com/rohmanhakim/retry
```

## Quick Start

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/rohmanhakim/retry"
)

// Define a custom error that implements RetryableError
type NetworkError struct {
    msg string
}

func (e *NetworkError) Error() string {
    return e.msg
}

func (e *NetworkError) RetryPolicy() retry.RetryPolicy {
    return retry.RetryPolicyAuto // Will be retried automatically
}

func main() {
    // Configure retry parameters
    backoffParam := retry.NewBackoffParam(
        1*time.Second,  // Initial delay
        2.0,            // Multiplier (doubles each time)
        30*time.Second, // Maximum delay
    )
    
    params := retry.NewRetryParam(
        100*time.Millisecond, // Base delay
        50*time.Millisecond,  // Jitter
        42,                   // Random seed
        5,                    // Max attempts
        backoffParam,
    )
    
    // Define the operation to retry
    fn := func() (string, retry.RetryableError) {
        // Your operation here
        // Return RetryPolicyAuto error for transient failures
        // Return RetryPolicyManual or RetryPolicyNever for permanent failures
        return "success", nil
    }
    
    // Execute with retry
    result := retry.Retry(params, retry.NewNoOpLogger(), fn)
    
    if result.IsSuccess() {
        fmt.Printf("Success: %s (attempts: %d)\n", result.Value(), result.Attempts())
    } else {
        fmt.Printf("Failed: %v (attempts: %d)\n", result.Err(), result.Attempts())
    }
}
```

## Retry Policies

The package supports three retry policies:

| Policy | Description |
|--------|-------------|
| `RetryPolicyAuto` | Error will be retried automatically with exponential backoff |
| `RetryPolicyManual` | Error should not be auto-retried, but is eligible for manual retry |
| `RetryPolicyNever` | Permanent failure, should not be retried at all |

## Implementing RetryableError

To control retry behavior, implement the `RetryableError` interface on your error types:

```go
type MyError struct {
    msg       string
    transient bool
}

func (e *MyError) Error() string {
    return e.msg
}

func (e *MyError) RetryPolicy() retry.RetryPolicy {
    if e.transient {
        return retry.RetryPolicyAuto
    }
    return retry.RetryPolicyNever
}
```

## Debug Logging

Implement the `DebugLogger` interface to add observability:

```go
type MyLogger struct{}

func (l *MyLogger) Enabled() bool {
    return true
}

func (l *MyLogger) LogRetry(ctx context.Context, attempt, maxAttempts int, backoff time.Duration, err error) {
    if err != nil {
        log.Printf("Retry %d/%d: backoff=%v, error=%v", attempt, maxAttempts, backoff, err)
    } else {
        log.Printf("Success on attempt %d/%d", attempt, maxAttempts)
    }
}
```

Use `retry.NewNoOpLogger()` for zero-overhead when logging is not needed.

## API Reference

### Types

```go
// RetryPolicy defines automatic retry behavior
type RetryPolicy int

const (
    RetryPolicyAuto   RetryPolicy = iota // Auto retry with backoff
    RetryPolicyManual                    // Manual retry only
    RetryPolicyNever                     // No retry
)

// RetryableError interface for error classification
type RetryableError interface {
    error
    RetryPolicy() RetryPolicy
}

// RetryParam holds retry configuration
type RetryParam struct {
    BaseDelay    time.Duration
    Jitter       time.Duration
    RandomSeed   int64
    MaxAttempts  int
    BackoffParam BackoffParam
}

// Result holds the outcome of a retry operation
type Result[T any] struct {
    // Contains value on success, zero value on failure
    // Contains error on failure, nil on success
    // Number of attempts made
}

// BackoffParam holds exponential backoff configuration
type BackoffParam struct {
    // Initial duration, multiplier, max duration
}

// DebugLogger interface for logging
type DebugLogger interface {
    Enabled() bool
    LogRetry(ctx context.Context, attempt, maxAttempts int, backoff time.Duration, err error)
}
```

### Functions

```go
// Retry executes fn with retry logic
func Retry[T any](retryParam RetryParam, logger DebugLogger, fn func() (T, RetryableError)) Result[T]

// NewRetryParam creates retry configuration
func NewRetryParam(baseDelay, jitter time.Duration, randomSeed int64, maxAttempts int, backoffParam BackoffParam) RetryParam

// NewBackoffParam creates backoff configuration
func NewBackoffParam(initialDuration time.Duration, multiplier float64, maxDuration time.Duration) BackoffParam

// NewNoOpLogger creates a no-op logger (zero overhead)
func NewNoOpLogger() *NoOpLogger

// NewRetryError creates a retry error
func NewRetryError(cause RetryErrorCause, message string, policy RetryPolicy, wrapped error) *RetryError
```

## License

MIT License - see [LICENSE](LICENSE) for details.

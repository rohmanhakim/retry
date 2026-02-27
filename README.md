# retrier

[![codecov](https://codecov.io/github/rohmanhakim/retrier/branch/master/graph/badge.svg?token=eYWKQEsSzz)](https://codecov.io/github/rohmanhakim/retrier)
[![Go Reference](https://pkg.go.dev/badge/github.com/rohmanhakim/retry.svg)](https://pkg.go.dev/github.com/rohmanhakim/retry)

A simple, standalone Go package for retrying operations with exponential backoff and jitter support. Zero external dependencies.

## Features

- **Generic API**: Works with any return type using Go generics
- **Zero Friction**: Accepts standard `error` - works seamlessly with stdlib and third-party packages
- **Exponential Backoff**: Configurable backoff with multiplier and max duration
- **Jitter Support**: Add randomness to backoff delays to avoid thundering herd
- **Flexible Retry Policies**: Control which errors should be retried automatically
- **Default Retry Policy**: Configure default behavior for standard errors
- **Context Cancellation**: Support for graceful cancellation during backoff delays
- **Debug Logging**: Optional logging interface for observability
- **Zero Dependencies**: Uses only Go standard library

## Installation

```bash
go get github.com/rohmanhakim/retrier
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"
    
    "github.com/rohmanhakim/retrier"
)

func main() {
    // Configure retry parameters
    backoffParam := retrier.NewBackoffParam(
        1*time.Second,  // Initial delay
        2.0,            // Multiplier (doubles each time)
        30*time.Second, // Maximum delay
    )
    
    params := retrier.NewRetryParam(
        50*time.Millisecond,  // Jitter (random delay added to backoff)
        5,                    // Max attempts
        backoffParam,
    )
    
    // Define the operation to retry - just return standard errors!
    fn := func() (*http.Response, error) {
        return http.Get("https://api.example.com/data")
    }
    
    // Execute with retry (context can be cancelled to abort mid-backoff)
    ctx := context.Background()
    result := retrier.Retry(ctx, params, retrier.NewNoOpLogger(), fn)
    
    if result.IsSuccess() {
        fmt.Printf("Success: %v (attempts: %d)\n", result.Value(), result.Attempts())
    } else {
        fmt.Printf("Failed: %v (attempts: %d)\n", result.Err(), result.Attempts())
    }
}
```

## Error Handling

### Standard Errors (Default Behavior)

By default, standard errors are automatically retried with exponential backoff:

```go
fn := func() (string, error) {
    // Standard error - will be auto-retried
    return "", errors.New("connection timeout")
}
```

### Custom Retry Behavior

For fine-grained control, implement the `RetryableError` interface:

```go
type NetworkError struct {
    msg       string
    transient bool
}

func (e *NetworkError) Error() string {
    return e.msg
}

func (e *NetworkError) RetryPolicy() retrier.RetryPolicy {
    if e.transient {
        return retrier.RetryPolicyAuto    // Auto retry with backoff
    }
    return retrier.RetryPolicyNever       // Don't retry
}

// Usage
fn := func() (string, error) {
    // This error will be auto-retried because it implements RetryPolicyAuto
    return "", &NetworkError{msg: "timeout", transient: true}
}
```

### Default Retry Policy

Configure the default behavior for standard errors via `RetryParam.DefaultRetryPolicy`:

```go
// Fail-fast mode: Only retry errors that explicitly implement RetryableError with RetryPolicyAuto
params := retrier.RetryParam{
    Jitter:              50 * time.Millisecond,
    MaxAttempts:         5,
    BackoffParam:        backoffParam,
    DefaultRetryPolicy:  retrier.RetryPolicyNever,  // Standard errors won't be retried
}
```

## Retry Policies

| Policy | Description |
|--------|-------------|
| `RetryPolicyAuto` | Error will be retried automatically with exponential backoff |
| `RetryPolicyManual` | Error should not be auto-retried, but is eligible for manual retry |
| `RetryPolicyNever` | Permanent failure, should not be retried at all |

### Behavior Matrix

| Error Type | `DefaultRetryPolicy=Auto` (default) | `DefaultRetryPolicy=Never` |
|------------|-------------------------------------|----------------------------|
| Standard `error` | ✅ Auto-retry | ❌ Stop immediately |
| `RetryableError(Auto)` | ✅ Auto-retry | ✅ Auto-retry |
| `RetryableError(Manual)` | ❌ Stop immediately | ❌ Stop immediately |
| `RetryableError(Never)` | ❌ Stop immediately | ❌ Stop immediately |

**Key insight**: `RetryableError` always takes precedence over `DefaultRetryPolicy`.

## Context Cancellation

The retry operation respects context cancellation. If the context is cancelled during a backoff delay, the operation stops immediately:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result := retrier.Retry(ctx, params, logger, fn)

if errors.Is(result.Err(), context.Canceled) {
    // Handle cancellation
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

Use `retrier.NewNoOpLogger()` for zero-overhead when logging is not needed.

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

// RetryableError interface for fine-grained retry control
// Optional: standard errors use DefaultRetryPolicy
type RetryableError interface {
    error
    RetryPolicy() RetryPolicy
}

// RetryParam holds retry configuration
type RetryParam struct {
    Jitter              time.Duration
    MaxAttempts         int
    BackoffParam        BackoffParam
    DefaultRetryPolicy  RetryPolicy  // Default: RetryPolicyAuto
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
// Accepts standard error - works with any function!
func Retry[T any](ctx context.Context, retryParam RetryParam, logger DebugLogger, fn func() (T, error)) Result[T]

// NewRetryParam creates retry configuration with DefaultRetryPolicy=Auto
func NewRetryParam(jitter time.Duration, maxAttempts int, backoffParam BackoffParam) RetryParam

// NewBackoffParam creates backoff configuration
func NewBackoffParam(initialDuration time.Duration, multiplier float64, maxDuration time.Duration) BackoffParam

// NewNoOpLogger creates a no-op logger (zero overhead)
func NewNoOpLogger() *NoOpLogger

// NewRetryError creates a retry error (use when you need explicit retry control)
func NewRetryError(cause RetryErrorCause, message string, policy RetryPolicy, wrapped error) *RetryError
```

## License

MIT License - see [LICENSE](LICENSE) for details.
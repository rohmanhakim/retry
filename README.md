# retrier

[![codecov](https://codecov.io/github/rohmanhakim/retrier/branch/master/graph/badge.svg?token=eYWKQEsSzz)](https://codecov.io/github/rohmanhakim/retrier)
[![Go Reference](https://pkg.go.dev/badge/github.com/rohmanhakim/retry.svg)](https://pkg.go.dev/github.com/rohmanhakim/retry)

A simple, standalone Go package for retrying operations with exponential backoff and jitter support.

## Features

- **Generic API**: Works with any return type using Go generics
- **Zero Friction**: Accepts standard `error` - works seamlessly with stdlib and third-party packages
- **Functional Options**: Clean, self-documenting configuration with sensible defaults
- **Exponential Backoff**: Configurable backoff with multiplier and max duration
- **Jitter Support**: Add randomness to backoff delays to avoid thundering herd
- **Flexible Retry Policies**: Control which errors should be retried automatically
- **Default Retry Policy**: Configure default behavior for standard errors
- **Context Cancellation**: Support for graceful cancellation during backoff delays
- **Debug Logging**: Optional logging interface for observability
- **Server-Suggested Delay**: Respect server backoff hints (e.g., HTTP `Retry-After`, gRPC `retry-info`)

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
    // Define the operation to retry - just return standard errors!
    fn := func() (*http.Response, error) {
        return http.Get("https://api.example.com/data")
    }
    
    // Execute with retry using functional options
    ctx := context.Background()
    result := retrier.Retry(ctx, retrier.NewNoOpLogger(), fn,
        retrier.WithMaxAttempts(5),
        retrier.WithJitter(100*time.Millisecond),
        retrier.WithInitialDuration(1*time.Second),
        retrier.WithMultiplier(2.0),
        retrier.WithMaxDuration(30*time.Second),
    )
    
    // Style 1: Idiomatic Go (multiple return values)
    value, attempts, err := result.Decompose()
    if err != nil {
        fmt.Printf("Failed after %d attempts: %v\n", attempts, err)
        return
    }
    fmt.Printf("Success: %v (attempts: %d)\n", value, attempts)
}
```

### Result Methods

The `Result[T]` type provides multiple ways to access the outcome:

#### Decompose() - Idiomatic Go

Returns a tuple for traditional error handling:

```go
value, attempts, err := retrier.Retry(ctx, logger, fn).Decompose()
if err != nil {
    return fmt.Errorf("failed after %d attempts: %w", attempts, err)
}
```

#### UnwrapOr() - Fallback Pattern

Returns the value or a default if failed. Perfect for configuration loading:

```go
// Beautiful, safe fallback logic in one line
cacheTTL := retrier.Retry(ctx, logger, fetchRemoteConfig).UnwrapOr(defaultTTL)
```

#### Unwrap() - Panic on Failure

Returns the value or panics. Use only when failure should crash:

```go
value := retrier.Retry(ctx, logger, fn).Unwrap()
```

#### Classic Methods

```go
result := retrier.Retry(ctx, logger, fn)
result.IsSuccess()   // bool
result.IsFailure()   // bool
result.Value()       // T (zero value if failed)
result.Err()         // error (nil if succeeded)
result.Attempts()    // int
```

## Configuration Options

### Functional Options

The `Retry` function accepts functional options for configuration. All options have sensible defaults:

| Option | Description | Default |
|--------|-------------|---------|
| `WithMaxAttempts(n int)` | Maximum number of retry attempts | 3 |
| `WithJitter(d time.Duration)` | Random delay added to backoff | 0 (no jitter) |
| `WithInitialDuration(d time.Duration)` | Initial backoff duration | 1 second |
| `WithMultiplier(m float64)` | Backoff multiplier | 2.0 |
| `WithMaxDuration(d time.Duration)` | Maximum backoff duration | 1 minute |
| `WithRetryPolicy(p RetryPolicy)` | Default retry policy for standard errors | RetryPolicyAuto |
| `WithLogAttrs(attrs ...any)` | Additional attributes for structured logging | none |

### Using Defaults

```go
// Simple case - just want retries with defaults (3 attempts, 1s initial backoff, 2x multiplier)
result := retrier.Retry(ctx, logger, fn)
```

### Custom Configuration

```go
// All options customized
result := retrier.Retry(ctx, logger, fn,
    retrier.WithMaxAttempts(10),
    retrier.WithJitter(50*time.Millisecond),
    retrier.WithInitialDuration(100*time.Millisecond),
    retrier.WithMultiplier(1.5),
    retrier.WithMaxDuration(5*time.Minute),
    retrier.WithRetryPolicy(retrier.RetryPolicyNever),
)
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

Configure the default behavior for standard errors:

```go
// Fail-fast mode: Only retry errors that explicitly implement RetryableError with RetryPolicyAuto
result := retrier.Retry(ctx, logger, fn,
    retrier.WithRetryPolicy(retrier.RetryPolicyNever),  // Standard errors won't be retried
)
```

### Server-Suggested Delay

For protocols that communicate backoff delays (like HTTP 429 with `Retry-After`), implement the `DelaySuggestioner` interface:

```go
type RateLimitError struct {
    StatusCode  int
    RetryAfter  time.Duration
}

func (e *RateLimitError) Error() string {
    return fmt.Sprintf("HTTP %d: Rate limited", e.StatusCode)
}

func (e *RateLimitError) RetryPolicy() retrier.RetryPolicy {
    return retrier.RetryPolicyAuto
}

func (e *RateLimitError) SuggestedDelay() time.Duration {
    return e.RetryAfter
}

// Usage - the backoff will respect the server's suggested delay
fn := func() (*http.Response, error) {
    resp, err := http.Get("https://api.example.com")
    if err != nil {
        return nil, err
    }
    if resp.StatusCode == 429 {
        retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
        resp.Body.Close()
        return nil, &RateLimitError{StatusCode: 429, RetryAfter: retryAfter}
    }
    return resp, nil
}
```

The delay calculation uses `max(serverDelay, calculatedBackoff)` for the initial attempt, ensuring the server's suggestion is respected while still applying exponential backoff for subsequent retries.

## Retry Policies

| Policy | Description |
|--------|-------------|
| `RetryPolicyAuto` | Error will be retried automatically with exponential backoff |
| `RetryPolicyManual` | Error should not be auto-retried, but is eligible for manual retry |
| `RetryPolicyNever` | Permanent failure, should not be retried at all |

### Behavior Matrix

| Error Type | `DefaultRetryPolicy=Auto` (default) | `DefaultRetryPolicy=Never` |
|------------|-------------------------------------|----------------------------|
| Standard `error` | Auto-retry | Stop immediately |
| `RetryableError(Auto)` | Auto-retry | Auto-retry |
| `RetryableError(Manual)` | Stop immediately | Stop immediately |
| `RetryableError(Never)` | Stop immediately | Stop immediately |

**Key insight**: `RetryableError` always takes precedence over `DefaultRetryPolicy`.

## Context Cancellation

The retry operation respects context cancellation. If the context is cancelled during a backoff delay, the operation stops immediately:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result := retrier.Retry(ctx, logger, fn,
    retrier.WithMaxAttempts(10),
    retrier.WithInitialDuration(1*time.Second),
)

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

func (l *MyLogger) LogRetry(ctx context.Context, attempt, maxAttempts int, backoff time.Duration, err error, attrs ...any) {
    if err != nil {
        log.Printf("Retry %d/%d: backoff=%v, error=%v", attempt, maxAttempts, backoff, err)
    } else {
        log.Printf("Success on attempt %d/%d", attempt, maxAttempts)
    }
}
```

### Using Attributes

The `attrs` parameter follows Go's `slog` convention for structured logging - it accepts alternating key-value pairs. This allows you to add custom context to your log entries:

```go
import (
    "log/slog"
    "time"
)

// SlogLogger integrates with Go's structured logging (Go 1.21+)
type SlogLogger struct {
    logger *slog.Logger
}

func (l *SlogLogger) Enabled() bool {
    return true
}

func (l *SlogLogger) LogRetry(ctx context.Context, attempt, maxAttempts int, backoff time.Duration, err error, attrs ...any) {
    // Build log attributes
    args := []any{
        "attempt", attempt,
        "max_attempts", maxAttempts,
        "backoff", backoff.Milliseconds(),
    }
    
    // Append user-provided attributes (if any)
    args = append(args, attrs...)
    
    if err != nil {
        args = append(args, "error", err.Error())
        l.logger.Warn("retry attempt", args...)
    } else {
        l.logger.Info("retry success", args...)
    }
}
```

You can pass additional context when implementing your own logging:

```go
// In your custom logger implementation, you can enrich logs with:
// - operation names
// - timing information
// - request IDs
// - any custom metadata

func (l *MyLogger) LogRetry(ctx context.Context, attempt, maxAttempts int, backoff time.Duration, err error, attrs ...any) {
    // attrs might contain: "operation", "db_query", "request_id", "abc123"
    // Process attrs as alternating key-value pairs
    fields := make(map[string]any)
    for i := 0; i < len(attrs)-1; i += 2 {
        if key, ok := attrs[i].(string); ok {
            fields[key] = attrs[i+1]
        }
    }
    // Now fields = {"operation": "db_query", "request_id": "abc123"}
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

// DelaySuggestioner interface for server-suggested backoff delays
// Useful for HTTP 429 Retry-After, gRPC retry-info, etc.
type DelaySuggestioner interface {
    error
    SuggestedDelay() time.Duration
}

// Result holds the outcome of a retry operation
type Result[T any] struct {
    // Contains value on success, zero value on failure
    // Contains error on failure, nil on success
    // Number of attempts made
}

// Result methods
func (r Result[T]) Decompose() (T, int, error)  // Returns tuple for idiomatic Go
func (r Result[T]) UnwrapOr(default T) T        // Returns value or default
func (r Result[T]) Unwrap() T                   // Returns value or panics
func (r Result[T]) IsSuccess() bool             // true if succeeded
func (r Result[T]) IsFailure() bool             // true if failed
func (r Result[T]) Value() T                    // value (zero if failed)
func (r Result[T]) Err() error                  // error (nil if succeeded)
func (r Result[T]) Attempts() int               // number of attempts

// DebugLogger interface for logging
type DebugLogger interface {
    Enabled() bool
    LogRetry(ctx context.Context, attempt, maxAttempts int, backoff time.Duration, err error, attrs ...any)
}
```

### Functions

```go
// Retry executes fn with retry logic
// Accepts standard error - works with any function!
func Retry[T any](ctx context.Context, logger DebugLogger, fn func() (T, error), opts ...RetryOption) Result[T]

// Functional options
func WithMaxAttempts(n int) RetryOption
func WithJitter(d time.Duration) RetryOption
func WithInitialDuration(d time.Duration) RetryOption
func WithMultiplier(m float64) RetryOption
func WithMaxDuration(d time.Duration) RetryOption
func WithRetryPolicy(p RetryPolicy) RetryOption
func WithLogAttrs(attrs ...any) RetryOption

// NewNoOpLogger creates a no-op logger (zero overhead)
func NewNoOpLogger() *NoOpLogger

// NewRetryError creates a retry error (use when you need explicit retry control)
func NewRetryError(cause RetryErrorCause, message string, policy RetryPolicy, wrapped error) *RetryError
```

## Examples

### Basic Retry with Backoff

A complete, runnable example is available in the `example/fetch/` directory that demonstrates:

- **Fake HTTP Server**: A server that simulates failures on initial attempts, then succeeds
- **Retry with Backoff**: Exponential backoff with jitter for failed requests
- **Custom Logger**: A simple logger implementation showing retry progress

```bash
cd example/fetch && go run .
```

### Rate Limit with Retry-After

Located in `example/rate_limit/` - demonstrates handling HTTP 429 responses with `Retry-After` headers:

- **429 Rate Limit Responses**: Server returns rate limit errors with `Retry-After` header
- **DelaySuggestioner Interface**: Custom error type implementing both `RetryableError` and `DelaySuggestioner`
- **Server Delay Respected**: Backoff uses `max(serverDelay, calculatedBackoff)`

```bash
cd example/rate_limit && go run .
```

### Example Output (Basic)

```
=== Retrier Example: HTTP Fetch with Retry ===

Fake server started on http://localhost:8080
Will fail 3 attempt(s) before succeeding...

 Received request #1
[15:26:07.616] Attempt 1/4 failed: HTTP 500: Simulated failure - attempt 1
[15:26:07.616]    ↳ Retrying in 503.052992ms...
 Received request #2
[15:26:08.120] Attempt 2/4 failed: HTTP 500: Simulated failure - attempt 2
[15:26:08.120]    ↳ Retrying in 1.03579938s...
 Received request #3
[15:26:09.156] Attempt 3/4 failed: HTTP 500: Simulated failure - attempt 3
[15:26:09.156]    ↳ Retrying in 2.039603414s...
 Received request #4
[15:26:11.197] Success on attempt 4/4

=== Result ===
 Success after 4 attempt(s)
 Response: Success after 4 attempts!
```

## License

MIT License - see [LICENSE](LICENSE) for details.

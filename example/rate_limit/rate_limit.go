// Package main demonstrates handling HTTP 429 Rate Limit responses with Retry-After headers.
// Run with: go run .
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/rohmanhakim/retrier"
)

// RateLimitError implements both retrier.RetryableError and retrier.DelaySuggestioner.
// This allows the retry mechanism to:
// 1. Know the error is retryable (RetryPolicy)
// 2. Respect the server's suggested delay (SuggestedDelay from Retry-After header)
type RateLimitError struct {
	StatusCode int
	RetryAfter time.Duration
	Body       string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

// RetryPolicy returns RetryPolicyAuto for rate limit errors (429).
func (e *RateLimitError) RetryPolicy() retrier.RetryPolicy {
	return retrier.RetryPolicyAuto
}

// SuggestedDelay returns the delay suggested by the Retry-After header.
func (e *RateLimitError) SuggestedDelay() time.Duration {
	return e.RetryAfter
}

// parseRetryAfter parses the Retry-After header value.
// It supports both delta-seconds and HTTP-date formats.
// For simplicity, this example only handles delta-seconds.
func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}

	// Try parsing as seconds (delta-seconds format)
	if secs, err := strconv.Atoi(value); err == nil {
		return time.Duration(secs) * time.Second
	}

	// TODO: Handle HTTP-date format if needed
	// Example: "Fri, 31 Dec 1999 23:59:59 GMT"
	return 0
}

// SimpleLogger is a minimal logger for demonstration
type SimpleLogger struct{}

func (l *SimpleLogger) Enabled() bool { return true }

func (l *SimpleLogger) LogRetry(_ context.Context, attempt, maxAttempts int, backoff time.Duration, err error, _ ...any) {
	now := time.Now().Format("15:04:05.000")
	if err != nil {
		fmt.Printf("[%s] Attempt %d/%d failed: %v\n", now, attempt, maxAttempts, err)
		if dl, ok := err.(retrier.DelaySuggestioner); ok && dl.SuggestedDelay() > 0 {
			fmt.Printf("[%s]    ↳ Server suggested delay: %v (Retry-After)\n", now, dl.SuggestedDelay())
		}
		fmt.Printf("[%s]    ↳ Retrying in %v...\n", now, backoff)
	} else {
		fmt.Printf("[%s] Success on attempt %d/%d\n", now, attempt, maxAttempts)
	}
}

func main() {
	fmt.Println("=== Retrier Example: Rate Limit with Retry-After ===")
	fmt.Println()

	// Start the fake server
	server := NewFakeServer(":8081", 2) // Rate limit 2 requests, succeed on 3rd
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create a logger to see retry progress
	logger := &SimpleLogger{}

	// Define the fetch operation
	fetchURL := "http://localhost:8081/"
	fn := func() (*http.Response, error) {
		resp, err := http.Get(fetchURL)
		if err != nil {
			return nil, err
		}

		// Handle non-2xx status codes
		if resp.StatusCode == http.StatusTooManyRequests {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			return nil, &RateLimitError{
				StatusCode: resp.StatusCode,
				RetryAfter: retryAfter,
				Body:       string(body),
			}
		}

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		return resp, nil
	}

	// Execute with retry
	// Note: InitialDuration is 500ms, but Retry-After is 2s
	// The retry will wait 2s (max of both) on rate-limited responses
	ctx := context.Background()
	result := retrier.Retry(ctx, logger, fn,
		retrier.WithMaxAttempts(5),
		retrier.WithInitialDuration(500*time.Millisecond),
		retrier.WithMultiplier(2.0),
		retrier.WithJitter(100*time.Millisecond),
	)

	fmt.Println()
	fmt.Println("=== Result ===")

	response, attempts, err := result.Decompose()
	if err != nil {
		fmt.Printf(" Failed after %d attempts: %v\n", attempts, err)
	} else {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		fmt.Printf(" Success after %d attempt(s)\n", attempts)
		fmt.Printf(" Response: %s\n", string(body))
	}

	// Cleanup
	if err := server.Stop(); err != nil {
		fmt.Printf("Warning: failed to stop server: %v\n", err)
	}
}

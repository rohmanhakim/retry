// Package main demonstrates the retrier package with a fake HTTP server.
// Run with: go run .
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rohmanhakim/retrier"
	"github.com/rohmanhakim/retrier/example/fetch/simple_logger"
)

func main() {
	fmt.Println("=== Retrier Example: HTTP Fetch with Retry ===")
	fmt.Println()

	// Start the fake server in the background
	server := NewFakeServer(":8080", 3) // Fail 3 times, succeed on 4th
	if err := server.Start(); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		return
	}

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Create a simple logger to see retry progress
	logger := simple_logger.NewSimpleLogger()

	// Define the fetch operation to retry
	// Note: http.Get doesn't return error for non-200 status codes,
	// so we need to check the status code ourselves
	fetchURL := "http://localhost:8080/"
	fn := func() (*http.Response, error) {
		resp, err := http.Get(fetchURL)
		if err != nil {
			return nil, err
		}
		// Treat non-2xx status codes as errors for retry purposes
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		return resp, nil
	}

	// Execute with retry
	ctx := context.Background()
	result := retrier.Retry(ctx, logger, fn,
		retrier.WithMaxAttempts(4),                                      // Try up to 4 times
		retrier.WithInitialDuration(500*time.Millisecond),               // Start with 500ms backoff
		retrier.WithMultiplier(2.0),                                     // Double the backoff each time
		retrier.WithJitter(100*time.Millisecond),                        // Add some randomness
		retrier.WithLogAttrs("operation", "fetch_url", "url", fetchURL), // Add context to logs
	)

	fmt.Println()
	fmt.Println("=== Result ===")

	// Use Decompose for idiomatic error handling
	response, attempts, err := result.Decompose()
	if err != nil {
		fmt.Printf("❌ Failed after %d attempts: %v\n", attempts, err)
		return
	}

	// Read and print the response body
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		fmt.Printf("❌ Failed to read response: %v\n", err)
		return
	}

	fmt.Printf("✅ Success after %d attempt(s)\n", attempts)
	fmt.Printf("📄 Response: %s\n", string(body))

	// Cleanup
	if err := server.Stop(); err != nil {
		fmt.Printf("Warning: failed to stop server: %v\n", err)
	}
}

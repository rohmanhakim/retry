// Package main demonstrates handling HTTP 429 Rate Limit responses with Retry-After headers.
package main

import (
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
)

// FakeServer simulates an HTTP server that returns 429 rate limit responses
// with Retry-After headers before eventually succeeding.
type FakeServer struct {
	addr         string
	failCount    int32 // Number of rate limit responses before success
	requestCount int32
	server       *http.Server
}

// NewFakeServer creates a new fake server.
// failCount specifies how many requests should receive 429 responses.
func NewFakeServer(addr string, failCount int) *FakeServer {
	return &FakeServer{
		addr:      addr,
		failCount: int32(failCount),
	}
}

// Start begins the fake server.
func (s *FakeServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		fmt.Printf("Fake server started on http://localhost%s\n", s.addr)
		fmt.Printf("Will rate-limit %d request(s) with Retry-After: 2s...\n", s.failCount)
	}()

	return s.server.ListenAndServe()
}

// Stop shuts down the server.
func (s *FakeServer) Stop() error {
	return s.server.Close()
}

func (s *FakeServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	count := atomic.AddInt32(&s.requestCount, 1)
	fmt.Printf("\n Received request #%d\n", count)

	// Check if we should still rate limit
	if count <= s.failCount {
		retryAfterSecs := 2
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSecs))
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintf(w, "Rate limited - retry after %d seconds", retryAfterSecs)
		fmt.Printf("   ↳ Returning 429 with Retry-After: %ds\n", retryAfterSecs)
		return
	}

	// Success!
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Success after %d attempts!", count)
	fmt.Printf("   ↳ Returning 200 OK\n")
}

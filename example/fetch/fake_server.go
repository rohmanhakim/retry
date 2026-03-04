// Package main contains a fake HTTP server for demonstrating the retrier package.
package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// FakeServer simulates a flaky HTTP server that returns errors on initial attempts
// and succeeds on the final attempt.
type FakeServer struct {
	port         string
	attemptCount atomic.Int32
	failUntil    int32 // Number of attempts to fail before succeeding
	server       *http.Server
}

// NewFakeServer creates a new fake server instance.
// failUntil specifies how many attempts should fail before returning success.
func NewFakeServer(port string, failUntil int) *FakeServer {
	return &FakeServer{
		port:      port,
		failUntil: int32(failUntil),
	}
}

// Start begins the fake server in a goroutine.
// Returns immediately; the server runs in the background.
func (s *FakeServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.server = &http.Server{
		Addr:    s.port,
		Handler: mux,
	}

	go func() {
		fmt.Printf("🚀 Fake server started on http://localhost%s\n", s.port)
		fmt.Printf("   Will fail %d attempt(s) before succeeding...\n\n", s.failUntil)
		// We ignore the error since it's expected when server closes
		s.server.ListenAndServe()
	}()

	return nil
}

// handleRequest handles incoming HTTP requests.
// Returns 500 for the first N attempts, then 200 for subsequent requests.
func (s *FakeServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	attempt := s.attemptCount.Add(1)
	attemptNum := int(attempt)

	fmt.Printf("📥 Received request #%d\n", attemptNum)

	if attempt <= s.failUntil {
		// Return error for first N attempts
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Simulated failure - attempt %d", attemptNum)))
		return
	}

	// Return success after N failures
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Success after %d attempts!", attemptNum)))
}

// Stop gracefully shuts down the server.
func (s *FakeServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// Reset resets the attempt counter for testing purposes.
func (s *FakeServer) Reset() {
	s.attemptCount.Store(0)
}

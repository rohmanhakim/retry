package retrier_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	retrier "github.com/rohmanhakim/retrier"
)

// noopLogger is a NoOpLogger instance for tests
var noopLogger = retrier.NewNoOpLogger()

// defaultBackoffParam returns a default backoff parameter for tests
func defaultBackoffParam() retrier.BackoffParam {
	return retrier.NewBackoffParam(
		10*time.Millisecond,
		2.0,
		30*time.Second,
	)
}

// mockError is a mock implementation of retrier.RetryableError for testing
type mockError struct {
	msg       string
	retryable bool
}

func (m *mockError) Error() string {
	return m.msg
}

func (m *mockError) RetryPolicy() retrier.RetryPolicy {
	if m.retryable {
		return retrier.RetryPolicyAuto
	}
	return retrier.RetryPolicyManual
}

// mockLogger is a mock implementation of retrier.DebugLogger for testing
type mockLogger struct {
	enabled       bool
	logRetryCalls []logRetryCall
}

type logRetryCall struct {
	attempt    int
	maxAttempt int
	backoff    time.Duration
	err        error
}

func newMockLogger(enabled bool) *mockLogger {
	return &mockLogger{
		enabled:       enabled,
		logRetryCalls: make([]logRetryCall, 0),
	}
}

func (m *mockLogger) Enabled() bool {
	return m.enabled
}

func (m *mockLogger) LogRetry(_ context.Context, attempt int, maxAttempt int, backoff time.Duration, err error) {
	m.logRetryCalls = append(m.logRetryCalls, logRetryCall{
		attempt:    attempt,
		maxAttempt: maxAttempt,
		backoff:    backoff,
		err:        err,
	})
}

// TestRetry_SuccessOnFirstAttempt verifies that a successful function returns immediately
func TestRetry_SuccessOnFirstAttempt(t *testing.T) {
	mock := newMockLogger(true)
	callCount := 0
	fn := func() (string, retrier.RetryableError) {
		callCount++
		return "success", nil
	}

	params := retrier.NewRetryParam(
		10*time.Millisecond,
		3,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, mock, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() != "success" {
		t.Fatalf("expected 'success', got: %s", result.Value())
	}
	if result.Attempts() != 1 {
		t.Fatalf("expected 1 attempt, got: %d", result.Attempts())
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got: %d", callCount)
	}

	// Debug logging assertions
	if len(mock.logRetryCalls) != 1 {
		t.Fatalf("expected 1 retry log call, got %d", len(mock.logRetryCalls))
	}
	call := mock.logRetryCalls[0]
	if call.attempt != 1 {
		t.Errorf("expected attempt=1, got %d", call.attempt)
	}
	if call.maxAttempt != 3 {
		t.Errorf("expected maxAttempt=3, got %d", call.maxAttempt)
	}
	if call.backoff != 0 {
		t.Errorf("expected backoff=0 for success, got %v", call.backoff)
	}
	if call.err != nil {
		t.Errorf("expected err=nil for success, got %v", call.err)
	}
}

func TestRetry_PassParameter(t *testing.T) {
	toPrint := "Hello"
	callCount := 0

	fn := func() (string, retrier.RetryableError) {
		callCount++
		return fmt.Sprintf("%s, world!", toPrint), nil
	}

	params := retrier.NewRetryParam(
		10*time.Millisecond,
		3,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, noopLogger, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() != "Hello, world!" {
		t.Fatalf("expected 'Hello, world!', got: %s", result.Value())
	}
	if result.Attempts() != 1 {
		t.Fatalf("expected 1 attempt, got: %d", result.Attempts())
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got: %d", callCount)
	}
}

// TestRetry_SuccessAfterRetries verifies that retryable errors lead to retries until success
func TestRetry_SuccessAfterRetries(t *testing.T) {
	mock := newMockLogger(true)
	callCount := 0
	fn := func() (string, retrier.RetryableError) {
		callCount++
		if callCount < 3 {
			return "", &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return "success", nil
	}

	params := retrier.NewRetryParam(
		5*time.Millisecond,
		5,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, mock, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() != "success" {
		t.Fatalf("expected 'success', got: %s", result.Value())
	}
	if result.Attempts() != 3 {
		t.Fatalf("expected 3 attempts, got: %d", result.Attempts())
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls, got: %d", callCount)
	}

	// Debug logging assertions
	// Should have 3 entries: 2 failed attempts with backoff + 1 success
	if len(mock.logRetryCalls) != 3 {
		t.Fatalf("expected 3 retry log calls, got %d", len(mock.logRetryCalls))
	}
	// First two entries should have backoff and error
	for i := 0; i < 2; i++ {
		if mock.logRetryCalls[i].backoff == 0 {
			t.Errorf("entry %d: expected non-zero backoff for failed attempt", i)
		}
		if mock.logRetryCalls[i].err == nil {
			t.Errorf("entry %d: expected error for failed attempt", i)
		}
	}
	// Last entry (success) should have no backoff and no error
	lastCall := mock.logRetryCalls[2]
	if lastCall.backoff != 0 {
		t.Errorf("expected backoff=0 for success, got %v", lastCall.backoff)
	}
	if lastCall.err != nil {
		t.Errorf("expected err=nil for success, got %v", lastCall.err)
	}
}

// TestRetry_NonRetryableErrorReturnsImmediately verifies that non-retryable errors return immediately
func TestRetry_NonRetryableErrorReturnsImmediately(t *testing.T) {
	mock := newMockLogger(true)
	callCount := 0
	expectedErr := &mockError{
		msg:       "fatal error",
		retryable: false,
	}

	fn := func() (string, retrier.RetryableError) {
		callCount++
		return "", expectedErr
	}

	params := retrier.NewRetryParam(
		10*time.Millisecond,
		5,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, mock, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}
	if result.Value() != "" {
		t.Fatalf("expected empty result, got: %s", result.Value())
	}
	if result.Attempts() != 1 {
		t.Fatalf("expected 1 attempt, got: %d", result.Attempts())
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call for non-retryable error, got: %d", callCount)
	}
	if result.Err().Error() != expectedErr.Error() {
		t.Fatalf("expected error '%s', got: '%s'", expectedErr.Error(), result.Err().Error())
	}

	// Debug logging assertions - non-retryable errors should NOT trigger LogRetry
	if len(mock.logRetryCalls) != 0 {
		t.Errorf("expected no LogRetry calls for non-retryable error, got %d", len(mock.logRetryCalls))
	}
}

// TestRetry_ExhaustedAttempts verifies that retryable errors exhaust all attempts
func TestRetry_ExhaustedAttempts(t *testing.T) {
	mock := newMockLogger(true)
	callCount := 0
	fn := func() (int, retrier.RetryableError) {
		callCount++
		return 0, &mockError{
			msg:       "persistent transient error",
			retryable: true,
		}
	}

	maxAttempts := 3
	params := retrier.NewRetryParam(
		5*time.Millisecond,
		maxAttempts,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, mock, fn)

	if result.IsSuccess() {
		t.Fatal("expected error after exhausting attempts, got nil")
	}
	if result.Value() != 0 {
		t.Fatalf("expected zero result, got: %d", result.Value())
	}
	if result.Attempts() != maxAttempts {
		t.Fatalf("expected %d attempts, got: %d", maxAttempts, result.Attempts())
	}
	if callCount != maxAttempts {
		t.Fatalf("expected %d calls, got: %d", maxAttempts, callCount)
	}

	// Exhausted attempts should return RetryPolicyManual
	if result.Err().RetryPolicy() != retrier.RetryPolicyManual {
		t.Fatalf("expected error RetryPolicy to be 'RetryPolicyManual', got: %v", result.Err().RetryPolicy())
	}
	var retryErr *retrier.RetryError
	errors.As(result.Err(), &retryErr)
	if retryErr.Cause != retrier.ErrExhaustedAttempts {
		t.Fatalf("expected error cause 'ErrExhaustedAttempts', got: '%s'", retryErr.Cause)
	}

	// Debug logging assertions
	// Should have maxAttempts entries
	if len(mock.logRetryCalls) != maxAttempts {
		t.Fatalf("expected %d retry log calls, got %d", maxAttempts, len(mock.logRetryCalls))
	}
	// Last entry should be the exhausted one with error
	lastCall := mock.logRetryCalls[maxAttempts-1]
	if lastCall.err == nil {
		t.Error("expected error for exhausted attempt")
	}
}

// TestRetry_MaxAttemptsLessThanOne verifies that MaxAttempts < 1 returns an error
func TestRetry_MaxAttemptsLessThanOne(t *testing.T) {
	mock := newMockLogger(true)
	fn := func() (string, retrier.RetryableError) {
		return "success", nil
	}

	params := retrier.NewRetryParam(
		10*time.Millisecond,
		0,
		defaultBackoffParam(),
	)

	var retryErr *retrier.RetryError
	result := retrier.Retry(context.Background(), params, mock, fn)

	if result.IsSuccess() {
		t.Fatal("expected error for MaxAttempts < 1, got nil")
	}
	// Zero attempt is a config error -> RetryPolicyNever
	if result.Err().RetryPolicy() != retrier.RetryPolicyNever {
		t.Fatalf("expected error RetryPolicy to be 'RetryPolicyNever', got: %v", result.Err().RetryPolicy())
	}
	errors.As(result.Err(), &retryErr)
	if retryErr.Cause != retrier.ErrZeroAttempt {
		t.Fatalf("expected error cause is ErrZeroAttempt, got %s", retryErr.Cause)
	}
	if result.Value() != "" {
		t.Fatalf("expected empty result, got: %s", result.Value())
	}
	if result.Attempts() != 0 {
		t.Fatalf("expected 0 attempts, got: %d", result.Attempts())
	}

	// Debug logging assertions - zero attempt should NOT trigger LogRetry
	if len(mock.logRetryCalls) != 0 {
		t.Errorf("expected no LogRetry calls for zero attempt config error, got %d", len(mock.logRetryCalls))
	}
}

// TestRetry_GenericTypePointer verifies that Retry works with pointer types
func TestRetry_GenericTypePointer(t *testing.T) {
	type Data struct {
		Value int
	}

	callCount := 0
	fn := func() (*Data, retrier.RetryableError) {
		callCount++
		if callCount < 2 {
			return nil, &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return &Data{Value: 42}, nil
	}

	params := retrier.NewRetryParam(
		5*time.Millisecond,
		3,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, noopLogger, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() == nil {
		t.Fatal("expected non-nil result, got nil")
	}
	if result.Value().Value != 42 {
		t.Fatalf("expected Value=42, got: %d", result.Value().Value)
	}
	if result.Attempts() != 2 {
		t.Fatalf("expected 2 attempts, got: %d", result.Attempts())
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got: %d", callCount)
	}
}

// TestRetry_GenericTypeSlice verifies that Retry works with slice types
func TestRetry_GenericTypeSlice(t *testing.T) {
	callCount := 0
	fn := func() ([]int, retrier.RetryableError) {
		callCount++
		if callCount < 2 {
			return nil, &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return []int{1, 2, 3}, nil
	}

	params := retrier.NewRetryParam(
		5*time.Millisecond,
		3,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, noopLogger, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if len(result.Value()) != 3 {
		t.Fatalf("expected 3 elements, got: %d", len(result.Value()))
	}
	if result.Attempts() != 2 {
		t.Fatalf("expected 2 attempts, got: %d", result.Attempts())
	}
}

// TestRetry_MixedRetryableAndNonRetryable verifies behavior with mixed error types
func TestRetry_MixedRetryableAndNonRetryable(t *testing.T) {
	callCount := 0
	fn := func() (string, retrier.RetryableError) {
		callCount++
		switch callCount {
		case 1:
			return "", &mockError{
				msg:       "retryable error 1",
				retryable: true,
			}
		case 2:
			return "", &mockError{
				msg:       "retryable error 2",
				retryable: true,
			}
		case 3:
			return "", &mockError{
				msg:       "non-retryable error",
				retryable: false,
			}
		default:
			return "success", nil
		}
	}

	params := retrier.NewRetryParam(
		5*time.Millisecond,
		5,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, noopLogger, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}
	if result.Value() != "" {
		t.Fatalf("expected empty result, got: %s", result.Value())
	}
	if result.Attempts() != 3 {
		t.Fatalf("expected 3 attempts, got: %d", result.Attempts())
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls (stops at non-retryable), got: %d", callCount)
	}
}

// TestRetry_BackoffDelayWithinBounds verifies that backoff delay with jitter is within expected bounds
func TestRetry_BackoffDelayWithinBounds(t *testing.T) {
	mock := newMockLogger(true)
	callCount := 0
	fn := func() (string, retrier.RetryableError) {
		callCount++
		if callCount < 3 {
			return "", &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return "success", nil
	}

	jitter := 5 * time.Millisecond
	params := retrier.NewRetryParam(
		jitter,
		5,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, mock, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}

	// Verify backoff delays are within expected bounds
	// Expected base delays: 10ms (attempt 1), 20ms (attempt 2)
	// With jitter: baseDelay <= actualDelay <= baseDelay + jitter
	expectedBaseDelays := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}

	for i := 0; i < 2; i++ {
		backoff := mock.logRetryCalls[i].backoff
		baseDelay := expectedBaseDelays[i]
		maxDelay := baseDelay + jitter

		if backoff < baseDelay {
			t.Errorf("Attempt %d: backoff %v is below minimum expected %v", i+1, backoff, baseDelay)
		}
		if backoff > maxDelay {
			t.Errorf("Attempt %d: backoff %v exceeds maximum expected %v", i+1, backoff, maxDelay)
		}
	}
}

// TestRetry_SuccessAfterManyFailures verifies eventual success after many retries
func TestRetry_SuccessAfterManyFailures(t *testing.T) {
	callCount := 0
	maxAttempts := 10
	fn := func() (string, retrier.RetryableError) {
		callCount++
		if callCount < maxAttempts {
			return "", &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return "eventual success", nil
	}

	params := retrier.NewRetryParam(
		2*time.Millisecond,
		maxAttempts,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, noopLogger, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() != "eventual success" {
		t.Fatalf("expected 'eventual success', got: %s", result.Value())
	}
	if result.Attempts() != maxAttempts {
		t.Fatalf("expected %d attempts, got: %d", maxAttempts, result.Attempts())
	}
	if callCount != maxAttempts {
		t.Fatalf("expected %d calls, got: %d", maxAttempts, callCount)
	}
}

// TestRetry_ExhaustedErrorIsRetryable verifies that exhausted attempt error has RetryPolicyManual
func TestRetry_ExhaustedErrorIsRetryable(t *testing.T) {
	fn := func() (string, retrier.RetryableError) {
		return "", &mockError{
			msg:       "persistent error",
			retryable: true,
		}
	}

	params := retrier.NewRetryParam(
		5*time.Millisecond,
		2,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, noopLogger, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}

	// Exhausted error should have RetryPolicyManual (eligible for manual retry)
	if result.Err().RetryPolicy() != retrier.RetryPolicyManual {
		t.Errorf("expected exhausted error to have RetryPolicyManual, got: %v", result.Err().RetryPolicy())
	}
}

// TestRetry_ErrorWrapping verifies that the original error is included
func TestRetry_ErrorWrapping(t *testing.T) {
	originalErr := &mockError{
		msg:       "original error message",
		retryable: true,
	}

	fn := func() (string, retrier.RetryableError) {
		return "", originalErr
	}

	params := retrier.NewRetryParam(
		5*time.Millisecond,
		2,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, noopLogger, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}

	if result.Err().Error() == "" {
		t.Error("expected non-empty error message")
	}

	// Verify the wrapped error can be accessed via Unwrap
	var retryErr *retrier.RetryError
	if errors.As(result.Err(), &retryErr) {
		// Unwrap should return the original error (the mockError wrapped in RetryError)
		unwrapped := retryErr.Unwrap()
		if unwrapped == nil {
			t.Error("expected wrapped error to be accessible via Unwrap")
		}
	}
}

// TestNewRetryParam verifies the constructor creates RetryParam correctly
func TestNewRetryParam(t *testing.T) {
	jitter := 50 * time.Millisecond
	maxAttempts := 5

	params := retrier.NewRetryParam(jitter, maxAttempts, defaultBackoffParam())

	callCount := 0
	fn := func() (string, retrier.RetryableError) {
		callCount++
		return "success", nil
	}

	result := retrier.Retry(context.Background(), params, noopLogger, fn)

	if result.IsFailure() {
		t.Fatalf("unexpected error: %v", result.Err())
	}
	if result.Value() != "success" {
		t.Fatalf("unexpected result: %s", result.Value())
	}
	if result.Attempts() != 1 {
		t.Fatalf("expected 1 attempt, got: %d", result.Attempts())
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got: %d", callCount)
	}
}

// BenchmarkRetry benchmarks the retry function
func BenchmarkRetry(b *testing.B) {
	fn := func() (int, retrier.RetryableError) {
		return 42, nil
	}

	params := retrier.NewRetryParam(
		1*time.Millisecond,
		3,
		defaultBackoffParam(),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retrier.Retry(context.Background(), params, noopLogger, fn)
	}
}

func TestRetry_NilErrorTypeSafety(t *testing.T) {
	fn := func() (string, retrier.RetryableError) {
		return "success", nil
	}

	params := retrier.NewRetryParam(
		5*time.Millisecond,
		3,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, noopLogger, fn)

	if result.IsFailure() {
		t.Fatalf("expected nil error, got: %v", result.Err())
	}
	if result.Value() != "success" {
		t.Fatalf("expected 'success', got: %s", result.Value())
	}
	if result.Attempts() != 1 {
		t.Fatalf("expected 1 attempt, got: %d", result.Attempts())
	}
}

func TestRetryErrorType(t *testing.T) {
	fn := func() (string, retrier.RetryableError) {
		return "", &mockError{
			msg:       "some error",
			retryable: true,
		}
	}

	params := retrier.NewRetryParam(
		5*time.Millisecond,
		1,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, noopLogger, fn)
	if result.IsSuccess() {
		t.Fatal("expected error after exhausting attempts")
	}
}

// TestRetry_DisabledLogger verifies that no logging occurs when logger is disabled
func TestRetry_DisabledLogger(t *testing.T) {
	mock := newMockLogger(false)

	callCount := 0
	fn := func() (string, retrier.RetryableError) {
		callCount++
		if callCount < 3 {
			return "", &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return "success", nil
	}

	params := retrier.NewRetryParam(
		5*time.Millisecond,
		5,
		defaultBackoffParam(),
	)

	result := retrier.Retry(context.Background(), params, mock, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}

	// Debug logging assertions - disabled logger should not record any entries
	if len(mock.logRetryCalls) != 0 {
		t.Errorf("expected 0 retry log calls when logger disabled, got %d", len(mock.logRetryCalls))
	}
}

// TestRetry_ContextCancellation verifies that context cancellation stops retry loop
func TestRetry_ContextCancellation(t *testing.T) {
	callCount := 0
	fn := func() (string, retrier.RetryableError) {
		callCount++
		return "", &mockError{
			msg:       "transient error",
			retryable: true,
		}
	}

	// Use a long backoff to ensure we can cancel during the wait
	params := retrier.NewRetryParam(
		5*time.Millisecond,
		10,
		retrier.NewBackoffParam(
			1*time.Second, // Long initial duration
			2.0,
			30*time.Second,
		),
	)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := retrier.Retry(ctx, params, noopLogger, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}

	// Should return ErrContextCancelled
	var retryErr *retrier.RetryError
	if !errors.As(result.Err(), &retryErr) {
		t.Fatalf("expected RetryError, got: %T", result.Err())
	}
	if retryErr.Cause != retrier.ErrContextCancelled {
		t.Fatalf("expected error cause 'ErrContextCancelled', got: '%s'", retryErr.Cause)
	}

	// Should have made at least one attempt but not all 10
	if result.Attempts() < 1 {
		t.Fatal("expected at least 1 attempt")
	}
	if result.Attempts() >= 10 {
		t.Fatal("expected fewer than 10 attempts due to cancellation")
	}

	// Verify it stopped early (callCount should equal attempts)
	if callCount != result.Attempts() {
		t.Fatalf("expected callCount (%d) to equal attempts (%d)", callCount, result.Attempts())
	}

	// Verify the wrapped error is context.Canceled
	unwrapped := retryErr.Unwrap()
	if unwrapped != context.Canceled {
		t.Fatalf("expected unwrapped error to be context.Canceled, got: %v", unwrapped)
	}
}

// TestRetry_JitterRandomness verifies that jitter produces different values across calls
// This test ensures the fix for the thundering herd problem works correctly
func TestRetry_JitterRandomness(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping randomness test in short mode")
	}

	// Collect backoff delays from multiple retry operations
	delays := make([]time.Duration, 0, 100)
	jitter := 10 * time.Millisecond
	params := retrier.NewRetryParam(
		jitter,
		2,
		retrier.NewBackoffParam(10*time.Millisecond, 2.0, 30*time.Second),
	)

	// Run multiple retry operations and collect first backoff delays
	for i := 0; i < 100; i++ {
		mock := newMockLogger(false)
		callCount := 0
		fn := func() (string, retrier.RetryableError) {
			callCount++
			if callCount == 1 {
				return "", &mockError{msg: "error", retryable: true}
			}
			return "success", nil
		}

		retrier.Retry(context.Background(), params, mock, fn)

		// Enable logging to capture backoff
		mock.enabled = true
		callCount = 0
		result := retrier.Retry(context.Background(), params, mock, fn)
		if result.IsSuccess() && len(mock.logRetryCalls) > 0 {
			delays = append(delays, mock.logRetryCalls[0].backoff)
		}
	}

	// Check that we got variation in delays (not all identical)
	uniqueDelays := make(map[time.Duration]int)
	for _, d := range delays {
		uniqueDelays[d]++
	}

	// With random jitter, we should see multiple unique values
	// If all values are the same, the jitter is deterministic (bug)
	if len(uniqueDelays) < 10 {
		t.Errorf("Expected at least 10 unique delay values, got %d. Jitter may not be random.", len(uniqueDelays))
	}

	// Verify all delays are within bounds [10ms, 20ms]
	for _, d := range delays {
		if d < 10*time.Millisecond || d > 20*time.Millisecond {
			t.Errorf("Delay %v is outside expected bounds [10ms, 20ms]", d)
		}
	}
}

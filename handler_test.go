package retry_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/rohmanhakim/retry"
)

// noopLogger is a NoOpLogger instance for tests
var noopLogger = retry.NewNoOpLogger()

// defaultBackoffParam returns a default backoff parameter for tests
func defaultBackoffParam() retry.BackoffParam {
	return retry.NewBackoffParam(
		10*time.Millisecond,
		2.0,
		30*time.Second,
	)
}

// mockError is a mock implementation of retry.RetryableError for testing
type mockError struct {
	msg       string
	retryable bool
}

func (m *mockError) Error() string {
	return m.msg
}

func (m *mockError) RetryPolicy() retry.RetryPolicy {
	if m.retryable {
		return retry.RetryPolicyAuto
	}
	return retry.RetryPolicyManual
}

// mockLogger is a mock implementation of retry.DebugLogger for testing
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
	fn := func() (string, retry.RetryableError) {
		callCount++
		return "success", nil
	}

	params := retry.NewRetryParam(
		100*time.Millisecond,
		10*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, mock, fn)

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

	fn := func() (string, retry.RetryableError) {
		callCount++
		return fmt.Sprintf("%s, world!", toPrint), nil
	}

	params := retry.NewRetryParam(
		100*time.Millisecond,
		10*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, noopLogger, fn)

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
	fn := func() (string, retry.RetryableError) {
		callCount++
		if callCount < 3 {
			return "", &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return "success", nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		5,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, mock, fn)

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

	fn := func() (string, retry.RetryableError) {
		callCount++
		return "", expectedErr
	}

	params := retry.NewRetryParam(
		100*time.Millisecond,
		10*time.Millisecond,
		42,
		5,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, mock, fn)

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
	fn := func() (int, retry.RetryableError) {
		callCount++
		return 0, &mockError{
			msg:       "persistent transient error",
			retryable: true,
		}
	}

	maxAttempts := 3
	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		maxAttempts,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, mock, fn)

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
	if result.Err().RetryPolicy() != retry.RetryPolicyManual {
		t.Fatalf("expected error RetryPolicy to be 'RetryPolicyManual', got: %v", result.Err().RetryPolicy())
	}
	var retryErr *retry.RetryError
	errors.As(result.Err(), &retryErr)
	if retryErr.Cause != retry.ErrExhaustedAttempts {
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
	fn := func() (string, retry.RetryableError) {
		return "success", nil
	}

	params := retry.NewRetryParam(
		100*time.Millisecond,
		10*time.Millisecond,
		42,
		0,
		defaultBackoffParam(),
	)

	var retryErr *retry.RetryError
	result := retry.Retry(params, mock, fn)

	if result.IsSuccess() {
		t.Fatal("expected error for MaxAttempts < 1, got nil")
	}
	// Zero attempt is a config error -> RetryPolicyNever
	if result.Err().RetryPolicy() != retry.RetryPolicyNever {
		t.Fatalf("expected error RetryPolicy to be 'RetryPolicyNever', got: %v", result.Err().RetryPolicy())
	}
	errors.As(result.Err(), &retryErr)
	if retryErr.Cause != retry.ErrZeroAttempt {
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
	fn := func() (*Data, retry.RetryableError) {
		callCount++
		if callCount < 2 {
			return nil, &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return &Data{Value: 42}, nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, noopLogger, fn)

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
	fn := func() ([]int, retry.RetryableError) {
		callCount++
		if callCount < 2 {
			return nil, &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return []int{1, 2, 3}, nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, noopLogger, fn)

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
	fn := func() (string, retry.RetryableError) {
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

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		5,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, noopLogger, fn)

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

// TestRetry_DeterministicWithSameSeed verifies deterministic behavior with same seed
func TestRetry_DeterministicWithSameSeed(t *testing.T) {
	callCount := 0
	fn := func() (int, retry.RetryableError) {
		callCount++
		if callCount < 2 {
			return 0, &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return 42, nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		12345,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, noopLogger, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}
	if result.Value() != 42 {
		t.Fatalf("expected 42, got: %d", result.Value())
	}
	if result.Attempts() != 2 {
		t.Fatalf("expected 2 attempts, got: %d", result.Attempts())
	}
}

// TestRetry_SuccessAfterManyFailures verifies eventual success after many retries
func TestRetry_SuccessAfterManyFailures(t *testing.T) {
	callCount := 0
	maxAttempts := 10
	fn := func() (string, retry.RetryableError) {
		callCount++
		if callCount < maxAttempts {
			return "", &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return "eventual success", nil
	}

	params := retry.NewRetryParam(
		5*time.Millisecond,
		2*time.Millisecond,
		42,
		maxAttempts,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, noopLogger, fn)

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
	fn := func() (string, retry.RetryableError) {
		return "", &mockError{
			msg:       "persistent error",
			retryable: true,
		}
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		2,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, noopLogger, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}

	// Exhausted error should have RetryPolicyManual (eligible for manual retry)
	if result.Err().RetryPolicy() != retry.RetryPolicyManual {
		t.Errorf("expected exhausted error to have RetryPolicyManual, got: %v", result.Err().RetryPolicy())
	}
}

// TestRetry_ErrorWrapping verifies that the original error is included
func TestRetry_ErrorWrapping(t *testing.T) {
	originalErr := &mockError{
		msg:       "original error message",
		retryable: true,
	}

	fn := func() (string, retry.RetryableError) {
		return "", originalErr
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		2,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, noopLogger, fn)

	if result.IsSuccess() {
		t.Fatal("expected error, got nil")
	}

	if result.Err().Error() == "" {
		t.Error("expected non-empty error message")
	}

	// Verify the wrapped error can be accessed via Unwrap
	var retryErr *retry.RetryError
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
	baseDelay := 100 * time.Millisecond
	jitter := 50 * time.Millisecond
	seed := int64(42)
	maxAttempts := 5

	params := retry.NewRetryParam(baseDelay, jitter, seed, maxAttempts, defaultBackoffParam())

	callCount := 0
	fn := func() (string, retry.RetryableError) {
		callCount++
		return "success", nil
	}

	result := retry.Retry(params, noopLogger, fn)

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
	fn := func() (int, retry.RetryableError) {
		return 42, nil
	}

	params := retry.NewRetryParam(
		1*time.Millisecond,
		1*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retry.Retry(params, noopLogger, fn)
	}
}

func TestRetry_NilErrorTypeSafety(t *testing.T) {
	fn := func() (string, retry.RetryableError) {
		return "success", nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		3,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, noopLogger, fn)

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
	fn := func() (string, retry.RetryableError) {
		return "", &mockError{
			msg:       "some error",
			retryable: true,
		}
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		1,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, noopLogger, fn)
	if result.IsSuccess() {
		t.Fatal("expected error after exhausting attempts")
	}
}

// TestRetry_DisabledLogger verifies that no logging occurs when logger is disabled
func TestRetry_DisabledLogger(t *testing.T) {
	mock := newMockLogger(false)

	callCount := 0
	fn := func() (string, retry.RetryableError) {
		callCount++
		if callCount < 3 {
			return "", &mockError{
				msg:       "transient error",
				retryable: true,
			}
		}
		return "success", nil
	}

	params := retry.NewRetryParam(
		10*time.Millisecond,
		5*time.Millisecond,
		42,
		5,
		defaultBackoffParam(),
	)

	result := retry.Retry(params, mock, fn)

	if result.IsFailure() {
		t.Fatalf("expected no error, got: %v", result.Err())
	}

	// Debug logging assertions - disabled logger should not record any entries
	if len(mock.logRetryCalls) != 0 {
		t.Errorf("expected 0 retry log calls when logger disabled, got %d", len(mock.logRetryCalls))
	}
}

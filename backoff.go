package retrier

import (
	"math"
	"math/rand"
	"time"
)

// computeJitter returns a pseudo-random duration between 0 and max (inclusive).
// Uses the global rand which is automatically seeded with a random value
// at startup (Go 1.20+), ensuring different jitter values across concurrent calls.
func computeJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(max)))
}

// exponentialBackoffDelay computes the delay for a given backoff count using
// exponential backoff with optional jitter.
//
// The formula is: initial * (multiplier ^ (count - 1)) + jitter
// First backoff (count=1): initialDuration
// Second backoff (count=2): initialDuration * multiplier
// And so on, capped at maxDuration.
func exponentialBackoffDelay(
	backoffCount int,
	jitter time.Duration,
	backoff backoffConfig,
) time.Duration {
	initialBackoff := backoff.initialDuration
	multiplier := backoff.multiplier
	maxBackoff := backoff.maxDuration

	// Compute exponential: initial * (multiplier ^ (count - 1))
	exponent := float64(backoffCount - 1)
	delay := float64(initialBackoff) * math.Pow(multiplier, exponent)
	if delay > float64(maxBackoff) {
		delay = float64(maxBackoff)
	}

	// Add jitter only if jitter > 0
	if jitter > 0 {
		jitterValue := computeJitter(jitter)
		delay += float64(jitterValue)
	}

	return time.Duration(delay)
}

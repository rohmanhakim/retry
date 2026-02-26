package retry

import (
	"math"
	"math/rand"
	"time"
)

// BackoffParam holds the parameters for exponential backoff calculation.
// Example usage:
//
//	params := NewBackoffParam(
//	    1 * time.Second,  // Start with 1s
//	    2.0,              // Double each time
//	    30 * time.Second, // Cap at 30s
//	)
type BackoffParam struct {
	initialDuration time.Duration
	multiplier      float64
	maxDuration     time.Duration
}

// NewBackoffParam creates a new BackoffParam with the given settings.
func NewBackoffParam(
	initialDuration time.Duration,
	multiplier float64,
	maxDuration time.Duration,
) BackoffParam {
	return BackoffParam{
		initialDuration: initialDuration,
		multiplier:      multiplier,
		maxDuration:     maxDuration,
	}
}

// InitialDuration returns the initial backoff duration.
func (b *BackoffParam) InitialDuration() time.Duration {
	return b.initialDuration
}

// Multiplier returns the backoff multiplier.
func (b *BackoffParam) Multiplier() float64 {
	return b.multiplier
}

// MaxDuration returns the maximum backoff duration.
func (b *BackoffParam) MaxDuration() time.Duration {
	return b.maxDuration
}

// computeJitter returns a pseudo-random duration between 0 and max (inclusive).
func computeJitter(max time.Duration, rng rand.Rand) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rng.Int63n(int64(max)))
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
	rng rand.Rand,
	backOffParam BackoffParam,
) time.Duration {
	initialBackoff := backOffParam.InitialDuration()
	multiplier := backOffParam.Multiplier()
	maxBackoff := backOffParam.MaxDuration()

	// Compute exponential: initial * (multiplier ^ (count - 1))
	exponent := float64(backoffCount - 1)
	delay := float64(initialBackoff) * math.Pow(multiplier, exponent)
	if delay > float64(maxBackoff) {
		delay = float64(maxBackoff)
	}

	// Add jitter only if jitter > 0
	if jitter > 0 {
		jitterValue := computeJitter(jitter, rng)
		delay += float64(jitterValue)
	}

	return time.Duration(delay)
}

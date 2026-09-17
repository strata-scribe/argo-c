package webhook

import (
	"math"
	"math/rand"
	"time"
)

// BackoffPolicy defines the strategy for calculating retry backoffs.
type BackoffPolicy string

const (
	// ConstantBackoff returns a constant delay across all retry attempts.
	ConstantBackoff BackoffPolicy = "constant"
	// LinearBackoff returns a delay that scales linearly with the attempt number.
	LinearBackoff BackoffPolicy = "linear"
	// ExponentialBackoff returns a delay that grows exponentially with the attempt number.
	ExponentialBackoff BackoffPolicy = "exponential"
)

// BackoffCalculator is responsible for calculating retry delays based on a defined policy.
type BackoffCalculator struct {
	Policy     BackoffPolicy
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	MaxRetries int
	Jitter     bool
}

// Calculate computes the backoff duration for a given attempt.
// It returns the calculated duration and a boolean indicating whether a retry should be performed (i.e., attempt <= MaxRetries).
func (b *BackoffCalculator) Calculate(attempt int) (time.Duration, bool) {
	if attempt > b.MaxRetries {
		return 0, false
	}

	var delay time.Duration
	switch b.Policy {
	case ConstantBackoff:
		delay = b.BaseDelay
	case LinearBackoff:
		delay = b.BaseDelay * time.Duration(attempt+1)
	case ExponentialBackoff:
		// BaseDelay * 2^attempt
		multiplier := math.Pow(2, float64(attempt))
		delay = time.Duration(float64(b.BaseDelay) * multiplier)
	default:
		// Fallback to constant if unknown policy
		delay = b.BaseDelay
	}

	if b.MaxDelay > 0 && delay > b.MaxDelay {
		delay = b.MaxDelay
	}

	if b.Jitter {
		// Apply +/- 20% jitter
		jitterFactor := 0.2
		maxJitter := float64(delay) * jitterFactor
		// rand.Float64() returns [0.0, 1.0)
		// We want [-maxJitter, maxJitter)
		jitter := (rand.Float64() * maxJitter * 2) - maxJitter
		delay = delay + time.Duration(jitter)

		// Jitter could push it over max delay again, but usually we clamp first, then jitter.
		// Alternatively, clamp after jitter. The spec says "+/- 20% jitter if configured, and enforce bounds like MaxDelay".
		// Let's enforce MaxDelay as a hard bound even after jitter, and also ensure delay doesn't go below 0.
		if b.MaxDelay > 0 && delay > b.MaxDelay {
			delay = b.MaxDelay
		}
		if delay < 0 {
			delay = 0
		}
	}

	return delay, true
}

package retry

import (
	"fmt"
	"time"
)

// RetryPolicy handles retry logic with exponential backoff
type RetryPolicy struct {
	maxAttempts  int
	initialDelay time.Duration
	maxDelay     time.Duration
}

// NewRetryPolicy creates a new retry policy
func NewRetryPolicy(maxAttempts int, initialDelay time.Duration) *RetryPolicy {
	return &RetryPolicy{
		maxAttempts:  maxAttempts,
		initialDelay: initialDelay,
		maxDelay:     30 * time.Second, // Cap at 30 seconds
	}
}

// Execute runs a function with retry logic
func (r *RetryPolicy) Execute(fn func() error) error {
	var lastErr error
	delay := r.initialDelay

	for attempt := 1; attempt <= r.maxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't sleep after last attempt
		if attempt < r.maxAttempts {
			time.Sleep(delay)
			// Exponential backoff: double the delay each time
			delay = time.Duration(float64(delay) * 1.5)
			if delay > r.maxDelay {
				delay = r.maxDelay
			}
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", r.maxAttempts, lastErr)
}


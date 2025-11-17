package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBucket implements a token bucket rate limiter using Redis
type TokenBucket struct {
	client       *redis.Client
	key          string
	maxTokens    int           // Maximum tokens in bucket
	refillRate   int           // Tokens added per minute
	refillPeriod time.Duration // How often to refill
}

// NewTokenBucket creates a new token bucket rate limiter
func NewTokenBucket(client *redis.Client, maxTokens int) *TokenBucket {
	return &TokenBucket{
		client:       client,
		key:          "alert:ratelimit:tokens",
		maxTokens:    maxTokens,
		refillRate:   maxTokens,         // Refill to max each minute
		refillPeriod: 1 * time.Minute,
	}
}

// AllowAlert returns true if an alert can be sent (token available)
func (tb *TokenBucket) AllowAlert(ctx context.Context) (bool, error) {
	// Initialize bucket if it doesn't exist
	if err := tb.initialize(ctx); err != nil {
		return false, err
	}

	// Try to consume a token
	tokens, err := tb.client.Decr(ctx, tb.key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to decrement tokens: %w", err)
	}

	// If tokens went negative, we're rate limited
	if tokens < 0 {
		// Restore the token we tried to take
		tb.client.Incr(ctx, tb.key)
		return false, nil
	}

	return true, nil
}

// initialize sets up the token bucket if it doesn't exist
func (tb *TokenBucket) initialize(ctx context.Context) error {
	// Check if key exists
	exists, err := tb.client.Exists(ctx, tb.key).Result()
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if exists == 0 {
		// Initialize with max tokens
		if err := tb.client.Set(ctx, tb.key, tb.maxTokens, 0).Err(); err != nil {
			return fmt.Errorf("failed to initialize bucket: %w", err)
		}

		// Start refill goroutine
		go tb.refillLoop(context.Background())
	}

	return nil
}

// refillLoop periodically refills the token bucket
func (tb *TokenBucket) refillLoop(ctx context.Context) {
	ticker := time.NewTicker(tb.refillPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Refill to max tokens
			if err := tb.client.Set(ctx, tb.key, tb.maxTokens, 0).Err(); err != nil {
				fmt.Printf("error refilling token bucket: %v\n", err)
			}
		}
	}
}

// GetTokens returns the current token count (for monitoring)
func (tb *TokenBucket) GetTokens(ctx context.Context) (int, error) {
	tokens, err := tb.client.Get(ctx, tb.key).Int()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get tokens: %w", err)
	}

	return tokens, nil
}

// Reset resets the bucket to max tokens (for testing)
func (tb *TokenBucket) Reset(ctx context.Context) error {
	return tb.client.Set(ctx, tb.key, tb.maxTokens, 0).Err()
}





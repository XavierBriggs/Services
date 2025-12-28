package dedup

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// BetIntentKeyPrefix is the Redis key prefix for bet intent deduplication
	BetIntentKeyPrefix = "bet_intent:"
	// DefaultIntentTTL is the default TTL for bet intent keys
	DefaultIntentTTL = 600 * time.Second // 10 minutes
)

// Deduplicator handles bet intent deduplication using Redis
type Deduplicator struct {
	redis *redis.Client
	ttl   time.Duration
}

// NewDeduplicator creates a new deduplicator
func NewDeduplicator(redisClient *redis.Client, ttl time.Duration) *Deduplicator {
	if ttl == 0 {
		ttl = DefaultIntentTTL
	}
	return &Deduplicator{
		redis: redisClient,
		ttl:   ttl,
	}
}

// AcquireIntent attempts to acquire a lock for a bet intent
// Returns true if the intent was acquired (first submission), false if it already exists (duplicate)
func (d *Deduplicator) AcquireIntent(ctx context.Context, betIntentID string) (bool, error) {
	key := BetIntentKeyPrefix + betIntentID

	// Use SETNX (SET if Not eXists) with TTL
	// Returns true if key was set, false if it already existed
	result, err := d.redis.SetNX(ctx, key, "1", d.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire intent lock: %w", err)
	}

	return result, nil
}

// ReleaseIntent releases a bet intent lock (for cleanup/testing)
func (d *Deduplicator) ReleaseIntent(ctx context.Context, betIntentID string) error {
	key := BetIntentKeyPrefix + betIntentID
	return d.redis.Del(ctx, key).Err()
}

// CheckIntent checks if a bet intent exists without modifying it
func (d *Deduplicator) CheckIntent(ctx context.Context, betIntentID string) (bool, error) {
	key := BetIntentKeyPrefix + betIntentID
	exists, err := d.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check intent: %w", err)
	}
	return exists > 0, nil
}

// ExtendIntent extends the TTL of an existing intent (useful for long-running operations)
func (d *Deduplicator) ExtendIntent(ctx context.Context, betIntentID string, additionalTTL time.Duration) error {
	key := BetIntentKeyPrefix + betIntentID
	return d.redis.Expire(ctx, key, d.ttl+additionalTTL).Err()
}

// MarkIntentCompleted marks an intent as completed with the result
// This can be used to store the outcome for debugging
func (d *Deduplicator) MarkIntentCompleted(ctx context.Context, betIntentID string, result string) error {
	key := BetIntentKeyPrefix + betIntentID
	// Update the value with result but keep the same TTL
	return d.redis.Set(ctx, key, result, d.ttl).Err()
}












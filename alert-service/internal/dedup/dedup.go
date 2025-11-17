package dedup

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/XavierBriggs/fortuna/services/alert-service/pkg/models"
	"github.com/redis/go-redis/v9"
)

// Deduplicator deduplicates alerts using Redis
type Deduplicator struct {
	client *redis.Client
	ttl    time.Duration
}

// NewDeduplicator creates a new deduplicator
func NewDeduplicator(client *redis.Client, ttlMinutes int) *Deduplicator {
	return &Deduplicator{
		client: client,
		ttl:    time.Duration(ttlMinutes) * time.Minute,
	}
}

// ShouldAlert returns true if this opportunity hasn't been alerted recently
func (d *Deduplicator) ShouldAlert(ctx context.Context, opp models.Opportunity) (bool, error) {
	// Generate dedup key based on opportunity characteristics
	dedupKey := d.generateDedupKey(opp)

	// Check if key exists in Redis
	exists, err := d.client.Exists(ctx, dedupKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check dedup key: %w", err)
	}

	if exists > 0 {
		// Already alerted recently
		return false, nil
	}

	// Set the key with TTL
	if err := d.client.Set(ctx, dedupKey, "1", d.ttl).Err(); err != nil {
		return false, fmt.Errorf("failed to set dedup key: %w", err)
	}

	return true, nil
}

// generateDedupKey creates a unique key for an opportunity
func (d *Deduplicator) generateDedupKey(opp models.Opportunity) string {
	// Create a deterministic hash based on opportunity characteristics
	// Key format: alert:dedup:{event_id}:{market_key}:{books_hash}

	// Sort book keys for deterministic ordering
	bookKeys := make([]string, 0, len(opp.Legs))
	for _, leg := range opp.Legs {
		bookKeys = append(bookKeys, leg.BookKey)
	}
	sort.Strings(bookKeys)

	// Create books hash
	booksStr := strings.Join(bookKeys, ",")
	hash := sha256.Sum256([]byte(booksStr))
	booksHash := fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes of hash

	// Build dedup key
	return fmt.Sprintf("alert:dedup:%s:%s:%s", opp.EventID, opp.MarketKey, booksHash)
}

// Clear removes a dedup entry (for testing)
func (d *Deduplicator) Clear(ctx context.Context, opp models.Opportunity) error {
	dedupKey := d.generateDedupKey(opp)
	return d.client.Del(ctx, dedupKey).Err()
}





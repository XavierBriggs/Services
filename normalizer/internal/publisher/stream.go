package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/models"
	"github.com/redis/go-redis/v9"
)

// StreamPublisher publishes normalized odds to Redis Streams
type StreamPublisher struct {
	redis *redis.Client
}

// NewStreamPublisher creates a new stream publisher
func NewStreamPublisher(redisClient *redis.Client) *StreamPublisher {
	return &StreamPublisher{
		redis: redisClient,
	}
}

// Publish publishes a normalized odds message to the specified stream
// Stream key format: odds.normalized.{sport_key}
func (p *StreamPublisher) Publish(ctx context.Context, normalized *models.NormalizedOdds) error {
	// Build stream key from sport
	streamKey := fmt.Sprintf("odds.normalized.%s", normalized.SportKey)

	// Marshal to JSON
	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("error marshaling normalized odds: %w", err)
	}

	// Publish to stream
	_, err = p.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("error publishing to stream %s: %w", streamKey, err)
	}

	return nil
}

// PublishBatch publishes multiple normalized odds in a single pipeline
func (p *StreamPublisher) PublishBatch(ctx context.Context, normalized []*models.NormalizedOdds) error {
	if len(normalized) == 0 {
		return nil
	}

	// Group by sport key for efficient batching
	bySport := make(map[string][]*models.NormalizedOdds)
	for _, odds := range normalized {
		bySport[odds.SportKey] = append(bySport[odds.SportKey], odds)
	}

	// Use pipeline for batch publish
	pipe := p.redis.Pipeline()

	for sportKey, oddsGroup := range bySport {
		streamKey := fmt.Sprintf("odds.normalized.%s", sportKey)

		for _, odds := range oddsGroup {
			data, err := json.Marshal(odds)
			if err != nil {
				fmt.Printf("error marshaling odds: %v\n", err)
				continue
			}

			pipe.XAdd(ctx, &redis.XAddArgs{
				Stream: streamKey,
				Values: map[string]interface{}{
					"data": string(data),
				},
			})
		}
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("error executing publish pipeline: %w", err)
	}

	return nil
}





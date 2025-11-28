package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/XavierBriggs/fortuna/services/opportunity-analytics/pkg/models"
	"github.com/redis/go-redis/v9"
)

// StreamConsumer consumes opportunities from Redis Stream
type StreamConsumer struct {
	client        *redis.Client
	consumerGroup string
	consumerID    string
}

// NewStreamConsumer creates a new stream consumer
func NewStreamConsumer(redisClient *redis.Client, consumerGroup, consumerID string) *StreamConsumer {
	return &StreamConsumer{
		client:        redisClient,
		consumerGroup: consumerGroup,
		consumerID:    consumerID,
	}
}

// Initialize creates the consumer group if it doesn't exist
func (sc *StreamConsumer) Initialize(ctx context.Context, streamKey string) error {
	return sc.ensureConsumerGroup(ctx, streamKey)
}

// ensureConsumerGroup creates the consumer group if it doesn't exist
// Uses "0" to process all messages from the beginning (for backfill after Redis restart)
func (sc *StreamConsumer) ensureConsumerGroup(ctx context.Context, streamKey string) error {
	// Try to create consumer group - use "0" to read from beginning if group is new
	err := sc.client.XGroupCreateMkStream(ctx, streamKey, sc.consumerGroup, "0").Err()
	if err != nil {
		errStr := err.Error()
		if errStr == "BUSYGROUP Consumer Group name already exists" {
			// Group exists, that's fine
			fmt.Printf("✓ Consumer group already exists: %s (group: %s, consumer: %s)\n",
				streamKey, sc.consumerGroup, sc.consumerID)
			return nil
		}
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	fmt.Printf("✓ Created new consumer group: %s (group: %s, consumer: %s)\n",
		streamKey, sc.consumerGroup, sc.consumerID)

	return nil
}

// ReadMessages reads opportunities from the stream
func (sc *StreamConsumer) ReadMessages(ctx context.Context, streamKey string, count int64, block time.Duration) ([]models.Opportunity, error) {
	streams, err := sc.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    sc.consumerGroup,
		Consumer: sc.consumerID,
		Streams:  []string{streamKey, ">"},
		Count:    count,
		Block:    block,
	}).Result()

	if err == redis.Nil {
		// No messages
		return nil, nil
	}

	if err != nil {
		// Handle NOGROUP error - consumer group was lost (e.g., Redis restart)
		if strings.Contains(err.Error(), "NOGROUP") {
			fmt.Printf("⚠️  Consumer group lost, recreating: %s\n", sc.consumerGroup)
			if recreateErr := sc.ensureConsumerGroup(ctx, streamKey); recreateErr != nil {
				return nil, fmt.Errorf("failed to recreate consumer group: %w", recreateErr)
			}
			// Return nil to retry on next tick
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read messages: %w", err)
	}

	opportunities := make([]models.Opportunity, 0)

	for _, stream := range streams {
		for _, message := range stream.Messages {
			// Parse opportunity from message
			oppJSON, exists := message.Values["opportunity"]
			if !exists {
				continue
			}

			oppStr, ok := oppJSON.(string)
			if !ok {
				continue
			}

			var opp models.Opportunity
			if err := json.Unmarshal([]byte(oppStr), &opp); err != nil {
				fmt.Printf("failed to parse opportunity: %v\n", err)
				continue
			}

			opportunities = append(opportunities, opp)

			// Acknowledge the message
			if err := sc.client.XAck(ctx, streamKey, sc.consumerGroup, message.ID).Err(); err != nil {
				fmt.Printf("failed to ack message %s: %v\n", message.ID, err)
			}
		}
	}

	return opportunities, nil
}

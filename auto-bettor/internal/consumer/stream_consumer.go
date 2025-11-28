package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
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

// ConsumeStream consumes messages from a stream
func (sc *StreamConsumer) ConsumeStream(ctx context.Context, streamKey string, batchSize int, blockTime time.Duration) error {
	// Create consumer group if it doesn't exist
	err := sc.client.XGroupCreateMkStream(ctx, streamKey, sc.consumerGroup, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	fmt.Printf("✓ Started consuming from stream: %s (group: %s, consumer: %s)\n",
		streamKey, sc.consumerGroup, sc.consumerID)

	return nil
}

// ReadMessages reads messages from the stream
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



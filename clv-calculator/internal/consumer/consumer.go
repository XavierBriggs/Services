package consumer

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// EventProcessor processes closing line events
type EventProcessor interface {
	ProcessEvent(ctx context.Context, eventID string) error
}

// Consumer consumes from Redis stream
type Consumer struct {
	redisClient   *redis.Client
	streamName    string
	consumerGroup string
	processor     EventProcessor
}

// NewConsumer creates a new stream consumer
func NewConsumer(redisClient *redis.Client, streamName, consumerGroup string, processor EventProcessor) *Consumer {
	return &Consumer{
		redisClient:   redisClient,
		streamName:    streamName,
		consumerGroup: consumerGroup,
		processor:     processor,
	}
}

// Start starts consuming from the stream
func (c *Consumer) Start(ctx context.Context) error {
	// Create consumer group if it doesn't exist
	err := c.redisClient.XGroupCreateMkStream(ctx, c.streamName, c.consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create consumer group: %w", err)
	}

	fmt.Println("✓ Consumer group ready")

	// Start consuming
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err := c.consumeBatch(ctx); err != nil {
				fmt.Printf("⚠️  Consume error: %v\n", err)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

func (c *Consumer) consumeBatch(ctx context.Context) error {
	// Read from stream
	streams, err := c.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.consumerGroup,
		Consumer: "clv-calculator-1",
		Streams:  []string{c.streamName, ">"},
		Count:    10,
		Block:    5 * time.Second,
	}).Result()

	if err == redis.Nil {
		// No messages, continue
		return nil
	}

	if err != nil {
		return fmt.Errorf("xreadgroup: %w", err)
	}

	// Process messages
	for _, stream := range streams {
		for _, message := range stream.Messages {
			if err := c.processMessage(ctx, message); err != nil {
				fmt.Printf("⚠️  Process message error: %v\n", err)
				continue
			}

			// ACK message
			c.redisClient.XAck(ctx, c.streamName, c.consumerGroup, message.ID)
		}
	}

	return nil
}

func (c *Consumer) processMessage(ctx context.Context, message redis.XMessage) error {
	// Extract event_id from message
	eventID, ok := message.Values["event_id"].(string)
	if !ok {
		return fmt.Errorf("missing event_id in message")
	}

	fmt.Printf("[CLV] Processing event: %s\n", eventID)

	// Process event
	return c.processor.ProcessEvent(ctx, eventID)
}





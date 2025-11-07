package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/models"
	"github.com/redis/go-redis/v9"
)

// StreamConsumer reads raw odds from Redis Streams
type StreamConsumer struct {
	redis       *redis.Client
	consumerID  string
	groupName   string
	batchSize   int64
	blockTime   time.Duration
}

// NewStreamConsumer creates a new stream consumer
func NewStreamConsumer(redisClient *redis.Client, consumerID, groupName string) *StreamConsumer {
	return &StreamConsumer{
		redis:      redisClient,
		consumerID: consumerID,
		groupName:  groupName,
		batchSize:  100,
		blockTime:  5 * time.Second,
	}
}

// ConsumeStream reads messages from a Redis stream
// Returns a channel of RawOdds messages
func (c *StreamConsumer) ConsumeStream(ctx context.Context, streamKey string) (<-chan Message, <-chan error) {
	messageCh := make(chan Message, c.batchSize)
	errorCh := make(chan error, 1)

	go func() {
		defer close(messageCh)
		defer close(errorCh)

		// Create consumer group if it doesn't exist
		err := c.createConsumerGroup(ctx, streamKey)
		if err != nil {
			errorCh <- fmt.Errorf("failed to create consumer group: %w", err)
			return
		}

		// Start consuming from stream
		for {
			select {
			case <-ctx.Done():
				return
			default:
				messages, err := c.readMessages(ctx, streamKey)
				if err != nil {
					errorCh <- fmt.Errorf("error reading messages: %w", err)
					continue
				}

				for _, msg := range messages {
					select {
					case messageCh <- msg:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return messageCh, errorCh
}

// readMessages reads a batch of messages from the stream
func (c *StreamConsumer) readMessages(ctx context.Context, streamKey string) ([]Message, error) {
	// Read from consumer group
	streams, err := c.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.groupName,
		Consumer: c.consumerID,
		Streams:  []string{streamKey, ">"},
		Count:    c.batchSize,
		Block:    c.blockTime,
	}).Result()

	if err != nil {
		if err == redis.Nil {
			// No new messages, not an error
			return nil, nil
		}
		return nil, err
	}

	var messages []Message

	for _, stream := range streams {
		for _, xmsg := range stream.Messages {
			// Parse message data
			data, ok := xmsg.Values["data"].(string)
			if !ok {
				continue
			}

			var rawOdds models.RawOdds
			if err := json.Unmarshal([]byte(data), &rawOdds); err != nil {
				fmt.Printf("error unmarshaling message %s: %v\n", xmsg.ID, err)
				// ACK the message anyway to prevent reprocessing
				c.ackMessage(context.Background(), streamKey, xmsg.ID)
				continue
			}

			messages = append(messages, Message{
				ID:       xmsg.ID,
				RawOdds:  rawOdds,
				StreamKey: streamKey,
			})
		}
	}

	return messages, nil
}

// AckMessage acknowledges a message has been processed
func (c *StreamConsumer) AckMessage(ctx context.Context, streamKey, messageID string) error {
	return c.ackMessage(ctx, streamKey, messageID)
}

// ackMessage internal implementation
func (c *StreamConsumer) ackMessage(ctx context.Context, streamKey, messageID string) error {
	return c.redis.XAck(ctx, streamKey, c.groupName, messageID).Err()
}

// createConsumerGroup creates the consumer group if it doesn't exist
func (c *StreamConsumer) createConsumerGroup(ctx context.Context, streamKey string) error {
	// Try to create group, ignore "BUSYGROUP" error if it already exists
	err := c.redis.XGroupCreateMkStream(ctx, streamKey, c.groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// Message represents a consumed stream message
type Message struct {
	ID        string
	RawOdds   models.RawOdds
	StreamKey string
}


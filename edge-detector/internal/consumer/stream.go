package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/models"
	"github.com/redis/go-redis/v9"
)

// StreamConsumer consumes normalized odds from Redis Streams
type StreamConsumer struct {
	client     *redis.Client
	consumerID string
	groupName  string
}

// Message represents a stream message with normalized odds
type Message struct {
	ID              string
	StreamKey       string
	NormalizedOdds  models.NormalizedOdds
}

// NewStreamConsumer creates a new stream consumer
func NewStreamConsumer(client *redis.Client, consumerID, groupName string) *StreamConsumer {
	return &StreamConsumer{
		client:     client,
		consumerID: consumerID,
		groupName:  groupName,
	}
}

// ensureConsumerGroup creates the consumer group if it doesn't exist
// Uses "0" to process all messages from the beginning (for recovery after Redis restart)
func (c *StreamConsumer) ensureConsumerGroup(ctx context.Context, streamKey string) error {
	err := c.client.XGroupCreateMkStream(ctx, streamKey, c.groupName, "0").Err()
	if err != nil {
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			return nil // Group exists, that's fine
		}
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	fmt.Printf("✓ Created consumer group: %s for stream: %s\n", c.groupName, streamKey)
	return nil
}

// ConsumeStream starts consuming from a stream and returns channels for messages and errors
func (c *StreamConsumer) ConsumeStream(ctx context.Context, streamKey string) (<-chan Message, <-chan error) {
	messageCh := make(chan Message, 100)
	errorCh := make(chan error, 10)

	// Create consumer group if it doesn't exist
	if err := c.ensureConsumerGroup(ctx, streamKey); err != nil {
		errorCh <- err
		close(messageCh)
		close(errorCh)
		return messageCh, errorCh
	}

	go func() {
		defer close(messageCh)
		defer close(errorCh)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Read from stream
				streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
					Group:    c.groupName,
					Consumer: c.consumerID,
					Streams:  []string{streamKey, ">"},
					Count:    10,
					Block:    1 * time.Second,
				}).Result()

				if err != nil {
					if err == redis.Nil {
						// No messages, continue
						continue
					}
					if ctx.Err() != nil {
						// Context cancelled
						return
					}
					// Handle NOGROUP error - consumer group was lost (e.g., Redis restart)
					if strings.Contains(err.Error(), "NOGROUP") {
						fmt.Printf("⚠️  Consumer group lost, recreating: %s\n", c.groupName)
						if recreateErr := c.ensureConsumerGroup(ctx, streamKey); recreateErr != nil {
							errorCh <- fmt.Errorf("failed to recreate consumer group: %w", recreateErr)
						}
						continue
					}
					errorCh <- fmt.Errorf("error reading from stream: %w", err)
					time.Sleep(1 * time.Second)
					continue
				}

				// Process messages
				for _, stream := range streams {
					for _, message := range stream.Messages {
						msg, err := c.parseMessage(streamKey, message)
						if err != nil {
							errorCh <- fmt.Errorf("error parsing message %s: %w", message.ID, err)
							continue
						}

						messageCh <- msg
					}
				}
			}
		}
	}()

	return messageCh, errorCh
}

// parseMessage parses a Redis stream message into a Message
func (c *StreamConsumer) parseMessage(streamKey string, xmsg redis.XMessage) (Message, error) {
	// Get the JSON payload from the message (normalizer publishes with "data" field)
	oddsJSON, ok := xmsg.Values["data"].(string)
	if !ok {
		return Message{}, fmt.Errorf("missing 'data' field in message")
	}

	// Parse normalized odds
	var odds models.NormalizedOdds
	if err := json.Unmarshal([]byte(oddsJSON), &odds); err != nil {
		return Message{}, fmt.Errorf("failed to parse odds JSON: %w", err)
	}

	return Message{
		ID:             xmsg.ID,
		StreamKey:      streamKey,
		NormalizedOdds: odds,
	}, nil
}

// AckMessage acknowledges a message as processed
func (c *StreamConsumer) AckMessage(ctx context.Context, streamKey, messageID string) error {
	return c.client.XAck(ctx, streamKey, c.groupName, messageID).Err()
}


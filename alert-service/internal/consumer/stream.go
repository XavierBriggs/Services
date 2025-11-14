package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/alert-service/pkg/models"
	"github.com/redis/go-redis/v9"
)

// StreamConsumer consumes opportunities from Redis Streams
type StreamConsumer struct {
	client     *redis.Client
	consumerID string
	groupName  string
}

// Message represents a stream message with an opportunity
type Message struct {
	ID          string
	StreamKey   string
	Opportunity models.Opportunity
}

// NewStreamConsumer creates a new stream consumer
func NewStreamConsumer(client *redis.Client, consumerID, groupName string) *StreamConsumer {
	return &StreamConsumer{
		client:     client,
		consumerID: consumerID,
		groupName:  groupName,
	}
}

// ConsumeStream starts consuming from the opportunities stream
func (c *StreamConsumer) ConsumeStream(ctx context.Context, streamKey string) (<-chan Message, <-chan error) {
	messageCh := make(chan Message, 100)
	errorCh := make(chan error, 10)

	// Create consumer group if it doesn't exist
	err := c.client.XGroupCreateMkStream(ctx, streamKey, c.groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		errorCh <- fmt.Errorf("failed to create consumer group: %w", err)
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
						continue
					}
					if ctx.Err() != nil {
						return
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
	// Get the JSON payload
	oppJSON, ok := xmsg.Values["opportunity"].(string)
	if !ok {
		return Message{}, fmt.Errorf("missing 'opportunity' field in message")
	}

	// Parse opportunity
	var opp models.Opportunity
	if err := json.Unmarshal([]byte(oppJSON), &opp); err != nil {
		return Message{}, fmt.Errorf("failed to parse opportunity JSON: %w", err)
	}

	return Message{
		ID:          xmsg.ID,
		StreamKey:   streamKey,
		Opportunity: opp,
	}, nil
}

// AckMessage acknowledges a message as processed
func (c *StreamConsumer) AckMessage(ctx context.Context, streamKey, messageID string) error {
	return c.client.XAck(ctx, streamKey, c.groupName, messageID).Err()
}




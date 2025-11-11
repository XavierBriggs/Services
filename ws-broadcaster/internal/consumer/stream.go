package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/config"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/hub"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/pkg/models"
	"github.com/redis/go-redis/v9"
)

const (
	// Batch size for reading messages
	batchSize = 100

	// Block duration when waiting for new messages
	blockDuration = 1 * time.Second
)

// StreamConsumer consumes odds updates from Redis Streams
type StreamConsumer struct {
	redis        *redis.Client
	hub          *hub.Hub
	streamConfig config.StreamConfig
}

// NewStreamConsumer creates a new stream consumer
func NewStreamConsumer(redisClient *redis.Client, h *hub.Hub, streamConfig config.StreamConfig) *StreamConsumer {
	return &StreamConsumer{
		redis:        redisClient,
		hub:          h,
		streamConfig: streamConfig,
	}
}

// Start begins consuming from Redis Streams
func (sc *StreamConsumer) Start(ctx context.Context) error {
	fmt.Println("✓ Stream consumer started")

	// Get all configured streams
	streams := sc.streamConfig.GetAllStreams()
	
	fmt.Printf("  📡 Configured streams: %v\n", streams)

	// Create consumer groups for all streams (ignore errors if they already exist)
	for _, stream := range streams {
		sc.createConsumerGroup(ctx, stream)
	}

	// Start consuming from all streams concurrently
	for _, stream := range streams {
		streamName := stream // Capture for goroutine
		go sc.consumeStream(ctx, streamName)
	}

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

// createConsumerGroup creates a consumer group for a stream
func (sc *StreamConsumer) createConsumerGroup(ctx context.Context, stream string) {
	err := sc.redis.XGroupCreateMkStream(ctx, stream, sc.streamConfig.ConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		fmt.Printf("⚠️  Failed to create consumer group for %s: %v\n", stream, err)
	}
}

// consumeStream consumes messages from a specific stream
func (sc *StreamConsumer) consumeStream(ctx context.Context, stream string) {
	fmt.Printf("  📡 Consuming stream: %s\n", stream)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Read messages from stream
			streams, err := sc.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    sc.streamConfig.ConsumerGroup,
				Consumer: sc.streamConfig.ConsumerID,
				Streams:  []string{stream, ">"},
				Count:    batchSize,
				Block:    blockDuration,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					// No new messages - continue
					continue
				}
				fmt.Printf("⚠️  Stream read error (%s): %v\n", stream, err)
				time.Sleep(1 * time.Second)
				continue
			}

			// Process messages
			for _, stream := range streams {
				for _, message := range stream.Messages {
					sc.processMessage(ctx, stream.Stream, message)
				}
			}
		}
	}
}

// processMessage processes a single stream message
func (sc *StreamConsumer) processMessage(ctx context.Context, stream string, msg redis.XMessage) {
	// Parse the message data
	dataStr, ok := msg.Values["data"].(string)
	if !ok {
		fmt.Printf("⚠️  Invalid message format in %s: %v\n", stream, msg.Values)
		sc.ackMessage(ctx, stream, msg.ID)
		return
	}

	// Route based on stream type
	if strings.HasPrefix(stream, "games.updates.") {
		sc.processGameUpdate(ctx, stream, dataStr, msg.ID)
	} else if strings.HasPrefix(stream, "odds.normalized.") {
		sc.processOddsUpdate(ctx, stream, dataStr, msg.ID)
	} else {
		// Handle other stream types (opportunities, etc.)
		var oddsUpdate models.OddsUpdate
		if err := json.Unmarshal([]byte(dataStr), &oddsUpdate); err != nil {
			fmt.Printf("⚠️  Failed to parse update from %s: %v\n", stream, err)
			sc.ackMessage(ctx, stream, msg.ID)
			return
		}

		sc.hub.Broadcast(oddsUpdate)
		sc.ackMessage(ctx, stream, msg.ID)
	}
}

// processOddsUpdate processes odds update messages
func (sc *StreamConsumer) processOddsUpdate(ctx context.Context, stream string, dataStr string, messageID string) {
	var oddsUpdate models.OddsUpdate
	if err := json.Unmarshal([]byte(dataStr), &oddsUpdate); err != nil {
		fmt.Printf("⚠️  Failed to parse odds update from %s: %v\n", stream, err)
		sc.ackMessage(ctx, stream, messageID)
		return
	}

	fmt.Printf("📤 Broadcasting odds: sport=%s market=%s book=%s outcome=%s\n", 
		oddsUpdate.SportKey, oddsUpdate.MarketKey, oddsUpdate.BookKey, oddsUpdate.OutcomeName)

	// Broadcast to connected clients
	sc.hub.Broadcast(oddsUpdate)

	// Acknowledge message
	sc.ackMessage(ctx, stream, messageID)
}

// processGameUpdate processes game update messages
func (sc *StreamConsumer) processGameUpdate(ctx context.Context, stream string, dataStr string, messageID string) {
	// Parse as generic map for game updates
	var gameUpdate map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &gameUpdate); err != nil {
		fmt.Printf("⚠️  Failed to parse game update from %s: %v\n", stream, err)
		sc.ackMessage(ctx, stream, messageID)
		return
	}

	// Add type field for frontend to distinguish message types
	gameUpdate["message_type"] = "game_update"

	fmt.Printf("📤 Broadcasting game: game_id=%s status=%s\n", 
		gameUpdate["game_id"], gameUpdate["status"])

	// Broadcast to connected clients
	sc.hub.Broadcast(gameUpdate)

	// Acknowledge message
	sc.ackMessage(ctx, stream, messageID)
}

// ackMessage acknowledges a message in the stream
func (sc *StreamConsumer) ackMessage(ctx context.Context, stream string, messageID string) {
	err := sc.redis.XAck(ctx, stream, sc.streamConfig.ConsumerGroup, messageID).Err()
	if err != nil {
		fmt.Printf("⚠️  Failed to ack message %s in %s: %v\n", messageID, stream, err)
	}
}


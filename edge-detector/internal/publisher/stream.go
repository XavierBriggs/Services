package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/models"
	"github.com/redis/go-redis/v9"
)

// StreamPublisher publishes opportunities to Redis Streams
type StreamPublisher struct {
	client *redis.Client
}

// NewStreamPublisher creates a new stream publisher
func NewStreamPublisher(client *redis.Client) *StreamPublisher {
	return &StreamPublisher{
		client: client,
	}
}

// PublishOpportunity publishes a single opportunity to the opportunities.detected stream
func (p *StreamPublisher) PublishOpportunity(ctx context.Context, opportunity models.Opportunity) error {
	// Convert opportunity to JSON
	opportunityJSON, err := json.Marshal(opportunity)
	if err != nil {
		return fmt.Errorf("failed to marshal opportunity: %w", err)
	}

	// Build stream key based on sport
	streamKey := fmt.Sprintf("opportunities.detected.%s", opportunity.SportKey)

	// Publish to stream
	_, err = p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"opportunity": string(opportunityJSON),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish to stream %s: %w", streamKey, err)
	}

	return nil
}

// PublishOpportunities publishes multiple opportunities
func (p *StreamPublisher) PublishOpportunities(ctx context.Context, opportunities []models.Opportunity) error {
	if len(opportunities) == 0 {
		return nil
	}

	for _, opp := range opportunities {
		if err := p.PublishOpportunity(ctx, opp); err != nil {
			return err
		}
	}

	return nil
}

// PublishToGlobalStream also publishes to a global opportunities.detected stream
// (without sport-specific suffix) for services that want all opportunities
func (p *StreamPublisher) PublishToGlobalStream(ctx context.Context, opportunity models.Opportunity) error {
	// Convert opportunity to JSON
	opportunityJSON, err := json.Marshal(opportunity)
	if err != nil {
		return fmt.Errorf("failed to marshal opportunity: %w", err)
	}

	// Publish to global stream
	_, err = p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: "opportunities.detected",
		Values: map[string]interface{}{
			"opportunity": string(opportunityJSON),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to publish to global stream: %w", err)
	}

	return nil
}

// Publish is the main publish method that publishes to both sport-specific and global streams
func (p *StreamPublisher) Publish(ctx context.Context, opportunity models.Opportunity) error {
	// Publish to sport-specific stream
	if err := p.PublishOpportunity(ctx, opportunity); err != nil {
		return err
	}

	// Also publish to global stream
	if err := p.PublishToGlobalStream(ctx, opportunity); err != nil {
		return err
	}

	return nil
}





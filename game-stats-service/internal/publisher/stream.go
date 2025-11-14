package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fortuna/services/game-stats-service/pkg/models"
	"github.com/redis/go-redis/v9"
)

// StreamPublisher publishes game updates to Redis streams
type StreamPublisher struct {
	client *redis.Client
}

// NewStreamPublisher creates a new stream publisher
func NewStreamPublisher(client *redis.Client) *StreamPublisher {
	return &StreamPublisher{
		client: client,
	}
}

// PublishGameUpdate publishes a game update to the sport-specific stream
func (p *StreamPublisher) PublishGameUpdate(ctx context.Context, game *models.Game) error {
	streamKey := fmt.Sprintf("games.updates.%s", game.SportKey)
	
	data, err := json.Marshal(game)
	if err != nil {
		return fmt.Errorf("marshaling game update: %w", err)
	}

	// Publish to Redis stream
	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"data": string(data),
			"game_id": game.GameID,
			"status": string(game.Status),
		},
	}).Err()
}

// PublishBoxScoreUpdate publishes a box score update
func (p *StreamPublisher) PublishBoxScoreUpdate(ctx context.Context, boxscore *models.BoxScore) error {
	streamKey := fmt.Sprintf("games.updates.%s", boxscore.Game.SportKey)
	
	data, err := json.Marshal(boxscore)
	if err != nil {
		return fmt.Errorf("marshaling boxscore update: %w", err)
	}

	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"data": string(data),
			"game_id": boxscore.Game.GameID,
			"type": "boxscore",
		},
	}).Err()
}




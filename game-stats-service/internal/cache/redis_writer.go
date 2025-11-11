package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fortuna/services/game-stats-service/pkg/models"
	"github.com/redis/go-redis/v9"
)

// TTL constants
const (
	TodaysGamesListTTL = 24 * time.Hour
	LiveGameTTL        = 2 * time.Hour
	FinalGameTTL       = 6 * time.Hour
	BoxScoreTTL        = 6 * time.Hour
)

// RedisWriter handles writing game data to Redis
type RedisWriter struct {
	client *redis.Client
}

// NewRedisWriter creates a new Redis writer
func NewRedisWriter(client *redis.Client) *RedisWriter {
	return &RedisWriter{
		client: client,
	}
}

// WriteTodaysGames stores the list of today's game IDs for a sport
func (w *RedisWriter) WriteTodaysGames(ctx context.Context, sportKey string, date time.Time, gameIDs []string) error {
	key := fmt.Sprintf("games:today:%s:%s", sportKey, date.Format("2006-01-02"))
	
	// Convert to interface slice
	values := make([]interface{}, len(gameIDs))
	for i, id := range gameIDs {
		values[i] = id
	}

	pipe := w.client.Pipeline()
	pipe.Del(ctx, key) // Clear old list
	if len(values) > 0 {
		pipe.RPush(ctx, key, values...)
	}
	pipe.Expire(ctx, key, TodaysGamesListTTL)
	
	_, err := pipe.Exec(ctx)
	return err
}

// WriteGameSummary stores game summary data
func (w *RedisWriter) WriteGameSummary(ctx context.Context, game *models.Game) error {
	key := fmt.Sprintf("game:%s:summary", game.GameID)
	
	data, err := json.Marshal(game)
	if err != nil {
		return fmt.Errorf("marshaling game: %w", err)
	}

	ttl := w.getTTLForGame(game)
	return w.client.Set(ctx, key, data, ttl).Err()
}

// WriteBoxScore stores full box score data
func (w *RedisWriter) WriteBoxScore(ctx context.Context, gameID string, boxscore *models.BoxScore) error {
	key := fmt.Sprintf("game:%s:boxscore", gameID)
	
	data, err := json.Marshal(boxscore)
	if err != nil {
		return fmt.Errorf("marshaling boxscore: %w", err)
	}

	return w.client.Set(ctx, key, data, BoxScoreTTL).Err()
}

// WriteGameStatus stores game status separately for quick access
func (w *RedisWriter) WriteGameStatus(ctx context.Context, gameID string, status models.GameStatus) error {
	key := fmt.Sprintf("game:%s:status", gameID)
	
	ttl := FinalGameTTL
	if status == models.StatusLive {
		ttl = LiveGameTTL
	}
	
	return w.client.Set(ctx, key, string(status), ttl).Err()
}

// getTTLForGame returns appropriate TTL based on game status
func (w *RedisWriter) getTTLForGame(game *models.Game) time.Duration {
	switch game.Status {
	case models.StatusLive:
		return LiveGameTTL
	case models.StatusFinal:
		return FinalGameTTL
	default:
		return LiveGameTTL
	}
}

// ReadGameSummary retrieves game summary from Redis
func (w *RedisWriter) ReadGameSummary(ctx context.Context, gameID string) (*models.Game, error) {
	key := fmt.Sprintf("game:%s:summary", gameID)
	
	data, err := w.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var game models.Game
	if err := json.Unmarshal([]byte(data), &game); err != nil {
		return nil, fmt.Errorf("unmarshaling game: %w", err)
	}

	return &game, nil
}

// ReadTodaysGames retrieves list of today's game IDs
func (w *RedisWriter) ReadTodaysGames(ctx context.Context, sportKey string, date time.Time) ([]string, error) {
	key := fmt.Sprintf("games:today:%s:%s", sportKey, date.Format("2006-01-02"))
	
	return w.client.LRange(ctx, key, 0, -1).Result()
}



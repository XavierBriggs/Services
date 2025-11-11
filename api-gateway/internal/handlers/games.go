package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// GamesHandler handles games-related API endpoints
type GamesHandler struct {
	redisClient *redis.Client
}

// NewGamesHandler creates a new games handler
func NewGamesHandler(redisClient *redis.Client) *GamesHandler {
	return &GamesHandler{
		redisClient: redisClient,
	}
}

// HandleGetTodaysGames returns today's games for a sport
// GET /api/v1/games/today?sport={sport_key}
func (h *GamesHandler) HandleGetTodaysGames(w http.ResponseWriter, r *http.Request) {
	sportKey := r.URL.Query().Get("sport")
	if sportKey == "" {
		sportKey = "basketball_nba" // Default to NBA
	}

	ctx := r.Context()
	
	// Try today's date (UTC - matches what game-stats-service uses)
	today := time.Now().UTC()
	dateStr := today.Format("2006-01-02") // YYYY-MM-DD format
	key := fmt.Sprintf("games:today:%s:%s", sportKey, dateStr)

	// Get game IDs from Redis list
	gameIDs, err := h.redisClient.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching games: %v", err), http.StatusInternalServerError)
		return
	}
	
	// If no games for today, also try tomorrow (for late night users)
	if len(gameIDs) == 0 {
		tomorrow := today.Add(24 * time.Hour)
		tomorrowStr := tomorrow.Format("2006-01-02")
		keyTomorrow := fmt.Sprintf("games:today:%s:%s", sportKey, tomorrowStr)
		gameIDs, _ = h.redisClient.LRange(ctx, keyTomorrow, 0, -1).Result()
	}

	// Fetch each game summary
	var games []interface{}
	for _, gameID := range gameIDs {
		game, err := h.getGameSummary(ctx, gameID)
		if err != nil {
			// Skip games that can't be loaded
			continue
		}
		games = append(games, game)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sport":  sportKey,
		"date":   dateStr,
		"games":  games,
		"count":  len(games),
	})
}

// HandleGetGame returns a single game summary
// GET /api/v1/games/{game_id}
func (h *GamesHandler) HandleGetGame(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "game_id")
	if gameID == "" {
		http.Error(w, "game_id is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	game, err := h.getGameSummary(ctx, gameID)
	if err != nil {
		if err == redis.Nil {
			http.Error(w, "Game not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Error fetching game: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

// HandleGetBoxScore returns detailed box score for a game
// GET /api/v1/games/{game_id}/boxscore
func (h *GamesHandler) HandleGetBoxScore(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "game_id")
	if gameID == "" {
		http.Error(w, "game_id is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	key := fmt.Sprintf("game:%s:boxscore", gameID)

	data, err := h.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			http.Error(w, "Box score not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Error fetching box score: %v", err), http.StatusInternalServerError)
		return
	}

	var boxscore map[string]interface{}
	if err := json.Unmarshal([]byte(data), &boxscore); err != nil {
		http.Error(w, fmt.Sprintf("Error parsing box score: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(boxscore)
}

// HandleGetLinkedOdds returns odds data linked to this game
// GET /api/v1/games/{game_id}/linked-odds
func (h *GamesHandler) HandleGetLinkedOdds(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "game_id")
	if gameID == "" {
		http.Error(w, "game_id is required", http.StatusBadRequest)
		return
	}

	// TODO: Implement game-to-odds mapping
	// For now, return placeholder
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"game_id": gameID,
		"has_odds": false,
		"message": "Odds mapping not yet implemented",
	})
}

// HandleGetEnabledSports returns list of enabled sports for games
// GET /api/v1/sports/enabled
func (h *GamesHandler) HandleGetEnabledSports(w http.ResponseWriter, r *http.Request) {
	// Hardcoded for now, could query game-stats-service in future
	sports := []map[string]interface{}{
		{
			"sport_key":    "basketball_nba",
			"display_name": "NBA",
			"enabled":      true,
			"icon":         "🏀",
		},
		{
			"sport_key":    "american_football_nfl",
			"display_name": "NFL",
			"enabled":      false,
			"icon":         "🏈",
			"coming_soon":  true,
		},
		{
			"sport_key":    "baseball_mlb",
			"display_name": "MLB",
			"enabled":      false,
			"icon":         "⚾",
			"coming_soon":  true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sports": sports,
	})
}

// getGameSummary fetches game summary from Redis
func (h *GamesHandler) getGameSummary(ctx context.Context, gameID string) (map[string]interface{}, error) {
	key := fmt.Sprintf("game:%s:summary", gameID)

	data, err := h.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var game map[string]interface{}
	if err := json.Unmarshal([]byte(data), &game); err != nil {
		return nil, fmt.Errorf("parsing game: %w", err)
	}

	return game, nil
}


package poller

import (
	"context"
	"log"
	"time"

	"github.com/fortuna/services/game-stats-service/internal/cache"
	"github.com/fortuna/services/game-stats-service/internal/providers/espn"
	"github.com/fortuna/services/game-stats-service/internal/publisher"
	"github.com/fortuna/services/game-stats-service/pkg/contracts"
	"github.com/fortuna/services/game-stats-service/pkg/models"
)

// SportPoller polls ESPN API for a specific sport
type SportPoller struct {
	module     contracts.SportModule
	espnClient *espn.Client
	cache      *cache.RedisWriter
	publisher  *publisher.StreamPublisher
}

// NewSportPoller creates a new poller for a sport
func NewSportPoller(
	module contracts.SportModule,
	espnClient *espn.Client,
	cache *cache.RedisWriter,
	publisher *publisher.StreamPublisher,
) *SportPoller {
	return &SportPoller{
		module:     module,
		espnClient: espnClient,
		cache:      cache,
		publisher:  publisher,
	}
}

// Run starts the polling loop for this sport
func (p *SportPoller) Run(ctx context.Context) {
	log.Printf("[%s] Starting poller", p.module.GetSportKey())

	ticker := time.NewTicker(1 * time.Minute) // Initial check every minute
	defer ticker.Stop()

	// Do initial poll
	p.pollOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Stopping poller", p.module.GetSportKey())
			return
		case <-ticker.C:
			p.pollOnce(ctx)
		}
	}
}

// pollOnce performs one polling cycle
func (p *SportPoller) pollOnce(ctx context.Context) {
	sportKey := p.module.GetSportKey()
	sportPath := p.module.GetESPNSportPath()

	// Fetch today's games (pass zero time to get ESPN's default "today")
	log.Printf("[%s] Fetching scoreboard", sportKey)
	scoreboard, err := p.espnClient.FetchScoreboard(ctx, sportPath, time.Time{})
	if err != nil {
		log.Printf("[%s] Error fetching scoreboard: %v", sportKey, err)
		return
	}

	// Extract events
	events, ok := scoreboard["events"].([]interface{})
	if !ok {
		log.Printf("[%s] No events found in scoreboard", sportKey)
		return
	}

	log.Printf("[%s] Found %d games", sportKey, len(events))

	var gameIDs []string

	for _, eventInterface := range events {
		event, ok := eventInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Parse game summary
		game, err := p.module.ParseGameSummary(event)
		if err != nil {
			log.Printf("[%s] Error parsing game: %v", sportKey, err)
			continue
		}

		// Validate
		if err := p.module.ValidateGame(game); err != nil {
			log.Printf("[%s] Invalid game %s: %v", sportKey, game.GameID, err)
			continue
		}

		gameIDs = append(gameIDs, game.GameID)

		// Write to cache
		if err := p.cache.WriteGameSummary(ctx, game); err != nil {
			log.Printf("[%s] Error caching game %s: %v", sportKey, game.GameID, err)
		}

		// Write status
		if err := p.cache.WriteGameStatus(ctx, game.GameID, game.Status); err != nil {
			log.Printf("[%s] Error caching status %s: %v", sportKey, game.GameID, err)
		}

		// Publish update
		if err := p.publisher.PublishGameUpdate(ctx, game); err != nil {
			log.Printf("[%s] Error publishing game %s: %v", sportKey, game.GameID, err)
		}

		// Fetch box score for live/final games
		if game.Status == models.StatusLive || game.Status == models.StatusFinal {
			p.fetchAndCacheBoxScore(ctx, game)
		}

		log.Printf("[%s] Processed game: %s vs %s (%s)", 
			sportKey, game.AwayTeamAbbr, game.HomeTeamAbbr, game.Status)
	}

	// Update today's games list
	if err := p.cache.WriteTodaysGames(ctx, sportKey, time.Now(), gameIDs); err != nil {
		log.Printf("[%s] Error caching today's games list: %v", sportKey, err)
	}
}

// fetchAndCacheBoxScore fetches and caches detailed box score
func (p *SportPoller) fetchAndCacheBoxScore(ctx context.Context, game *models.Game) {
	sportPath := p.module.GetESPNSportPath()
	
	summary, err := p.espnClient.FetchGameSummary(ctx, sportPath, game.GameID)
	if err != nil {
		log.Printf("[%s] Error fetching game summary %s: %v", p.module.GetSportKey(), game.GameID, err)
		return
	}

	// Parse box score
	boxscore, err := p.module.ParseBoxScore(summary)
	if err != nil {
		log.Printf("[%s] Error parsing box score %s: %v", p.module.GetSportKey(), game.GameID, err)
		return
	}

	// Parse player stats
	playerStats, err := p.module.ParsePlayerStats(summary)
	if err != nil {
		log.Printf("[%s] Error parsing player stats %s: %v", p.module.GetSportKey(), game.GameID, err)
	} else {
		// Split players by team
		for _, stat := range playerStats {
			if stat.TeamAbbr == game.HomeTeamAbbr {
				boxscore.HomePlayers = append(boxscore.HomePlayers, stat)
			} else {
				boxscore.AwayPlayers = append(boxscore.AwayPlayers, stat)
			}
		}
	}

	// Cache box score
	if err := p.cache.WriteBoxScore(ctx, game.GameID, boxscore); err != nil {
		log.Printf("[%s] Error caching box score %s: %v", p.module.GetSportKey(), game.GameID, err)
	}

	// Publish box score update
	if err := p.publisher.PublishBoxScoreUpdate(ctx, boxscore); err != nil {
		log.Printf("[%s] Error publishing box score %s: %v", p.module.GetSportKey(), game.GameID, err)
	}
}

// determinePollInterval calculates next poll interval based on game status
func (p *SportPoller) determinePollInterval(game *models.Game) time.Duration {
	config := p.module.GetPollingConfig()

	switch game.Status {
	case models.StatusLive:
		return config.LiveInterval // 30s for live games
	case models.StatusUpcoming:
		// Ramp up as game approaches
		untilStart := time.Until(game.CommenceTime)
		if untilStart < config.PreGameRampup {
			return 1 * time.Minute // Fast polling 30min before
		}
		return config.UpcomingInterval // 5min for far-future
	case models.StatusFinal:
		return 0 // Stop polling completed games
	default:
		return config.UpcomingInterval
	}
}


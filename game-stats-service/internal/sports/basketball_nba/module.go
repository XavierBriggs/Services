package basketball_nba

import (
	"fmt"
	"time"

	"github.com/fortuna/services/game-stats-service/pkg/contracts"
	"github.com/fortuna/services/game-stats-service/pkg/models"
)

// NBAModule implements SportModule for NBA basketball
type NBAModule struct {
	enabled bool
}

// New creates a new NBA sport module
func New() *NBAModule {
	return &NBAModule{enabled: true}
}

func (m *NBAModule) GetSportKey() string {
	return "basketball_nba"
}

func (m *NBAModule) GetDisplayName() string {
	return "NBA"
}

func (m *NBAModule) GetESPNSportPath() string {
	return "basketball/nba"
}

func (m *NBAModule) GetPollingConfig() contracts.PollingConfig {
	return contracts.PollingConfig{
		LiveInterval:     30 * time.Second,
		UpcomingInterval: 5 * time.Minute,
		PreGameRampup:    30 * time.Minute,
		Enabled:          m.enabled,
	}
}

func (m *NBAModule) IsEnabled() bool {
	return m.enabled
}

// ParseGameSummary parses ESPN scoreboard event data into Game model
func (m *NBAModule) ParseGameSummary(rawData map[string]interface{}) (*models.Game, error) {
	game := &models.Game{
		GameID:     extractString(rawData, "id"),
		SportKey:   m.GetSportKey(),
		UpdatedAt:  time.Now(),
	}

	// Parse commence time
	if dateStr := extractString(rawData, "date"); dateStr != "" {
		game.CommenceTime = parseCommenceTime(dateStr)
	}

	// Parse status
	status := extractMap(rawData, "status")
	statusType := extractMap(status, "type")
	game.Status = parseGameStatus(statusType)

	// Get period and time remaining
	game.Period = extractInt(status, "period")
	game.PeriodLabel = getPeriodLabel(game.Period)
	game.TimeRemaining = extractString(status, "displayClock")

	// Parse competitions (teams and scores)
	competitions := extractArray(rawData, "competitions")
	if len(competitions) == 0 {
		return nil, fmt.Errorf("no competitions found in event")
	}

	comp := competitions[0].(map[string]interface{})
	competitors := extractArray(comp, "competitors")
	
	if len(competitors) < 2 {
		return nil, fmt.Errorf("insufficient competitors")
	}

	// Extract home and away teams
	for _, compInterface := range competitors {
		competitor := compInterface.(map[string]interface{})
		homeAway := extractString(competitor, "homeAway")
		team := extractMap(competitor, "team")
		
		teamName := extractString(team, "displayName")
		teamAbbr := extractString(team, "abbreviation")
		score := extractInt(competitor, "score")

		if homeAway == "home" {
			game.HomeTeam = teamName
			game.HomeTeamAbbr = teamAbbr
			game.HomeScore = score
		} else if homeAway == "away" {
			game.AwayTeam = teamName
			game.AwayTeamAbbr = teamAbbr
			game.AwayScore = score
		}
	}

	return game, nil
}

// ParseBoxScore parses ESPN summary data into BoxScore model
func (m *NBAModule) ParseBoxScore(rawData map[string]interface{}) (*models.BoxScore, error) {
	boxscore := extractMap(rawData, "boxscore")
	if len(boxscore) == 0 {
		return nil, fmt.Errorf("no boxscore data found")
	}

	// Get header info (same as game summary)
	header := extractMap(rawData, "header")
	if len(header) == 0 {
		// Try to use competitions from rawData
		header = rawData
	}

	game, err := m.ParseGameSummary(header)
	if err != nil {
		return nil, fmt.Errorf("parsing game summary: %w", err)
	}

	box := &models.BoxScore{
		Game:         game,
		HomeStats:    make(map[string]interface{}),
		AwayStats:    make(map[string]interface{}),
		HomePlayers:  []models.PlayerStat{},
		AwayPlayers:  []models.PlayerStat{},
		PeriodScores: []models.PeriodScore{},
	}

	// Parse period scores
	// TODO: Extract from linescore if available

	return box, nil
}

// ParsePlayerStats parses ESPN player statistics
func (m *NBAModule) ParsePlayerStats(rawData map[string]interface{}) ([]models.PlayerStat, error) {
	boxscore := extractMap(rawData, "boxscore")
	playersData := extractArray(boxscore, "players")
	
	if len(playersData) == 0 {
		return []models.PlayerStat{}, nil
	}

	var allStats []models.PlayerStat

	for _, teamDataInterface := range playersData {
		teamData := teamDataInterface.(map[string]interface{})
		team := extractMap(teamData, "team")
		teamAbbr := extractString(team, "abbreviation")

		statistics := extractArray(teamData, "statistics")
		if len(statistics) == 0 {
			continue
		}

		// First group has player stats
		statGroup := statistics[0].(map[string]interface{})
		athletes := extractArray(statGroup, "athletes")

		for _, athleteInterface := range athletes {
			athleteData := athleteInterface.(map[string]interface{})
			athlete := extractMap(athleteData, "athlete")

			playerName := extractString(athlete, "displayName")
			position := extractString(athlete, "position")

			// Check if player played
			if didNotPlay, ok := athleteData["didNotPlay"].(bool); ok && didNotPlay {
				continue
			}

			// Parse stats array
			stats := extractArray(athleteData, "stats")
			if len(stats) < 17 {
				continue // Not enough stats
			}

			// Parse NBA stats
			nbaStats := NBAPlayerStats{
				Minutes:       parseMinutes(fmt.Sprint(stats[idxMinutes])),
				Points:        parseInt(stats[idxPoints]),
				OffRebounds:   parseInt(stats[idxOffReb]),
				DefRebounds:   parseInt(stats[idxDefReb]),
				Rebounds:      parseInt(stats[idxReb]),
				Assists:       parseInt(stats[idxAst]),
				Steals:        parseInt(stats[idxStl]),
				Blocks:        parseInt(stats[idxBlk]),
				Turnovers:     parseInt(stats[idxTO]),
				FieldGoals:    fmt.Sprint(stats[idxFG]),
				ThreePointers: fmt.Sprint(stats[idx3PT]),
				FreeThrows:    fmt.Sprint(stats[idxFT]),
				PersonalFouls: parseInt(stats[idxPF]),
				PlusMinus:     parsePlusMinus(fmt.Sprint(stats[idxPlusMinus])),
			}

			playerStat := models.PlayerStat{
				PlayerName:   playerName,
				TeamAbbr:     teamAbbr,
				Position:     position,
				Stats:        nbaStats.ToJSON(),
				DisplayStats: nbaStats.ToDisplayStats(),
			}

			allStats = append(allStats, playerStat)
		}
	}

	return allStats, nil
}

// ValidateGame validates NBA-specific game data
func (m *NBAModule) ValidateGame(game *models.Game) error {
	if game.Period < 0 || game.Period > 10 { // Allow up to 6 overtimes
		return fmt.Errorf("invalid NBA period: %d", game.Period)
	}

	if game.HomeTeamAbbr == "" || game.AwayTeamAbbr == "" {
		return fmt.Errorf("missing team abbreviations")
	}

	return nil
}

// NormalizeTeamName converts ESPN team name to standard format
func (m *NBAModule) NormalizeTeamName(espnName string) string {
	// ESPN names are already standard, but could add mappings here
	return espnName
}

// GetTeamAbbreviation returns team abbreviation for full name
func (m *NBAModule) GetTeamAbbreviation(fullName string) string {
	return GetTeamAbbreviation(fullName)
}


package models

import "time"

// GameStatus represents the current state of a game
type GameStatus string

const (
	StatusUpcoming  GameStatus = "upcoming"
	StatusLive      GameStatus = "live"
	StatusFinal     GameStatus = "final"
	StatusPostponed GameStatus = "postponed"
)

// Game is the universal model for any sport
type Game struct {
	GameID        string                 `json:"game_id"`
	SportKey      string                 `json:"sport_key"`        // "basketball_nba"
	Status        GameStatus             `json:"status"`           // "live", "upcoming", "final"
	HomeTeam      string                 `json:"home_team"`        // Full team name
	HomeTeamAbbr  string                 `json:"home_team_abbr"`   // "LAL"
	AwayTeam      string                 `json:"away_team"`        // Full team name
	AwayTeamAbbr  string                 `json:"away_team_abbr"`   // "BOS"
	HomeScore     int                    `json:"home_score"`
	AwayScore     int                    `json:"away_score"`
	Period        int                    `json:"period"`           // Quarter/Period/Inning
	PeriodLabel   string                 `json:"period_label"`     // "Q4", "3rd", "Top 9th"
	TimeRemaining string                 `json:"time_remaining,omitempty"`
	CommenceTime  time.Time              `json:"commence_time"`
	UpdatedAt     time.Time              `json:"updated_at"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"` // Sport-specific extras
}





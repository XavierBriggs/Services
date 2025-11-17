package models

// BoxScore contains full game statistics
type BoxScore struct {
	Game         *Game                  `json:"game"`
	HomeStats    map[string]interface{} `json:"home_stats"`    // Sport-specific team stats
	AwayStats    map[string]interface{} `json:"away_stats"`
	HomePlayers  []PlayerStat           `json:"home_players"`
	AwayPlayers  []PlayerStat           `json:"away_players"`
	PeriodScores []PeriodScore          `json:"period_scores"` // Quarter-by-quarter
}

// PeriodScore represents scoring by period (quarter, inning, etc.)
type PeriodScore struct {
	Period    int    `json:"period"`
	Label     string `json:"label"`      // "Q1", "1st", etc.
	HomeScore int    `json:"home_score"`
	AwayScore int    `json:"away_score"`
}





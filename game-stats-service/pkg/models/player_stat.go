package models

// PlayerStat represents a player's statistics in a game
// Uses hybrid model: raw stats map + display stats for UI
type PlayerStat struct {
	PlayerName   string                 `json:"player_name"`
	TeamAbbr     string                 `json:"team_abbr"`
	Position     string                 `json:"position,omitempty"`
	Stats        map[string]interface{} `json:"stats"`          // Sport-specific raw stats
	DisplayStats []DisplayStat          `json:"display_stats"`  // Formatted for UI
}

// DisplayStat provides formatted stat display info
// Frontend uses this to render stats without knowing sport semantics
type DisplayStat struct {
	Label    string `json:"label"`    // "PTS", "Passing Yards", "HR"
	Value    string `json:"value"`    // "28", "312", "2"
	Category string `json:"category"` // "Scoring", "Passing", "Hitting"
}





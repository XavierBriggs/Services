package basketball_nba

import (
	"fmt"

	"github.com/fortuna/services/game-stats-service/pkg/models"
)

// NBAPlayerStats represents NBA-specific player statistics
type NBAPlayerStats struct {
	Minutes       float64
	Points        int
	Rebounds      int
	Assists       int
	Steals        int
	Blocks        int
	Turnovers     int
	FieldGoals    string // "10-18"
	ThreePointers string // "2-5"
	FreeThrows    string // "6-8"
	PlusMinus     int
	OffRebounds   int
	DefRebounds   int
	PersonalFouls int
}

// ToJSON converts NBA stats to generic map for API
func (s NBAPlayerStats) ToJSON() map[string]interface{} {
	return map[string]interface{}{
		"minutes":          s.Minutes,
		"points":           s.Points,
		"rebounds":         s.Rebounds,
		"assists":          s.Assists,
		"steals":           s.Steals,
		"blocks":           s.Blocks,
		"turnovers":        s.Turnovers,
		"field_goals":      s.FieldGoals,
		"three_pointers":   s.ThreePointers,
		"free_throws":      s.FreeThrows,
		"plus_minus":       s.PlusMinus,
		"off_rebounds":     s.OffRebounds,
		"def_rebounds":     s.DefRebounds,
		"personal_fouls":   s.PersonalFouls,
	}
}

// ToDisplayStats converts NBA stats to formatted display stats for UI
func (s NBAPlayerStats) ToDisplayStats() []models.DisplayStat {
	return []models.DisplayStat{
		{Label: "MIN", Value: fmt.Sprintf("%.1f", s.Minutes), Category: "Game"},
		{Label: "PTS", Value: fmt.Sprintf("%d", s.Points), Category: "Scoring"},
		{Label: "REB", Value: fmt.Sprintf("%d", s.Rebounds), Category: "Rebounding"},
		{Label: "AST", Value: fmt.Sprintf("%d", s.Assists), Category: "Playmaking"},
		{Label: "STL", Value: fmt.Sprintf("%d", s.Steals), Category: "Defense"},
		{Label: "BLK", Value: fmt.Sprintf("%d", s.Blocks), Category: "Defense"},
		{Label: "TO", Value: fmt.Sprintf("%d", s.Turnovers), Category: "Ball Control"},
		{Label: "FG", Value: s.FieldGoals, Category: "Shooting"},
		{Label: "3PT", Value: s.ThreePointers, Category: "Shooting"},
		{Label: "FT", Value: s.FreeThrows, Category: "Shooting"},
		{Label: "+/-", Value: fmt.Sprintf("%+d", s.PlusMinus), Category: "Impact"},
	}
}



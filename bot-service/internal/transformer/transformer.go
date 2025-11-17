package transformer

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// Transformer converts Fortuna opportunity format to Talos bot format
type Transformer struct {
	alexandriaDB *sql.DB
	atlasDB      *sql.DB // For team short_name lookups
}

// Opportunity represents a Fortuna opportunity
type Opportunity struct {
	ID              int64
	OpportunityType string
	SportKey        string
	EventID         string
	MarketKey       string
	EdgePercent     float64
	Legs            []OpportunityLeg
}

// OpportunityLeg represents a single leg of an opportunity
type OpportunityLeg struct {
	BookKey     string
	OutcomeName string
	Price       int
	Point       *float64
}

// EventInfo contains event details from Alexandria
type EventInfo struct {
	HomeTeam      string
	AwayTeam      string
	EventName     string
	HomeTeamShort string // Optional: from enriched payload
	AwayTeamShort string // Optional: from enriched payload
}

// LegRequest contains the bet request for a leg
type LegRequest struct {
	BookKey     string
	OutcomeName string
	Stake       float64
	ExpectedOdds int
}

// TalosBetRequest is the format sent to Talos bots
type TalosBetRequest struct {
	Book       string
	Team1      string // Short name (lowercase)
	Team2      string // Short name (lowercase)
	BetTeam    string // Short name (lowercase)
	BetType    string
	BetPeriod  string
	BetAmount  string
	BetOdds    string
	Sport      string
	RequestID  string
}

// NewTransformer creates a new transformer
func NewTransformer(alexandriaDB *sql.DB, atlasDB *sql.DB) *Transformer {
	return &Transformer{
		alexandriaDB: alexandriaDB,
		atlasDB:      atlasDB,
	}
}

// Transform converts Fortuna format to Talos format
func (t *Transformer) Transform(
	opportunity Opportunity,
	leg OpportunityLeg,
	legReq LegRequest,
) (*TalosBetRequest, error) {
	// Fetch event info from Alexandria
	eventInfo, err := t.fetchEventInfo(opportunity.EventID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch event info: %w", err)
	}

	// Get short names from Atlas teams table
	team1Short, err := t.getShortName(opportunity.SportKey, eventInfo.AwayTeam)
	if err != nil {
		return nil, fmt.Errorf("failed to get short name for team1: %w", err)
	}

	team2Short, err := t.getShortName(opportunity.SportKey, eventInfo.HomeTeam)
	if err != nil {
		return nil, fmt.Errorf("failed to get short name for team2: %w", err)
	}

	// Determine bet_team from outcome_name
	betTeamShort, err := t.extractBetTeam(leg.OutcomeName, eventInfo.AwayTeam, eventInfo.HomeTeam, opportunity.SportKey)
	if err != nil {
		return nil, fmt.Errorf("failed to extract bet team: %w", err)
	}

	// Map market_key to bet_type
	betType, err := t.mapMarketToBetType(opportunity.MarketKey, leg.OutcomeName, leg.Point)
	if err != nil {
		return nil, fmt.Errorf("failed to map market: %w", err)
	}

	// Map book_key to Talos bot name first (needed for sport mapping)
	talosBookName := t.mapBookKeyToTalos(legReq.BookKey)

	// Map sport_key to Talos sport format (book-specific, using mapped book name)
	sport := t.mapSportKey(opportunity.SportKey, talosBookName)

	// Generate request ID
	requestID := fmt.Sprintf("fortuna_%d_%d_%s", opportunity.ID, time.Now().Unix(), legReq.BookKey)

	return &TalosBetRequest{
		Book:       talosBookName,
		Team1:      strings.ToLower(team1Short), // Convert to lowercase for bots
		Team2:      strings.ToLower(team2Short), // Convert to lowercase for bots
		BetTeam:    strings.ToLower(betTeamShort), // Convert to lowercase for bots
		BetType:    betType,
		BetPeriod:  "game", // Default, can be configurable
		BetAmount:  fmt.Sprintf("%.2f", legReq.Stake),
		BetOdds:    fmt.Sprintf("%d", leg.Price),
		Sport:      sport,
		RequestID:  requestID,
	}, nil
}

// TransformWithEventInfo converts Fortuna format to Talos format using pre-fetched event info
// This method avoids DB reads by using enriched data from API Gateway
func (t *Transformer) TransformWithEventInfo(
	opportunity Opportunity,
	eventInfo EventInfo,
	leg OpportunityLeg,
	legReq LegRequest,
) (*TalosBetRequest, error) {
	// Use pre-fetched team short names - no DB reads!
	// Prefer short names from enriched payload, fallback to extracting from full name
	team1Short := eventInfo.AwayTeamShort
	if team1Short == "" {
		team1Short = t.extractShortNameFromFull(eventInfo.AwayTeam)
	}
	
	team2Short := eventInfo.HomeTeamShort
	if team2Short == "" {
		team2Short = t.extractShortNameFromFull(eventInfo.HomeTeam)
	}
	
	// Determine bet_team from outcome_name using pre-fetched data
	betTeamShort, err := t.extractBetTeamFromPreFetched(
		leg.OutcomeName,
		eventInfo.AwayTeam,
		eventInfo.HomeTeam,
		team1Short,
		team2Short,
		opportunity.SportKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to extract bet team: %w", err)
	}

	// Map market_key to bet_type
	betType, err := t.mapMarketToBetType(opportunity.MarketKey, leg.OutcomeName, leg.Point)
	if err != nil {
		return nil, fmt.Errorf("failed to map market: %w", err)
	}

	// Map book_key to Talos bot name first (needed for sport mapping)
	talosBookName := t.mapBookKeyToTalos(legReq.BookKey)

	// Map sport_key to Talos sport format (book-specific, using mapped book name)
	sport := t.mapSportKey(opportunity.SportKey, talosBookName)

	// Generate request ID
	requestID := fmt.Sprintf("fortuna_%d_%d_%s", opportunity.ID, time.Now().Unix(), legReq.BookKey)

	return &TalosBetRequest{
		Book:       talosBookName,
		Team1:      strings.ToLower(team1Short), // Convert to lowercase for bots
		Team2:      strings.ToLower(team2Short), // Convert to lowercase for bots
		BetTeam:    strings.ToLower(betTeamShort), // Convert to lowercase for bots
		BetType:    betType,
		BetPeriod:  "game", // Default, can be configurable
		BetAmount:  fmt.Sprintf("%.2f", legReq.Stake),
		BetOdds:    fmt.Sprintf("%d", leg.Price),
		Sport:      sport,
		RequestID:  requestID,
	}, nil
}

// extractBetTeamFromPreFetched determines which team to bet on using pre-fetched team data
func (t *Transformer) extractBetTeamFromPreFetched(
	outcomeName string,
	awayTeam string,
	homeTeam string,
	awayShort string,
	homeShort string,
	sportKey string,
) (string, error) {
	// Normalize for matching
	outcomeLower := strings.ToLower(outcomeName)
	awayLower := strings.ToLower(awayShort)
	homeLower := strings.ToLower(homeShort)

	// Check for team abbreviations (e.g., "LAL", "BOS")
	awayAbbr := t.getAbbreviation(awayTeam)
	homeAbbr := t.getAbbreviation(homeTeam)

	// Try to match outcome_name to a team
	if strings.Contains(outcomeLower, awayLower) ||
		strings.Contains(outcomeLower, strings.ToLower(awayAbbr)) ||
		strings.Contains(outcomeLower, strings.ToLower(awayTeam)) {
		return awayShort, nil
	}

	if strings.Contains(outcomeLower, homeLower) ||
		strings.Contains(outcomeLower, strings.ToLower(homeAbbr)) ||
		strings.Contains(outcomeLower, strings.ToLower(homeTeam)) {
		return homeShort, nil
	}

	// For totals (over/under), we need to determine from context
	// This is a simplification - you may need more sophisticated logic
	if strings.Contains(outcomeLower, "over") {
		// For over/under, default to away team
		// Talos bots may handle this differently
		return awayShort, nil
	}

	// Default to away team if can't determine
	return awayShort, nil
}

// fetchEventInfo gets event details from Alexandria
func (t *Transformer) fetchEventInfo(eventID string) (*EventInfo, error) {
	query := `
		SELECT home_team, away_team
		FROM events
		WHERE event_id = $1
	`

	var homeTeam, awayTeam string
	err := t.alexandriaDB.QueryRow(query, eventID).Scan(&homeTeam, &awayTeam)
	if err != nil {
		return nil, err
	}

	return &EventInfo{
		HomeTeam:  homeTeam,
		AwayTeam:  awayTeam,
		EventName: fmt.Sprintf("%s @ %s", awayTeam, homeTeam),
	}, nil
}

// getShortName looks up short_name from Atlas teams table by full_name
func (t *Transformer) getShortName(sportKey string, fullName string) (string, error) {
	if t.atlasDB == nil {
		// Fallback: try to extract from full name if Atlas not available
		return t.extractShortNameFromFull(fullName), nil
	}

	query := `
		SELECT short_name
		FROM teams
		WHERE sport = $1 AND full_name = $2 AND is_active = true
		LIMIT 1
	`

	var shortName string
	err := t.atlasDB.QueryRow(query, sportKey, fullName).Scan(&shortName)
	if err != nil {
		if err == sql.ErrNoRows {
			// Fallback to extraction if not found
			return t.extractShortNameFromFull(fullName), nil
		}
		return "", err
	}

	if shortName == "" {
		// Fallback if short_name is NULL
		return t.extractShortNameFromFull(fullName), nil
	}

	return shortName, nil
}

// extractShortNameFromFull attempts to extract short name from full name
// e.g., "Los Angeles Lakers" -> "Lakers"
func (t *Transformer) extractShortNameFromFull(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) > 0 {
		// Return last word (usually the team name)
		return parts[len(parts)-1]
	}
	return fullName
}

// extractBetTeam determines which team to bet on from outcome_name
func (t *Transformer) extractBetTeam(
	outcomeName string,
	awayTeam string,
	homeTeam string,
	sportKey string,
) (string, error) {
	// Get short names for both teams
	awayShort, err := t.getShortName(sportKey, awayTeam)
	if err != nil {
		return "", err
	}

	homeShort, err := t.getShortName(sportKey, homeTeam)
	if err != nil {
		return "", err
	}

	// Normalize for matching
	outcomeLower := strings.ToLower(outcomeName)
	awayLower := strings.ToLower(awayShort)
	homeLower := strings.ToLower(homeShort)

	// Check for team abbreviations (e.g., "LAL", "BOS")
	awayAbbr := t.getAbbreviation(awayTeam)
	homeAbbr := t.getAbbreviation(homeTeam)

	// Try to match outcome_name to a team
	if strings.Contains(outcomeLower, awayLower) || 
	   strings.Contains(outcomeLower, strings.ToLower(awayAbbr)) ||
	   strings.Contains(outcomeLower, strings.ToLower(awayTeam)) {
		return awayShort, nil
	}

	if strings.Contains(outcomeLower, homeLower) || 
	   strings.Contains(outcomeLower, strings.ToLower(homeAbbr)) ||
	   strings.Contains(outcomeLower, strings.ToLower(homeTeam)) {
		return homeShort, nil
	}

	// For totals (over/under), we need to determine from context
	// This is a simplification - you may need more sophisticated logic
	if strings.Contains(outcomeLower, "over") {
		// For over/under, default to away team
		// Talos bots may handle this differently
		return awayShort, nil
	}

	// Default to away team if can't determine
	return awayShort, nil
}

// getAbbreviation attempts to get abbreviation from team name
// This is a fallback - ideally should query Atlas teams table
func (t *Transformer) getAbbreviation(teamName string) string {
	// Simple mapping - could be enhanced
	abbrMap := map[string]string{
		"Los Angeles Lakers": "LAL",
		"Boston Celtics":     "BOS",
		"Golden State Warriors": "GSW",
		"Chicago Bulls":      "CHI",
		"Miami Heat":         "MIA",
		"New York Knicks":    "NYK",
		// Add more as needed
	}
	if abbr, ok := abbrMap[teamName]; ok {
		return abbr
	}
	// Fallback: first letter of each word
	words := strings.Fields(teamName)
	if len(words) >= 2 {
		return strings.ToUpper(words[0][:1] + words[1][:1] + words[len(words)-1][:1])
	}
	return strings.ToUpper(teamName[:3])
}

// mapMarketToBetType converts Fortuna market_key to Talos bet_type
func (t *Transformer) mapMarketToBetType(marketKey string, outcomeName string, point *float64) (string, error) {
	switch marketKey {
	case "spreads":
		return "spread", nil
	case "h2h":
		return "moneyline", nil
	case "totals":
		// Determine over/under from outcome_name
		outcomeLower := strings.ToLower(outcomeName)
		if strings.Contains(outcomeLower, "over") {
			return "total_over", nil
		}
		if strings.Contains(outcomeLower, "under") {
			return "total_under", nil
		}
		// Default to over if can't determine
		return "total_over", nil
	default:
		return "moneyline", nil
	}
}

// mapSportKey converts Fortuna sport_key to Talos sport format (book-specific)
func (t *Transformer) mapSportKey(sportKey string, bookKey string) string {
	// Book-specific sport format mapping
	// BetUS uses: "nba", "nfl", "mlb"
	// BetOnline uses: "basketball/nba", "football/nfl"
	// Bovada uses: "basketball", "football", "baseball", "hockey"
	
	sportMap := map[string]map[string]string{
		"betus": {
			"basketball_nba":              "nba",
			"american_football_nfl":       "nfl",
			"baseball_mlb":                "mlb",
			"hockey_nhl":                  "nhl",
			"basketball_ncaab":             "ncaab",
			"american_football_ncaaf":      "ncaaf",
		},
		"betonline": {
			"basketball_nba":              "basketball/nba",
			"american_football_nfl":      "football/nfl",
			"baseball_mlb":                "baseball/mlb",
			"hockey_nhl":                  "hockey/nhl",
			"basketball_ncaab":             "basketball/ncaab",
			"american_football_ncaaf":     "football/ncaaf",
		},
		"bovada": {
			"basketball_nba":              "basketball",
			"american_football_nfl":       "football",
			"baseball_mlb":                "baseball",
			"hockey_nhl":                  "hockey",
			"basketball_ncaab":             "basketball",
			"american_football_ncaaf":      "football",
		},
	}

	bookMap, exists := sportMap[bookKey]
	if !exists {
		// Default to betus format
		bookMap = sportMap["betus"]
	}

	if mapped, ok := bookMap[sportKey]; ok {
		return mapped
	}

	// Default fallback
	return "nba"
}

// mapBookKeyToTalos converts Fortuna book_key to Talos bot name
func (t *Transformer) mapBookKeyToTalos(bookKey string) string {
	// Map database book keys to Talos bot names
	bookMap := map[string]string{
		"betus":      "betus",
		"betonline":  "betonline",
		"betonlineag": "betonline", // BetOnline.ag maps to betonline bot
		"bovada":     "bovada",
		"bovada.lv":  "bovada",    // Bovada.lv maps to bovada bot
		// Add more mappings as needed
	}

	if mapped, ok := bookMap[strings.ToLower(bookKey)]; ok {
		return mapped
	}

	// Default: return lowercase book_key if no mapping found
	return strings.ToLower(bookKey)
}


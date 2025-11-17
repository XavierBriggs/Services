package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// BotHandler handles bot-related requests by proxying to bot-service
type BotHandler struct {
	botServiceURL string
	holocronDB    *sql.DB
	alexandriaDB  *sql.DB
	atlasDB       *sql.DB
	httpClient    *http.Client
}

// NewBotHandler creates a new bot handler
func NewBotHandler(botServiceURL string, holocronDB, alexandriaDB, atlasDB *sql.DB) *BotHandler {
	return &BotHandler{
		botServiceURL: botServiceURL,
		holocronDB:    holocronDB,
		alexandriaDB:  alexandriaDB,
		atlasDB:       atlasDB,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PlaceBetRequest is the incoming request from the client
type PlaceBetRequest struct {
	OpportunityID int64        `json:"opportunity_id"`
	Legs          []LegRequest `json:"legs"`
}

// LegRequest contains bet details for a single leg
type LegRequest struct {
	BookKey      string  `json:"book_key"`
	OutcomeName  string  `json:"outcome_name"`
	Stake        float64 `json:"stake"`
	ExpectedOdds int     `json:"expected_odds"`
}

// EnrichedPlaceBetRequest is the enriched request sent to bot-service
type EnrichedPlaceBetRequest struct {
	OpportunityID int64        `json:"opportunity_id"`
	Legs          []LegRequest `json:"legs"`
	EventInfo     EventInfo    `json:"event_info"`
	Opportunity   Opportunity  `json:"opportunity"`
}

// EventInfo contains pre-fetched event and team data
type EventInfo struct {
	EventID       string `json:"event_id"`
	SportKey      string `json:"sport_key"`
	HomeTeam      string `json:"home_team"`
	AwayTeam      string `json:"away_team"`
	HomeTeamShort string `json:"home_team_short"`
	AwayTeamShort string `json:"away_team_short"`
}

// Opportunity contains pre-fetched opportunity data
type Opportunity struct {
	OpportunityType string  `json:"opportunity_type"`
	MarketKey       string  `json:"market_key"`
	EdgePercent     float64 `json:"edge_pct"`
}

// PlaceBetWithBot enriches and proxies bet placement requests to bot-service
func (h *BotHandler) PlaceBetWithBot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	// Read and parse request body
	var req PlaceBetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse request body", err)
		return
	}

	// Validate request
	if req.OpportunityID == 0 {
		respondError(w, http.StatusBadRequest, "opportunity_id is required", nil)
		return
	}

	if len(req.Legs) == 0 {
		respondError(w, http.StatusBadRequest, "at least one leg is required", nil)
		return
	}

	// Enrich request with event info and team data
	enrichedReq, err := h.enrichBetRequest(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to enrich request", err)
		return
	}

	// Marshal enriched request
	body, err := json.Marshal(enrichedReq)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to marshal request", err)
		return
	}

	// Forward enriched request to bot-service
	resp, err := http.Post(
		h.botServiceURL+"/api/v1/place-bet",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "bot service unavailable", err)
		return
	}
	defer resp.Body.Close()

	// Forward response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// enrichBetRequest fetches opportunity, event, and team data to enrich the request
func (h *BotHandler) enrichBetRequest(ctx context.Context, req PlaceBetRequest) (*EnrichedPlaceBetRequest, error) {
	// 1. Fetch opportunity from Holocron
	var opp struct {
		OpportunityType string
		SportKey        string
		EventID         string
		MarketKey       string
		EdgePercent     float64
	}

	oppQuery := `
		SELECT opportunity_type, sport_key, event_id, market_key, edge_pct
		FROM opportunities
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := h.holocronDB.QueryRowContext(ctx, oppQuery, req.OpportunityID).Scan(
		&opp.OpportunityType,
		&opp.SportKey,
		&opp.EventID,
		&opp.MarketKey,
		&opp.EdgePercent,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch opportunity: %w", err)
	}

	// 2. Fetch event info from Alexandria
	var eventInfo struct {
		HomeTeam string
		AwayTeam string
	}

	eventQuery := `
		SELECT home_team, away_team
		FROM events
		WHERE event_id = $1
	`

	err = h.alexandriaDB.QueryRowContext(ctx, eventQuery, opp.EventID).Scan(
		&eventInfo.HomeTeam,
		&eventInfo.AwayTeam,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch event info: %w", err)
	}

	// 3. Fetch team short names from Atlas (batch query)
	var homeShort, awayShort sql.NullString

	teamQuery := `
		SELECT 
			MAX(CASE WHEN full_name = $2 THEN short_name END) as home_short,
			MAX(CASE WHEN full_name = $3 THEN short_name END) as away_short
		FROM teams
		WHERE sport = $1 AND full_name IN ($2, $3) AND is_active = true
	`

	err = h.atlasDB.QueryRowContext(ctx, teamQuery, opp.SportKey, eventInfo.HomeTeam, eventInfo.AwayTeam).Scan(
		&homeShort,
		&awayShort,
	)

	// Fallback to extracting from full name if not found in Atlas
	homeTeamShort := homeShort.String
	if homeTeamShort == "" {
		homeTeamShort = extractShortNameFromFull(eventInfo.HomeTeam)
	}

	awayTeamShort := awayShort.String
	if awayTeamShort == "" {
		awayTeamShort = extractShortNameFromFull(eventInfo.AwayTeam)
	}

	// Build enriched request
	return &EnrichedPlaceBetRequest{
		OpportunityID: req.OpportunityID,
		Legs:          req.Legs,
		EventInfo: EventInfo{
			EventID:       opp.EventID,
			SportKey:      opp.SportKey,
			HomeTeam:      eventInfo.HomeTeam,
			AwayTeam:      eventInfo.AwayTeam,
			HomeTeamShort: homeTeamShort,
			AwayTeamShort: awayTeamShort,
		},
		Opportunity: Opportunity{
			OpportunityType: opp.OpportunityType,
			MarketKey:       opp.MarketKey,
			EdgePercent:     opp.EdgePercent,
		},
	}, nil
}

// extractShortNameFromFull attempts to extract short name from full name
// e.g., "Los Angeles Lakers" -> "Lakers"
func extractShortNameFromFull(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) > 0 {
		// Return last word (usually the team name)
		return parts[len(parts)-1]
	}
	return fullName
}

// proxyToBotService forwards requests to the bot-service
func (h *BotHandler) proxyToBotService(w http.ResponseWriter, r *http.Request, path string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s%s", h.botServiceURL, path)

	// Add query parameters
	if r.URL.RawQuery != "" {
		url = fmt.Sprintf("%s?%s", url, r.URL.RawQuery)
	}

	// Create new request
	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, url, r.Body)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create proxy request", err)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Execute request
	resp, err := h.httpClient.Do(proxyReq)
	if err != nil {
		respondError(w, http.StatusBadGateway, "bot service unavailable", err)
		return
	}
	defer resp.Body.Close()

	// Copy response headers (skip CORS headers - handled by middleware)
	for key, values := range resp.Header {
		// Skip CORS headers to avoid duplicates
		if key == "Access-Control-Allow-Origin" ||
			key == "Access-Control-Allow-Methods" ||
			key == "Access-Control-Allow-Headers" ||
			key == "Access-Control-Max-Age" ||
			key == "Access-Control-Allow-Credentials" {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

// GetBotStatus proxies bot status requests to bot-service
func (h *BotHandler) GetBotStatus(w http.ResponseWriter, r *http.Request) {
	h.proxyToBotService(w, r, "/api/v1/bot-status")
}

// GetBotsStatus proxies detailed bot status requests with balance to bot-service
func (h *BotHandler) GetBotsStatus(w http.ResponseWriter, r *http.Request) {
	h.proxyToBotService(w, r, "/api/v1/bots/status")
}

// GetRecentBets proxies recent bets requests to bot-service
func (h *BotHandler) GetRecentBets(w http.ResponseWriter, r *http.Request) {
	h.proxyToBotService(w, r, "/api/v1/bots/bets/recent")
}


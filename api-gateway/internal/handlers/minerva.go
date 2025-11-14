package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// MinervaHandler handles proxying requests to Minerva service
type MinervaHandler struct {
	minervaURL string
	httpClient *http.Client
}

// NewMinervaHandler creates a new Minerva handler
func NewMinervaHandler(minervaURL string) *MinervaHandler {
	return &MinervaHandler{
		minervaURL: minervaURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// proxyToMinerva forwards requests to the Minerva service
func (h *MinervaHandler) proxyToMinerva(w http.ResponseWriter, r *http.Request, path string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s%s", h.minervaURL, path)

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
		respondError(w, http.StatusBadGateway, "minerva service unavailable", err)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

// GetLiveGames retrieves all currently live games
func (h *MinervaHandler) GetLiveGames(w http.ResponseWriter, r *http.Request) {
	h.proxyToMinerva(w, r, "/api/v1/games/live")
}

// GetUpcomingGames retrieves upcoming scheduled games
func (h *MinervaHandler) GetUpcomingGames(w http.ResponseWriter, r *http.Request) {
	h.proxyToMinerva(w, r, "/api/v1/games/upcoming")
}

// GetGamesByDate retrieves games on a specific date
func (h *MinervaHandler) GetGamesByDate(w http.ResponseWriter, r *http.Request) {
	h.proxyToMinerva(w, r, "/api/v1/games")
}

// GetGame retrieves a specific game by ID
func (h *MinervaHandler) GetGame(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "gameID")
	path := fmt.Sprintf("/api/v1/games/%s", gameID)
	h.proxyToMinerva(w, r, path)
}

// GetGameBoxScore retrieves the box score for a game
func (h *MinervaHandler) GetGameBoxScore(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "gameID")
	path := fmt.Sprintf("/api/v1/games/%s/boxscore", gameID)
	h.proxyToMinerva(w, r, path)
}

// GetPlayer retrieves a player by ID
func (h *MinervaHandler) GetPlayer(w http.ResponseWriter, r *http.Request) {
	playerID := chi.URLParam(r, "playerID")
	path := fmt.Sprintf("/api/v1/players/%s", playerID)
	h.proxyToMinerva(w, r, path)
}

// SearchPlayers searches for players by name
func (h *MinervaHandler) SearchPlayers(w http.ResponseWriter, r *http.Request) {
	h.proxyToMinerva(w, r, "/api/v1/players/search")
}

// GetPlayerStats retrieves a player's recent game stats
func (h *MinervaHandler) GetPlayerStats(w http.ResponseWriter, r *http.Request) {
	playerID := chi.URLParam(r, "playerID")
	path := fmt.Sprintf("/api/v1/players/%s/stats", playerID)
	h.proxyToMinerva(w, r, path)
}

// GetPlayerSeasonAverages retrieves a player's season averages
func (h *MinervaHandler) GetPlayerSeasonAverages(w http.ResponseWriter, r *http.Request) {
	playerID := chi.URLParam(r, "playerID")
	path := fmt.Sprintf("/api/v1/players/%s/averages", playerID)
	h.proxyToMinerva(w, r, path)
}

// GetPlayerTrend retrieves a player's performance trend
func (h *MinervaHandler) GetPlayerTrend(w http.ResponseWriter, r *http.Request) {
	playerID := chi.URLParam(r, "playerID")
	path := fmt.Sprintf("/api/v1/players/%s/trend", playerID)
	h.proxyToMinerva(w, r, path)
}

// GetPlayerMLFeatures retrieves ML features for a player
func (h *MinervaHandler) GetPlayerMLFeatures(w http.ResponseWriter, r *http.Request) {
	playerID := chi.URLParam(r, "playerID")
	path := fmt.Sprintf("/api/v1/players/%s/ml-features", playerID)
	h.proxyToMinerva(w, r, path)
}

// GetTeamRoster retrieves a team's current roster
func (h *MinervaHandler) GetTeamRoster(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "teamID")
	path := fmt.Sprintf("/api/v1/teams/%s/roster", teamID)
	h.proxyToMinerva(w, r, path)
}

// GetTeamSchedule retrieves a team's schedule
func (h *MinervaHandler) GetTeamSchedule(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "teamID")
	path := fmt.Sprintf("/api/v1/teams/%s/schedule", teamID)
	h.proxyToMinerva(w, r, path)
}

// HealthCheck checks if Minerva service is healthy
func (h *MinervaHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/health", h.minervaURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create health check request", err)
		return
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"service": "minerva",
			"status":  "unhealthy",
			"error":   err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"service": "minerva",
			"status":  "unhealthy",
			"error":   "failed to decode health response",
		})
		return
	}

	respondJSON(w, http.StatusOK, health)
}


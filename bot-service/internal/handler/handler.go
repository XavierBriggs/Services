package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/XavierBriggs/fortuna/services/bot-service/internal/client"
	"github.com/XavierBriggs/fortuna/services/bot-service/internal/executor"
)

// BotHandler handles HTTP requests for bot operations
type BotHandler struct {
	executor    *executor.Executor
	talosClient *client.TalosClient
	holocronDB  *sql.DB
	botURLs     map[string]string
}

// NewBotHandler creates a new bot handler
func NewBotHandler(executor *executor.Executor, talosClient *client.TalosClient, holocronDB *sql.DB) *BotHandler {
	// Get bot URLs from environment variables
	botURLs := map[string]string{
		"betus":     getEnv("BETUS_BOT_URL", "http://localhost:5002"),
		"betonline": getEnv("BETONLINE_BOT_URL", "http://localhost:5003"),
		"bovada":    getEnv("BOVADA_BOT_URL", "http://localhost:5004"),
	}
	
	return &BotHandler{
		executor:    executor,
		talosClient: talosClient,
		holocronDB:  holocronDB,
		botURLs:     botURLs,
	}
}

func getEnv(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// PlaceBet handles bet placement requests
func (h *BotHandler) PlaceBet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	var req executor.PlaceBetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
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

	resp, err := h.executor.PlaceBet(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to place bet", err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// HealthCheck returns service health
func (h *BotHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "bot-service",
	})
}

// GetBotStatus returns the status of all registered bots
func (h *BotHandler) GetBotStatus(w http.ResponseWriter, r *http.Request) {
	bots, err := h.talosClient.ListBots()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get bot status", err)
		return
	}

	// Check health of each bot
	botStatuses := make(map[string]bool)
	for _, bot := range bots {
		healthy, _ := h.talosClient.CheckHealth(bot)
		botStatuses[bot] = healthy
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"bots":   bots,
		"status": botStatuses,
	})
}

// GetBotsStatus returns detailed status with balance for all bots
func (h *BotHandler) GetBotsStatus(w http.ResponseWriter, r *http.Request) {
	type BotStatus struct {
		Name           string                 `json:"name"`
		DisplayName    string                 `json:"display_name"`
		Status         string                 `json:"status"`
		LoggedIn       bool                   `json:"logged_in"`
		Balance        *string                `json:"balance"`
		SessionDuration map[string]interface{} `json:"session_duration"`
		Error          *string                `json:"error,omitempty"`
	}

	botDisplayNames := map[string]string{
		"betus":     "BetUS",
		"betonline": "BetOnline",
		"bovada":    "Bovada",
	}

	botStatuses := make([]BotStatus, 0, len(h.botURLs))
	
	for botKey, botURL := range h.botURLs {
		health, err := h.talosClient.GetBotHealthDirect(botURL)
		if err != nil {
			errorMsg := err.Error()
			botStatuses = append(botStatuses, BotStatus{
				Name:        botKey,
				DisplayName: botDisplayNames[botKey],
				Status:      "error",
				LoggedIn:    false,
				Error:       &errorMsg,
			})
			continue
		}

		status := "ok"
		if !health.LoggedIn {
			status = "offline"
		}

		botStatuses = append(botStatuses, BotStatus{
			Name:           botKey,
			DisplayName:    botDisplayNames[botKey],
			Status:         status,
			LoggedIn:       health.LoggedIn,
			Balance:        health.Balance,
			SessionDuration: health.SessionDuration,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"bots": botStatuses,
	})
}

// GetRecentBets returns recent bets from the database
func (h *BotHandler) GetRecentBets(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	botName := r.URL.Query().Get("bot")
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "50"
	}

	limit := 50
	if parsed, err := strconv.ParseInt(limitStr, 10, 32); err == nil && parsed > 0 && parsed <= 200 {
		limit = int(parsed)
	}

	// Build query
	query := `
		SELECT id, opportunity_id, sport_key, event_id, market_key, book_key, 
		       outcome_name, bet_type, stake_amount, bet_price, point, 
		       placed_at, settled_at, result, payout_amount
		FROM bets
		WHERE placed_at >= NOW() - INTERVAL '7 days'
	`
	args := []interface{}{}
	argPos := 1

	// Filter by book if specified
	bookKeyMap := map[string]string{
		"betus":     "betus",
		"betonline": "betonline",
		"bovada":    "bovada",
	}

	if botName != "" {
		if bookKey, ok := bookKeyMap[botName]; ok {
			query += " AND book_key = $" + string(rune('0'+argPos))
			args = append(args, bookKey)
			argPos++
		}
	}

	query += " ORDER BY placed_at DESC LIMIT $" + string(rune('0'+argPos))
	args = append(args, limit)

	rows, err := h.holocronDB.Query(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to query bets", err)
		return
	}
	defer rows.Close()

	type Bet struct {
		ID            int64      `json:"id"`
		OpportunityID *int64     `json:"opportunity_id,omitempty"`
		SportKey      string     `json:"sport_key"`
		EventID       string     `json:"event_id"`
		MarketKey     string     `json:"market_key"`
		BookKey       string     `json:"book_key"`
		OutcomeName   string     `json:"outcome_name"`
		BetType       string     `json:"bet_type"`
		StakeAmount   float64    `json:"stake_amount"`
		BetPrice      int        `json:"bet_price"`
		Point         *float64   `json:"point,omitempty"`
		PlacedAt      time.Time  `json:"placed_at"`
		SettledAt     *time.Time `json:"settled_at,omitempty"`
		Result        *string    `json:"result,omitempty"`
		PayoutAmount  *float64   `json:"payout_amount,omitempty"`
	}

	bets := []Bet{}
	for rows.Next() {
		var bet Bet
		err := rows.Scan(
			&bet.ID,
			&bet.OpportunityID,
			&bet.SportKey,
			&bet.EventID,
			&bet.MarketKey,
			&bet.BookKey,
			&bet.OutcomeName,
			&bet.BetType,
			&bet.StakeAmount,
			&bet.BetPrice,
			&bet.Point,
			&bet.PlacedAt,
			&bet.SettledAt,
			&bet.Result,
			&bet.PayoutAmount,
		)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to scan bet", err)
			return
		}
		bets = append(bets, bet)
	}

	if err = rows.Err(); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read bets", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"bets": bets,
		"count": len(bets),
	})
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError writes an error response
func respondError(w http.ResponseWriter, status int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errorMsg := message
	if err != nil {
		errorMsg = message + ": " + err.Error()
	}
	json.NewEncoder(w).Encode(map[string]string{
		"error": errorMsg,
	})
}


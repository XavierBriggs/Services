package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/XavierBriggs/fortuna/services/api-gateway/internal/db"
	"github.com/XavierBriggs/fortuna/services/api-gateway/pkg/models"
	"github.com/go-chi/chi/v5"
)

// BetHandler handles bet-related requests
type BetHandler struct {
	holocronDB db.HolocronDB
}

// NewBetHandler creates a new bet handler
func NewBetHandler(holocronDB db.HolocronDB) *BetHandler {
	return &BetHandler{
		holocronDB: holocronDB,
	}
}

// CreateBet creates a new bet entry and updates the user's bankroll atomically
func (h *BetHandler) CreateBet(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse request body
	var bet models.Bet
	if err := json.NewDecoder(r.Body).Decode(&bet); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Validate required fields
	if bet.SportKey == "" || bet.EventID == "" || bet.MarketKey == "" ||
		bet.BookKey == "" || bet.OutcomeName == "" || bet.BetType == "" {
		respondError(w, http.StatusBadRequest, "missing required fields", nil)
		return
	}

	if bet.StakeAmount <= 0 {
		respondError(w, http.StatusBadRequest, "stake_amount must be positive", nil)
		return
	}

	if bet.BetPrice == 0 {
		respondError(w, http.StatusBadRequest, "bet_price cannot be zero", nil)
		return
	}

	// Set default placed_at if not provided
	if bet.PlacedAt.IsZero() {
		bet.PlacedAt = time.Now()
	}

	// Get user ID (currently hardcoded, future: from auth middleware)
	userID := "default"

	// Create bet and update bankroll atomically
	result, err := h.holocronDB.CreateBetAndUpdateBankroll(ctx, &bet, userID)
	if err != nil {
		// Check for specific error types
		if strings.Contains(err.Error(), "insufficient bankroll") {
			respondError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if strings.Contains(err.Error(), "user settings not found") {
			respondError(w, http.StatusNotFound, "user settings not configured", err)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create bet", err)
		return
	}

	respondJSON(w, http.StatusCreated, result)
}

// GetBets retrieves bets with optional filters
func (h *BetHandler) GetBets(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse filters
	filters := models.BetFilters{
		SportKey: r.URL.Query().Get("sport"),
		BookKey:  r.URL.Query().Get("book"),
		Result:   r.URL.Query().Get("result"),
		Limit:    parseIntParam(r, "limit", 50),
		Offset:   parseIntParam(r, "offset", 0),
	}

	// Parse time filters
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			filters.Since = &t
		}
	}

	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			filters.Until = &t
		}
	}

	// Validate limit
	if filters.Limit > 500 {
		filters.Limit = 500
	}

	// Query bets
	bets, err := h.holocronDB.GetBets(ctx, filters)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve bets", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"bets":   bets,
		"count":  len(bets),
		"limit":  filters.Limit,
		"offset": filters.Offset,
	})
}

// GetBet retrieves a single bet by ID
func (h *BetHandler) GetBet(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get bet ID from URL
	betIDStr := chi.URLParam(r, "id")
	betID, err := strconv.ParseInt(betIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid bet ID", err)
		return
	}

	// Query bet
	bet, err := h.holocronDB.GetBetByID(ctx, betID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve bet", err)
		return
	}

	if bet == nil {
		respondError(w, http.StatusNotFound, "bet not found", nil)
		return
	}

	respondJSON(w, http.StatusOK, bet)
}

// GetBetSummary retrieves aggregate P&L statistics
func (h *BetHandler) GetBetSummary(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Query summary
	summary, err := h.holocronDB.GetBetSummary(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve summary", err)
		return
	}

	respondJSON(w, http.StatusOK, summary)
}


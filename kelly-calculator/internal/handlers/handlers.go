package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/XavierBriggs/fortuna/services/kelly-calculator/internal/calculator"
	"github.com/XavierBriggs/fortuna/services/kelly-calculator/pkg/models"
)

// Handler contains dependencies for HTTP handlers
type Handler struct {
	defaultBankroll float64
	kellyFraction   float64
	minEdge         float64
	maxPct          float64
}

// NewHandler creates a new handler
func NewHandler(defaultBankroll, kellyFraction, minEdge, maxPct float64) *Handler {
	return &Handler{
		defaultBankroll: defaultBankroll,
		kellyFraction:   kellyFraction,
		minEdge:         minEdge,
		maxPct:          maxPct,
	}
}

// HealthCheck returns service health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "kelly-calculator",
	})
}

// CalculateFromOpportunity calculates stake recommendations for an opportunity
func (h *Handler) CalculateFromOpportunity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse request
	var req models.OpportunityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	// Use defaults if not provided
	if req.Bankroll == 0 {
		req.Bankroll = h.defaultBankroll
	}
	if req.KellyFraction == 0 {
		req.KellyFraction = h.kellyFraction
	}

	// Validate bankroll
	if req.Bankroll <= 0 {
		respondError(w, http.StatusBadRequest, "bankroll must be positive")
		return
	}

	// Validate Kelly fraction
	if req.KellyFraction <= 0 || req.KellyFraction > 1.0 {
		respondError(w, http.StatusBadRequest, "kelly_fraction must be between 0 and 1")
		return
	}

	// Calculate based on opportunity type
	var response *models.KellyResponse
	var err error

	switch req.Opportunity.OpportunityType {
	case "edge":
		response, err = calculator.CalculateEdgeKelly(
			req.Opportunity,
			req.Bankroll,
			req.KellyFraction,
			h.minEdge,
			h.maxPct,
		)

	case "middle":
		response, err = calculator.CalculateMiddleKelly(
			req.Opportunity,
			req.Bankroll,
			req.KellyFraction,
			h.minEdge,
			h.maxPct,
		)

	case "scalp":
		// For scalps, use a reasonable total stake (e.g., 1% of bankroll or $1000 max)
		totalStake := req.Bankroll * 0.01
		if totalStake > 1000 {
			totalStake = 1000
		}
		response, err = calculator.CalculateScalpStakes(req.Opportunity, totalStake)

	default:
		respondError(w, http.StatusBadRequest, fmt.Sprintf("unknown opportunity type: %s", req.Opportunity.OpportunityType))
		return
	}

	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("calculation error: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, response)
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError writes an error response
func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}




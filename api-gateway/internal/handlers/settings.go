package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/XavierBriggs/fortuna/services/api-gateway/internal/db"
	"github.com/XavierBriggs/fortuna/services/api-gateway/pkg/models"
)

// SettingsHandler handles user settings requests
type SettingsHandler struct {
	holocronDB db.HolocronDB
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(holocronDB db.HolocronDB) *SettingsHandler {
	return &SettingsHandler{
		holocronDB: holocronDB,
	}
}

// GetSettings retrieves user settings
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// For now, we use a single "default" user
	// Future: extract user ID from authentication
	userID := "default"

	settings, err := h.holocronDB.GetUserSettings(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve settings", err)
		return
	}

	if settings == nil {
		respondError(w, http.StatusNotFound, "settings not found", nil)
		return
	}

	respondJSON(w, http.StatusOK, settings)
}

// UpdateSettings updates user settings
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse request body
	var update models.UserSettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Validate settings
	if update.KellyFraction <= 0 || update.KellyFraction > 1.0 {
		respondError(w, http.StatusBadRequest, "kelly_fraction must be between 0 and 1", nil)
		return
	}

	if update.MinEdgeThreshold < 0 {
		respondError(w, http.StatusBadRequest, "min_edge_threshold must be non-negative", nil)
		return
	}

	if update.MaxStakePct <= 0 || update.MaxStakePct > 100 {
		respondError(w, http.StatusBadRequest, "max_stake_pct must be between 0 and 100", nil)
		return
	}

	// Validate bankrolls (all must be non-negative)
	for book, amount := range update.Bankrolls {
		if amount < 0 {
			respondError(w, http.StatusBadRequest, "bankroll for "+book+" must be non-negative", nil)
			return
		}
	}

	// For now, we use a single "default" user
	userID := "default"

	// Update settings
	if err := h.holocronDB.UpdateUserSettings(ctx, userID, &update); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update settings", err)
		return
	}

	// Retrieve updated settings
	settings, err := h.holocronDB.GetUserSettings(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve updated settings", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "updated",
		"settings": settings,
	})
}



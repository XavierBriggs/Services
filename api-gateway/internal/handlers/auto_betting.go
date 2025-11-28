package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

// AutoBettingHandlers handles auto-betting related endpoints
type AutoBettingHandlers struct {
	holocronDB *sql.DB
}

// NewAutoBettingHandlers creates new auto-betting handlers
func NewAutoBettingHandlers(holocronDB *sql.DB) *AutoBettingHandlers {
	return &AutoBettingHandlers{
		holocronDB: holocronDB,
	}
}

// GetSettings returns auto-betting settings for a user
func (h *AutoBettingHandlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "default"
	}

	query := `
		SELECT 
			user_id,
			auto_betting_enabled,
			auto_min_edge_pct,
			auto_enabled_opportunity_types,
			auto_enabled_markets,
			auto_enabled_books,
			auto_disabled_books,
			auto_max_stake_per_bet,
			auto_max_exposure_per_event,
			auto_max_exposure_total,
			auto_max_bets_per_hour,
			auto_max_bets_per_day,
			kelly_fraction,
			auto_max_kelly_pct,
			auto_min_stake,
			auto_max_data_age_seconds,
			auto_min_time_to_start_hours,
			auto_max_time_to_start_hours,
			auto_pause_on_loss_streak,
			auto_pause_on_daily_loss,
			auto_correlation_discount,
			auto_middle_enabled,
			auto_middle_execution_strategy,
			auto_middle_required_legs,
			auto_scalp_enabled,
			auto_scalp_bankroll_pct,
			auto_scalp_min_profit_pct,
			auto_edge_allow_live_games
		FROM user_settings
		WHERE user_id = $1
	`

	var settings struct {
		UserID                           string   `json:"user_id"`
		AutoBettingEnabled               bool     `json:"auto_betting_enabled"`
		AutoMinEdgePct                   float64  `json:"auto_min_edge_pct"`
		AutoEnabledOpportunityTypes      []string `json:"auto_enabled_opportunity_types"`
		AutoEnabledMarkets               []string `json:"auto_enabled_markets"`
		AutoEnabledBooks                 []string `json:"auto_enabled_books"`
		AutoDisabledBooks                []string `json:"auto_disabled_books"`
		AutoMaxStakePerBet               float64  `json:"auto_max_stake_per_bet"`
		AutoMaxExposurePerEvent          float64  `json:"auto_max_exposure_per_event"`
		AutoMaxExposureTotal             float64  `json:"auto_max_exposure_total"`
		AutoMaxBetsPerHour               int      `json:"auto_max_bets_per_hour"`
		AutoMaxBetsPerDay                int      `json:"auto_max_bets_per_day"`
		KellyFraction                    float64  `json:"kelly_fraction"`
		AutoMaxKellyPct                  float64  `json:"auto_max_kelly_pct"`
		AutoMinStake                     float64  `json:"auto_min_stake"`
		AutoMaxDataAgeSeconds            int      `json:"auto_max_data_age_seconds"`
		AutoMinTimeToStartHours          int      `json:"auto_min_time_to_start_hours"`
		AutoMaxTimeToStartHours          int      `json:"auto_max_time_to_start_hours"`
		AutoPauseOnLossStreak            int      `json:"auto_pause_on_loss_streak"`
		AutoPauseOnDailyLoss             float64  `json:"auto_pause_on_daily_loss"`
		AutoCorrelationDiscount          float64  `json:"auto_correlation_discount"`
		AutoMiddleEnabled                bool     `json:"auto_middle_enabled"`
		AutoMiddleExecutionStrategy      string   `json:"auto_middle_execution_strategy"`
		AutoMiddleRequiredLegs           int      `json:"auto_middle_required_legs"`
		AutoScalpEnabled                 bool     `json:"auto_scalp_enabled"`
		AutoScalpBankrollPct             float64  `json:"auto_scalp_bankroll_pct"`
		AutoScalpMinProfitPct            float64  `json:"auto_scalp_min_profit_pct"`
		AutoEdgeAllowLiveGames           bool     `json:"auto_edge_allow_live_games"`
	}

	err := h.holocronDB.QueryRow(query, userID).Scan(
		&settings.UserID,
		&settings.AutoBettingEnabled,
		&settings.AutoMinEdgePct,
		pq.Array(&settings.AutoEnabledOpportunityTypes),
		pq.Array(&settings.AutoEnabledMarkets),
		pq.Array(&settings.AutoEnabledBooks),
		pq.Array(&settings.AutoDisabledBooks),
		&settings.AutoMaxStakePerBet,
		&settings.AutoMaxExposurePerEvent,
		&settings.AutoMaxExposureTotal,
		&settings.AutoMaxBetsPerHour,
		&settings.AutoMaxBetsPerDay,
		&settings.KellyFraction,
		&settings.AutoMaxKellyPct,
		&settings.AutoMinStake,
		&settings.AutoMaxDataAgeSeconds,
		&settings.AutoMinTimeToStartHours,
		&settings.AutoMaxTimeToStartHours,
		&settings.AutoPauseOnLossStreak,
		&settings.AutoPauseOnDailyLoss,
		&settings.AutoCorrelationDiscount,
		&settings.AutoMiddleEnabled,
		&settings.AutoMiddleExecutionStrategy,
		&settings.AutoMiddleRequiredLegs,
		&settings.AutoScalpEnabled,
		&settings.AutoScalpBankrollPct,
		&settings.AutoScalpMinProfitPct,
		&settings.AutoEdgeAllowLiveGames,
	)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// UpdateSettings updates auto-betting settings
func (h *AutoBettingHandlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" && r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var settings map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, ok := settings["user_id"].(string)
	if !ok {
		userID = "default"
	}

	// Build dynamic UPDATE query based on provided fields
	// For simplicity, we'll update all fields that exist in the request
	// In production, you'd want more granular control

	query := `
		UPDATE user_settings
		SET 
			auto_betting_enabled = COALESCE($2, auto_betting_enabled),
			auto_min_edge_pct = COALESCE($3, auto_min_edge_pct),
			auto_max_stake_per_bet = COALESCE($4, auto_max_stake_per_bet),
			auto_max_exposure_total = COALESCE($5, auto_max_exposure_total),
			auto_middle_enabled = COALESCE($6, auto_middle_enabled),
			auto_scalp_enabled = COALESCE($7, auto_scalp_enabled),
			kelly_fraction = COALESCE($8, kelly_fraction)
		WHERE user_id = $1
	`

	_, err := h.holocronDB.Exec(query,
		userID,
		settings["auto_betting_enabled"],
		settings["auto_min_edge_pct"],
		settings["auto_max_stake_per_bet"],
		settings["auto_max_exposure_total"],
		settings["auto_middle_enabled"],
		settings["auto_scalp_enabled"],
		settings["kelly_fraction"],
	)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// GetState returns current auto-betting state
func (h *AutoBettingHandlers) GetState(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "default"
	}

	query := `
		SELECT 
			user_id,
			total_exposure,
			exposure_by_event,
			exposure_by_book,
			bets_placed_last_hour,
			bets_placed_today,
			COALESCE(last_bet_placed_at, NOW()) as last_bet_placed_at,
			todays_pnl,
			current_loss_streak,
			total_bets_placed,
			total_bets_won,
			total_bets_lost,
			is_paused,
			COALESCE(pause_reason, '') as pause_reason,
			last_updated
		FROM auto_betting_state
		WHERE user_id = $1
	`

	var state struct {
		UserID             string                 `json:"user_id"`
		TotalExposure      float64                `json:"total_exposure"`
		ExposureByEvent    map[string]interface{} `json:"exposure_by_event"`
		ExposureByBook     map[string]interface{} `json:"exposure_by_book"`
		BetsPlacedLastHour int                    `json:"bets_placed_last_hour"`
		BetsPlacedToday    int                    `json:"bets_placed_today"`
		LastBetPlacedAt    string                 `json:"last_bet_placed_at"`
		TodaysPnL          float64                `json:"todays_pnl"`
		CurrentLossStreak  int                    `json:"current_loss_streak"`
		TotalBetsPlaced    int64                  `json:"total_bets_placed"`
		TotalBetsWon       int64                  `json:"total_bets_won"`
		TotalBetsLost      int64                  `json:"total_bets_lost"`
		IsPaused           bool                   `json:"is_paused"`
		PauseReason        string                 `json:"pause_reason"`
		LastUpdated        string                 `json:"last_updated"`
	}

	var exposureByEventJSON, exposureByBookJSON []byte

	err := h.holocronDB.QueryRow(query, userID).Scan(
		&state.UserID,
		&state.TotalExposure,
		&exposureByEventJSON,
		&exposureByBookJSON,
		&state.BetsPlacedLastHour,
		&state.BetsPlacedToday,
		&state.LastBetPlacedAt,
		&state.TodaysPnL,
		&state.CurrentLossStreak,
		&state.TotalBetsPlaced,
		&state.TotalBetsWon,
		&state.TotalBetsLost,
		&state.IsPaused,
		&state.PauseReason,
		&state.LastUpdated,
	)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse JSON fields
	state.ExposureByEvent = make(map[string]interface{})
	state.ExposureByBook = make(map[string]interface{})

	if len(exposureByEventJSON) > 0 {
		json.Unmarshal(exposureByEventJSON, &state.ExposureByEvent)
	}

	if len(exposureByBookJSON) > 0 {
		json.Unmarshal(exposureByBookJSON, &state.ExposureByBook)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// PauseAutoBetting pauses auto-betting
func (h *AutoBettingHandlers) PauseAutoBetting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Reason string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		req.UserID = "default"
	}

	if req.Reason == "" {
		req.Reason = "manual pause"
	}

	_, err := h.holocronDB.Exec(`
		UPDATE auto_betting_state
		SET is_paused = true, pause_reason = $2, paused_at = NOW()
		WHERE user_id = $1
	`, req.UserID, req.Reason)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

// ResumeAutoBetting resumes auto-betting
func (h *AutoBettingHandlers) ResumeAutoBetting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		req.UserID = "default"
	}

	_, err := h.holocronDB.Exec(`
		UPDATE auto_betting_state
		SET is_paused = false, pause_reason = NULL, paused_at = NULL
		WHERE user_id = $1
	`, req.UserID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

// GetDecisions returns auto-betting decision history
func (h *AutoBettingHandlers) GetDecisions(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "default"
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		// Parse limit
		// For simplicity, using default
	}

	query := `
		SELECT 
			id,
			opportunity_id,
			decision,
			decision_reason,
			calculated_stake,
			calculated_edge,
			execution_time_ms,
			current_exposure,
			current_bankroll,
			bets_placed_today,
			created_at
		FROM auto_betting_decisions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := h.holocronDB.Query(query, userID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	decisions := []map[string]interface{}{}

	for rows.Next() {
		var d struct {
			ID               int64      `json:"id"`
			OpportunityID    int64      `json:"opportunity_id"`
			Decision         string     `json:"decision"`
			DecisionReason   string     `json:"decision_reason"`
			CalculatedStake  *float64   `json:"calculated_stake"`
			CalculatedEdge   *float64   `json:"calculated_edge"`
			ExecutionTimeMs  *int       `json:"execution_time_ms"`
			CurrentExposure  float64    `json:"current_exposure"`
			CurrentBankroll  float64    `json:"current_bankroll"`
			BetsPlacedToday  int        `json:"bets_placed_today"`
			CreatedAt        string     `json:"created_at"`
		}

		err := rows.Scan(
			&d.ID,
			&d.OpportunityID,
			&d.Decision,
			&d.DecisionReason,
			&d.CalculatedStake,
			&d.CalculatedEdge,
			&d.ExecutionTimeMs,
			&d.CurrentExposure,
			&d.CurrentBankroll,
			&d.BetsPlacedToday,
			&d.CreatedAt,
		)

		if err != nil {
			continue
		}

		decisions = append(decisions, map[string]interface{}{
			"id":                d.ID,
			"opportunity_id":    d.OpportunityID,
			"decision":          d.Decision,
			"decision_reason":   d.DecisionReason,
			"calculated_stake":  d.CalculatedStake,
			"calculated_edge":   d.CalculatedEdge,
			"execution_time_ms": d.ExecutionTimeMs,
			"current_exposure":  d.CurrentExposure,
			"current_bankroll":  d.CurrentBankroll,
			"bets_placed_today": d.BetsPlacedToday,
			"created_at":        d.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"decisions": decisions,
		"total":     len(decisions),
	})
}


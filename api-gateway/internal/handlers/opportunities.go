package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// OpportunityHandler handles opportunity-related endpoints
type OpportunityHandler struct {
	holocronDB *sql.DB
}

// NewOpportunityHandler creates a new opportunity handler
func NewOpportunityHandler(holocronDB *sql.DB) *OpportunityHandler {
	return &OpportunityHandler{
		holocronDB: holocronDB,
	}
}

// GetOpportunities retrieves opportunities with filtering
// Query params: type, sport, since, limit, offset
func (h *OpportunityHandler) GetOpportunities(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse query parameters
	oppType := r.URL.Query().Get("type")
	sportKey := r.URL.Query().Get("sport")
	sinceStr := r.URL.Query().Get("since")
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	// Build query
	query := `
		SELECT o.id, o.opportunity_type, o.sport_key, o.event_id, o.market_key,
		       o.edge_pct, o.fair_price, o.detected_at, o.data_age_seconds
		FROM opportunities o
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	if oppType != "" {
		query += fmt.Sprintf(" AND o.opportunity_type = $%d", argCount)
		args = append(args, oppType)
		argCount++
	}

	if sportKey != "" {
		query += fmt.Sprintf(" AND o.sport_key = $%d", argCount)
		args = append(args, sportKey)
		argCount++
	}

	if sinceStr != "" {
		if since, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			query += fmt.Sprintf(" AND o.detected_at >= $%d", argCount)
			args = append(args, since)
			argCount++
		}
	}

	query += " ORDER BY o.detected_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, limit, offset)

	// Execute query
	rows, err := h.holocronDB.QueryContext(ctx, query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to query opportunities", err)
		return
	}
	defer rows.Close()

	opportunities := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var oppType, sportKey, eventID, marketKey string
		var edgePct float64
		var fairPrice sql.NullInt32
		var detectedAt time.Time
		var dataAge int

		err := rows.Scan(&id, &oppType, &sportKey, &eventID, &marketKey,
			&edgePct, &fairPrice, &detectedAt, &dataAge)
		if err != nil {
			continue
		}

		opp := map[string]interface{}{
			"id":               id,
			"opportunity_type": oppType,
			"sport_key":        sportKey,
			"event_id":         eventID,
			"market_key":       marketKey,
			"edge_pct":         edgePct,
			"detected_at":      detectedAt,
			"data_age_seconds": dataAge,
		}

		if fairPrice.Valid {
			opp["fair_price"] = fairPrice.Int32
		}

		// Get legs for this opportunity
		legs, _ := h.getOpportunityLegs(ctx, id)
		opp["legs"] = legs

		opportunities = append(opportunities, opp)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"opportunities": opportunities,
		"count":         len(opportunities),
		"limit":         limit,
		"offset":        offset,
	})
}

// GetOpportunity retrieves a single opportunity by ID
func (h *OpportunityHandler) GetOpportunity(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid opportunity ID", err)
		return
	}

	query := `
		SELECT o.id, o.opportunity_type, o.sport_key, o.event_id, o.market_key,
		       o.edge_pct, o.fair_price, o.detected_at, o.data_age_seconds
		FROM opportunities o
		WHERE o.id = $1
	`

	var oppType, sportKey, eventID, marketKey string
	var edgePct float64
	var fairPrice sql.NullInt32
	var detectedAt time.Time
	var dataAge int

	err = h.holocronDB.QueryRowContext(ctx, query, id).Scan(
		&id, &oppType, &sportKey, &eventID, &marketKey,
		&edgePct, &fairPrice, &detectedAt, &dataAge)

	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "opportunity not found", nil)
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to query opportunity", err)
		return
	}

	opp := map[string]interface{}{
		"id":               id,
		"opportunity_type": oppType,
		"sport_key":        sportKey,
		"event_id":         eventID,
		"market_key":       marketKey,
		"edge_pct":         edgePct,
		"detected_at":      detectedAt,
		"data_age_seconds": dataAge,
	}

	if fairPrice.Valid {
		opp["fair_price"] = fairPrice.Int32
	}

	// Get legs
	legs, _ := h.getOpportunityLegs(ctx, id)
	opp["legs"] = legs

	respondJSON(w, http.StatusOK, opp)
}

// CreateOpportunityAction creates an action on an opportunity
func (h *OpportunityHandler) CreateOpportunityAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	idStr := chi.URLParam(r, "id")
	opportunityID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid opportunity ID", err)
		return
	}

	// Parse request body
	var req struct {
		ActionType string `json:"action_type"`
		Operator   string `json:"operator"`
		Notes      string `json:"notes,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Validate action type
	validActions := map[string]bool{"taken": true, "dismissed": true, "noted": true}
	if !validActions[req.ActionType] {
		respondError(w, http.StatusBadRequest, "invalid action_type", nil)
		return
	}

	// Insert action
	query := `
		INSERT INTO opportunity_actions (opportunity_id, action_type, operator, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, action_time
	`

	var actionID int64
	var actionTime time.Time

	err = h.holocronDB.QueryRowContext(ctx, query,
		opportunityID, req.ActionType, req.Operator, req.Notes).Scan(&actionID, &actionTime)

	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create action", err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           actionID,
		"action_type":  req.ActionType,
		"action_time":  actionTime,
		"operator":     req.Operator,
		"notes":        req.Notes,
	})
}

// getOpportunityLegs retrieves legs for an opportunity
func (h *OpportunityHandler) getOpportunityLegs(ctx context.Context, opportunityID int64) ([]map[string]interface{}, error) {
	query := `
		SELECT book_key, outcome_name, price, point, leg_edge_pct
		FROM opportunity_legs
		WHERE opportunity_id = $1
		ORDER BY id
	`

	rows, err := h.holocronDB.QueryContext(ctx, query, opportunityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	legs := []map[string]interface{}{}
	for rows.Next() {
		var bookKey, outcomeName string
		var price int
		var point sql.NullFloat64
		var legEdge sql.NullFloat64

		if err := rows.Scan(&bookKey, &outcomeName, &price, &point, &legEdge); err != nil {
			continue
		}

		leg := map[string]interface{}{
			"book_key":     bookKey,
			"outcome_name": outcomeName,
			"price":        price,
		}

		if point.Valid {
			leg["point"] = point.Float64
		}
		if legEdge.Valid {
			leg["leg_edge_pct"] = legEdge.Float64
		}

		legs = append(legs, leg)
	}

	return legs, nil
}


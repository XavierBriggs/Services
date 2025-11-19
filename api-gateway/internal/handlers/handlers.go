package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/XavierBriggs/fortuna/services/api-gateway/internal/db"
	"github.com/XavierBriggs/fortuna/services/api-gateway/pkg/models"
	"github.com/go-chi/chi/v5"
)

// Handler contains dependencies for HTTP handlers
type Handler struct {
	db db.AlexandriaDB
}

// NewHandler creates a new handler with dependencies
func NewHandler(database db.AlexandriaDB) *Handler {
	return &Handler{
		db: database,
	}
}

// HealthCheck returns the health status of the API
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	// Check database connectivity
	if err := h.db.Ping(ctx); err != nil {
		respondError(w, http.StatusServiceUnavailable, "database unhealthy", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"timestamp": time.Now().UTC(),
		"service": "api-gateway",
	})
}

// GetEvents retrieves events with optional filtering
// Query params: sport, status, limit, offset
func (h *Handler) GetEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse query parameters
	sportKey := r.URL.Query().Get("sport")
	status := r.URL.Query().Get("status")
	limit := parseIntParam(r, "limit", 100)
	offset := parseIntParam(r, "offset", 0)

	// Validate limit
	if limit > 500 {
		limit = 500
	}

	filters := db.EventFilters{
		SportKey:    sportKey,
		EventStatus: status,
		Limit:       limit,
		Offset:      offset,
	}

	events, err := h.db.GetEvents(ctx, filters)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve events", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
		"limit":  limit,
		"offset": offset,
	})
}

// GetEvent retrieves a single event by ID
func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	eventID := chi.URLParam(r, "eventID")
	if eventID == "" {
		respondError(w, http.StatusBadRequest, "event_id is required", nil)
		return
	}

	event, err := h.db.GetEvent(ctx, eventID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve event", err)
		return
	}

	if event == nil {
		respondError(w, http.StatusNotFound, "event not found", nil)
		return
	}

	respondJSON(w, http.StatusOK, event)
}

// GetCurrentOdds retrieves the latest odds with filtering
// Query params: event_id, sport, market, book, limit, offset
func (h *Handler) GetCurrentOdds(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse query parameters
	eventID := r.URL.Query().Get("event_id")
	sportKey := r.URL.Query().Get("sport")
	marketKey := r.URL.Query().Get("market")
	bookKey := r.URL.Query().Get("book")
	limit := parseIntParam(r, "limit", 1000)
	offset := parseIntParam(r, "offset", 0)

	// Validate limit
	if limit > 5000 {
		limit = 5000
	}

	filters := db.OddsFilters{
		EventID:   eventID,
		SportKey:  sportKey,
		MarketKey: marketKey,
		BookKey:   bookKey,
		Limit:     limit,
		Offset:    offset,
	}

	odds, err := h.db.GetCurrentOdds(ctx, filters)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve odds", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"odds":   odds,
		"count":  len(odds),
		"limit":  limit,
		"offset": offset,
	})
}

// GetOddsHistory retrieves historical odds data
// Query params: event_id, market, book, since, until, limit, offset
func (h *Handler) GetOddsHistory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	eventID := r.URL.Query().Get("event_id")
	marketKey := r.URL.Query().Get("market")
	bookKey := r.URL.Query().Get("book")
	limit := parseIntParam(r, "limit", 1000)
	offset := parseIntParam(r, "offset", 0)

	// Parse time filters
	var since, until *time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &t
		}
	}
	if untilStr := r.URL.Query().Get("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = &t
		}
	}

	// Validate limit
	if limit > 10000 {
		limit = 10000
	}

	filters := db.OddsHistoryFilters{
		EventID:   eventID,
		MarketKey: marketKey,
		BookKey:   bookKey,
		Since:     since,
		Until:     until,
		Limit:     limit,
		Offset:    offset,
	}

	history, err := h.db.GetOddsHistory(ctx, filters)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve odds history", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"history": history,
		"count":   len(history),
		"limit":   limit,
		"offset":  offset,
	})
}

// GetEventWithOdds retrieves an event with its current odds
func (h *Handler) GetEventWithOdds(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	eventID := chi.URLParam(r, "eventID")
	if eventID == "" {
		respondError(w, http.StatusBadRequest, "event_id is required", nil)
		return
	}

	eventWithOdds, err := h.db.GetEventWithOdds(ctx, eventID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve event with odds", err)
		return
	}

	if eventWithOdds == nil {
		respondError(w, http.StatusNotFound, "event not found", nil)
		return
	}

	respondJSON(w, http.StatusOK, eventWithOdds)
}

// GetBooks retrieves all books from Alexandria database
func (h *Handler) GetBooks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	books, err := h.db.GetBooks(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve books", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"books": books,
		"count": len(books),
	})
}

// Helper functions

func parseIntParam(r *http.Request, param string, defaultValue int) int {
	valueStr := r.URL.Query().Get(param)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("error encoding response: %v\n", err)
	}
}

func respondError(w http.ResponseWriter, status int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	errResp := models.ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	}

	if err != nil {
		fmt.Printf("error: %s - %v\n", message, err)
	}

	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		fmt.Printf("error encoding error response: %v\n", err)
	}
}


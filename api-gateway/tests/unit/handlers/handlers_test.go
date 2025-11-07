package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/XavierBriggs/fortuna/services/api-gateway/internal/db"
	"github.com/XavierBriggs/fortuna/services/api-gateway/internal/handlers"
	"github.com/XavierBriggs/fortuna/services/api-gateway/pkg/models"
	"github.com/go-chi/chi/v5"
)

// MockDB implements db.AlexandriaDB for testing
type MockDB struct {
	events       []models.Event
	currentOdds  []models.CurrentOdds
	oddsHistory  []models.OddsHistory
	shouldError  bool
}

func (m *MockDB) GetEvents(ctx context.Context, filters db.EventFilters) ([]models.Event, error) {
	if m.shouldError {
		return nil, context.DeadlineExceeded
	}
	return m.events, nil
}

func (m *MockDB) GetEvent(ctx context.Context, eventID string) (*models.Event, error) {
	if m.shouldError {
		return nil, context.DeadlineExceeded
	}
	for _, e := range m.events {
		if e.EventID == eventID {
			return &e, nil
		}
	}
	return nil, nil
}

func (m *MockDB) GetCurrentOdds(ctx context.Context, filters db.OddsFilters) ([]models.CurrentOdds, error) {
	if m.shouldError {
		return nil, context.DeadlineExceeded
	}
	return m.currentOdds, nil
}

func (m *MockDB) GetOddsHistory(ctx context.Context, filters db.OddsHistoryFilters) ([]models.OddsHistory, error) {
	if m.shouldError {
		return nil, context.DeadlineExceeded
	}
	return m.oddsHistory, nil
}

func (m *MockDB) GetEventWithOdds(ctx context.Context, eventID string) (*models.EventWithOdds, error) {
	if m.shouldError {
		return nil, context.DeadlineExceeded
	}
	event, _ := m.GetEvent(ctx, eventID)
	if event == nil {
		return nil, nil
	}
	odds, _ := m.GetCurrentOdds(ctx, db.OddsFilters{EventID: eventID})
	return &models.EventWithOdds{
		Event:       *event,
		CurrentOdds: odds,
	}, nil
}

func (m *MockDB) Close() error {
	return nil
}

func (m *MockDB) Ping(ctx context.Context) error {
	if m.shouldError {
		return context.DeadlineExceeded
	}
	return nil
}

func TestHealthCheck_Success(t *testing.T) {
	mockDB := &MockDB{}
	handler := handlers.NewHandler(mockDB)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", response["status"])
	}
}

func TestHealthCheck_DatabaseUnhealthy(t *testing.T) {
	mockDB := &MockDB{shouldError: true}
	handler := handlers.NewHandler(mockDB)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

func TestGetEvents_Success(t *testing.T) {
	mockDB := &MockDB{
		events: []models.Event{
			{
				EventID:      "event1",
				SportKey:     "basketball_nba",
				HomeTeam:     "Lakers",
				AwayTeam:     "Celtics",
				CommenceTime: time.Now().Add(2 * time.Hour),
				EventStatus:  "upcoming",
			},
		},
	}
	handler := handlers.NewHandler(mockDB)

	req := httptest.NewRequest("GET", "/api/v1/events?sport=basketball_nba", nil)
	w := httptest.NewRecorder()

	handler.GetEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	events := response["events"].([]interface{})
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

func TestGetEvent_Success(t *testing.T) {
	mockDB := &MockDB{
		events: []models.Event{
			{
				EventID:      "event1",
				SportKey:     "basketball_nba",
				HomeTeam:     "Lakers",
				AwayTeam:     "Celtics",
				CommenceTime: time.Now().Add(2 * time.Hour),
				EventStatus:  "upcoming",
			},
		},
	}
	handler := handlers.NewHandler(mockDB)

	// Setup chi router to handle URL params
	r := chi.NewRouter()
	r.Get("/events/{eventID}", handler.GetEvent)

	req := httptest.NewRequest("GET", "/events/event1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var event models.Event
	if err := json.NewDecoder(w.Body).Decode(&event); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if event.EventID != "event1" {
		t.Errorf("expected event_id 'event1', got %s", event.EventID)
	}
}

func TestGetEvent_NotFound(t *testing.T) {
	mockDB := &MockDB{
		events: []models.Event{},
	}
	handler := handlers.NewHandler(mockDB)

	r := chi.NewRouter()
	r.Get("/events/{eventID}", handler.GetEvent)

	req := httptest.NewRequest("GET", "/events/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetCurrentOdds_Success(t *testing.T) {
	point := 7.5
	mockDB := &MockDB{
		currentOdds: []models.CurrentOdds{
			{
				EventID:          "event1",
				SportKey:         "basketball_nba",
				MarketKey:        "spreads",
				BookKey:          "fanduel",
				OutcomeName:      "Lakers",
				Price:            -110,
				Point:            &point,
				VendorLastUpdate: time.Now(),
				ReceivedAt:       time.Now(),
				DataAge:          5.0,
			},
		},
	}
	handler := handlers.NewHandler(mockDB)

	req := httptest.NewRequest("GET", "/api/v1/odds/current?event_id=event1", nil)
	w := httptest.NewRecorder()

	handler.GetCurrentOdds(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	odds := response["odds"].([]interface{})
	if len(odds) != 1 {
		t.Errorf("expected 1 odds entry, got %d", len(odds))
	}
}

func TestGetCurrentOdds_WithFilters(t *testing.T) {
	mockDB := &MockDB{
		currentOdds: []models.CurrentOdds{},
	}
	handler := handlers.NewHandler(mockDB)

	// Test with multiple filters
	req := httptest.NewRequest("GET", "/api/v1/odds/current?sport=basketball_nba&market=spreads&book=fanduel&limit=50", nil)
	w := httptest.NewRecorder()

	handler.GetCurrentOdds(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetOddsHistory_Success(t *testing.T) {
	point := 7.5
	mockDB := &MockDB{
		oddsHistory: []models.OddsHistory{
			{
				ID:               1,
				EventID:          "event1",
				SportKey:         "basketball_nba",
				MarketKey:        "spreads",
				BookKey:          "fanduel",
				OutcomeName:      "Lakers",
				Price:            -110,
				Point:            &point,
				VendorLastUpdate: time.Now(),
				ReceivedAt:       time.Now(),
				IsLatest:         false,
			},
		},
	}
	handler := handlers.NewHandler(mockDB)

	req := httptest.NewRequest("GET", "/api/v1/odds/history?event_id=event1", nil)
	w := httptest.NewRecorder()

	handler.GetOddsHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	history := response["history"].([]interface{})
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestGetOddsHistory_WithTimeFilters(t *testing.T) {
	mockDB := &MockDB{
		oddsHistory: []models.OddsHistory{},
	}
	handler := handlers.NewHandler(mockDB)

	// Test with RFC3339 time format
	since := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	until := time.Now().Format(time.RFC3339)

	req := httptest.NewRequest("GET", "/api/v1/odds/history?event_id=event1&since="+since+"&until="+until, nil)
	w := httptest.NewRecorder()

	handler.GetOddsHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetEventWithOdds_Success(t *testing.T) {
	point := 7.5
	mockDB := &MockDB{
		events: []models.Event{
			{
				EventID:      "event1",
				SportKey:     "basketball_nba",
				HomeTeam:     "Lakers",
				AwayTeam:     "Celtics",
				CommenceTime: time.Now().Add(2 * time.Hour),
				EventStatus:  "upcoming",
			},
		},
		currentOdds: []models.CurrentOdds{
			{
				EventID:     "event1",
				SportKey:    "basketball_nba",
				MarketKey:   "spreads",
				BookKey:     "fanduel",
				OutcomeName: "Lakers",
				Price:       -110,
				Point:       &point,
			},
		},
	}
	handler := handlers.NewHandler(mockDB)

	r := chi.NewRouter()
	r.Get("/events/{eventID}/odds", handler.GetEventWithOdds)

	req := httptest.NewRequest("GET", "/events/event1/odds", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response models.EventWithOdds
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Event.EventID != "event1" {
		t.Errorf("expected event_id 'event1', got %s", response.Event.EventID)
	}

	if len(response.CurrentOdds) != 1 {
		t.Errorf("expected 1 odds entry, got %d", len(response.CurrentOdds))
	}
}

func TestGetEventWithOdds_NotFound(t *testing.T) {
	mockDB := &MockDB{
		events: []models.Event{},
	}
	handler := handlers.NewHandler(mockDB)

	r := chi.NewRouter()
	r.Get("/events/{eventID}/odds", handler.GetEventWithOdds)

	req := httptest.NewRequest("GET", "/events/nonexistent/odds", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestErrorHandling(t *testing.T) {
	mockDB := &MockDB{shouldError: true}
	handler := handlers.NewHandler(mockDB)

	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
	}{
		{"GetEvents error", handler.GetEvents, "/api/v1/events"},
		{"GetCurrentOdds error", handler.GetCurrentOdds, "/api/v1/odds/current"},
		{"GetOddsHistory error", handler.GetOddsHistory, "/api/v1/odds/history"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			tt.handler(w, req)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("expected status 500, got %d", w.Code)
			}

			var errResp models.ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errResp.Code != http.StatusInternalServerError {
				t.Errorf("expected error code 500, got %d", errResp.Code)
			}
		})
	}
}



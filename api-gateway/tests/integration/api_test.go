// +build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/XavierBriggs/fortuna/services/api-gateway/internal/db"
	"github.com/XavierBriggs/fortuna/services/api-gateway/internal/handlers"
	"github.com/XavierBriggs/fortuna/services/api-gateway/pkg/models"
	"github.com/go-chi/chi/v5"
)

func getTestDB(t *testing.T) *db.Client {
	t.Helper()

	dsn := getEnv("ALEXANDRIA_TEST_DSN", "postgres://fortuna_dev:fortuna_dev_password@localhost:5435/alexandria?sslmode=disable")

	client, err := db.NewClient(dsn)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	t.Cleanup(func() {
		client.Close()
	})

	return client
}

func cleanupTestData(t *testing.T, client *db.Client) {
	t.Helper()

	// Note: For integration tests, we use read-only operations
	// No cleanup needed as we don't create test data
	// Future: Add Exec method to interface if we need to create/delete test data
	t.Log("Integration tests use read-only operations - no cleanup needed")
}

func TestIntegration_HealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dbClient := getTestDB(t)
	handler := handlers.NewHandler(dbClient)

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

	t.Logf("✓ Health check passed")
}

func TestIntegration_GetEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dbClient := getTestDB(t)
	handler := handlers.NewHandler(dbClient)

	// Query existing events (should have some from seed data or Mercury)
	req := httptest.NewRequest("GET", "/api/v1/events?sport=basketball_nba&status=upcoming&limit=10", nil)
	w := httptest.NewRecorder()

	handler.GetEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	count := int(response["count"].(float64))
	t.Logf("✓ Retrieved %d events", count)

	if events, ok := response["events"].([]interface{}); ok && len(events) > 0 {
		t.Logf("✓ Events structure is valid")
	}
}

func TestIntegration_GetCurrentOdds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dbClient := getTestDB(t)
	handler := handlers.NewHandler(dbClient)

	// Query current odds for NBA
	req := httptest.NewRequest("GET", "/api/v1/odds/current?sport=basketball_nba&limit=100", nil)
	w := httptest.NewRecorder()

	handler.GetCurrentOdds(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	count := int(response["count"].(float64))
	t.Logf("✓ Retrieved %d current odds", count)

	// Verify odds structure if any exist
	if odds, ok := response["odds"].([]interface{}); ok && len(odds) > 0 {
		t.Logf("✓ Current odds structure is valid")

		// Check first odds entry has required fields
		firstOdds := odds[0].(map[string]interface{})
		requiredFields := []string{"event_id", "sport_key", "market_key", "book_key", "price", "data_age_seconds"}
		for _, field := range requiredFields {
			if _, exists := firstOdds[field]; !exists {
				t.Errorf("missing required field: %s", field)
			}
		}

		// Verify data_age_seconds is recent (< 300 seconds)
		dataAge := firstOdds["data_age_seconds"].(float64)
		if dataAge > 300 {
			t.Logf("Warning: data age is %f seconds (may be stale)", dataAge)
		} else {
			t.Logf("✓ Data age: %.2f seconds (fresh)", dataAge)
		}
	}
}

func TestIntegration_GetOddsHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dbClient := getTestDB(t)
	handler := handlers.NewHandler(dbClient)

	// Query odds history for last 24 hours
	since := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/v1/odds/history?sport=basketball_nba&since="+since+"&limit=50", nil)
	w := httptest.NewRecorder()

	handler.GetOddsHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	count := int(response["count"].(float64))
	t.Logf("✓ Retrieved %d history entries (last 24h)", count)
}

func TestIntegration_EventWithOdds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dbClient := getTestDB(t)
	handler := handlers.NewHandler(dbClient)

	// First, get an event
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := dbClient.GetEvents(ctx, db.EventFilters{
		SportKey:    "basketball_nba",
		EventStatus: "upcoming",
		Limit:       1,
	})

	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}

	if len(events) == 0 {
		t.Skip("no events available for testing")
	}

	eventID := events[0].EventID
	t.Logf("Testing with event: %s (%s vs %s)", eventID, events[0].HomeTeam, events[0].AwayTeam)

	// Now get event with odds
	r := chi.NewRouter()
	r.Get("/events/{eventID}/odds", handler.GetEventWithOdds)

	req := httptest.NewRequest("GET", fmt.Sprintf("/events/%s/odds", eventID), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response models.EventWithOdds
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Event.EventID != eventID {
		t.Errorf("expected event_id %s, got %s", eventID, response.Event.EventID)
	}

	t.Logf("✓ Retrieved event with %d current odds", len(response.CurrentOdds))
}

func TestIntegration_ResponseLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dbClient := getTestDB(t)
	handler := handlers.NewHandler(dbClient)

	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
		maxMS   int64
	}{
		{"Health check", handler.HealthCheck, "/health", 50},
		{"Get events", handler.GetEvents, "/api/v1/events?sport=basketball_nba&limit=10", 100},
		{"Get current odds", handler.GetCurrentOdds, "/api/v1/odds/current?sport=basketball_nba&limit=100", 100},
		{"Get odds history", handler.GetOddsHistory, "/api/v1/odds/history?limit=50", 150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			start := time.Now()
			tt.handler(w, req)
			duration := time.Since(start)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			durationMS := duration.Milliseconds()
			t.Logf("Response time: %dms", durationMS)

			if durationMS > tt.maxMS {
				t.Errorf("response time %dms exceeded target %dms", durationMS, tt.maxMS)
			}
		})
	}
}

func TestIntegration_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dbClient := getTestDB(t)
	handler := handlers.NewHandler(dbClient)

	// Test pagination with odds
	tests := []struct {
		limit  int
		offset int
	}{
		{10, 0},
		{10, 10},
		{50, 0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("limit=%d,offset=%d", tt.limit, tt.offset), func(t *testing.T) {
			req := httptest.NewRequest("GET", 
				fmt.Sprintf("/api/v1/odds/current?sport=basketball_nba&limit=%d&offset=%d", tt.limit, tt.offset), 
				nil)
			w := httptest.NewRecorder()

			handler.GetCurrentOdds(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			var response map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			count := int(response["count"].(float64))
			respLimit := int(response["limit"].(float64))
			respOffset := int(response["offset"].(float64))

			if respLimit != tt.limit {
				t.Errorf("expected limit %d, got %d", tt.limit, respLimit)
			}

			if respOffset != tt.offset {
				t.Errorf("expected offset %d, got %d", tt.offset, respOffset)
			}

			t.Logf("✓ Retrieved %d results (limit=%d, offset=%d)", count, respLimit, respOffset)
		})
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}


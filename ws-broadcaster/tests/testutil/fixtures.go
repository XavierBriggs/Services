package testutil

import (
	"time"

	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/pkg/models"
)

// MockOddsUpdate creates a test odds update
func MockOddsUpdate(eventID, sportKey, marketKey, bookKey string) models.OddsUpdate {
	return models.OddsUpdate{
		EventID:            eventID,
		SportKey:           sportKey,
		MarketKey:          marketKey,
		BookKey:            bookKey,
		OutcomeName:        "Los Angeles Lakers",
		Price:              -110,
		Point:              nil,
		ImpliedProbability: 0.5238,
		NoVigProbability:   ptrFloat64(0.50),
		FairPrice:          ptrInt(-100),
		Edge:               ptrFloat64(0.024),
		SharpConsensus:     ptrFloat64(0.50),
		MarketType:         "two_way",
		NormalizedAt:       time.Now(),
		DataAge:            0.5,
	}
}

// MockSubscriptionFilter creates a test subscription filter
func MockSubscriptionFilter(sports, events, markets, books []string) models.SubscriptionFilter {
	return models.SubscriptionFilter{
		Sports:  sports,
		Events:  events,
		Markets: markets,
		Books:   books,
	}
}

// MockClientMessage creates a test client message
func MockClientMessage(msgType string, payload map[string]interface{}) models.ClientMessage {
	return models.ClientMessage{
		Type:    msgType,
		Payload: payload,
	}
}

// MockServerMessage creates a test server message
func MockServerMessage(msgType string, payload interface{}) models.ServerMessage {
	return models.ServerMessage{
		Type:      msgType,
		Payload:   payload,
		Timestamp: time.Now(),
	}
}

// Helper functions
func ptrFloat64(f float64) *float64 {
	return &f
}

func ptrInt(i int) *int {
	return &i
}


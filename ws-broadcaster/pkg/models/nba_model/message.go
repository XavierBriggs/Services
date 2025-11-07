package models

import "time"

// Message types for WebSocket communication
const (
	MessageTypeOddsUpdate      = "odds_update"
	MessageTypeSubscribe       = "subscribe"
	MessageTypeUnsubscribe     = "unsubscribe"
	MessageTypeHeartbeat       = "heartbeat"
	MessageTypeError           = "error"
	MessageTypeConnectionStats = "connection_stats"
)

// ClientMessage represents a message from client to server
type ClientMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// ServerMessage represents a message from server to client
type ServerMessage struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// OddsUpdate represents normalized odds data for broadcast
type OddsUpdate struct {
	// Raw odds data
	EventID          string    `json:"event_id"`
	SportKey         string    `json:"sport_key"`
	MarketKey        string    `json:"market_key"`
	BookKey          string    `json:"book_key"`
	OutcomeName      string    `json:"outcome_name"`
	Price            int       `json:"price"`
	Point            *float64  `json:"point"`
	VendorLastUpdate time.Time `json:"vendor_last_update"`
	ReceivedAt       time.Time `json:"received_at"`
	
	// Normalized values
	DecimalOdds        float64  `json:"decimal_odds"`
	ImpliedProbability float64  `json:"implied_probability"`
	NoVigProbability   *float64 `json:"novig_probability"`
	FairPrice          *int     `json:"fair_price"`
	Edge               *float64 `json:"edge"`
	SharpConsensus     *float64 `json:"sharp_consensus"`
	MarketType         string   `json:"market_type"`
	VigMethod          string   `json:"vig_method"`
	NormalizedAt       time.Time `json:"normalized_at"`
	ProcessingLatency  int64    `json:"processing_latency_ms"`
}

// SubscriptionFilter represents client subscription preferences
type SubscriptionFilter struct {
	Sports  []string `json:"sports,omitempty"`   // Filter by sport keys
	Events  []string `json:"events,omitempty"`   // Filter by event IDs
	Markets []string `json:"markets,omitempty"`  // Filter by market keys
	Books   []string `json:"books,omitempty"`    // Filter by book keys
}

// ConnectionStats represents connection statistics
type ConnectionStats struct {
	ClientID          string    `json:"client_id"`
	ConnectedAt       time.Time `json:"connected_at"`
	MessagesSent      int64     `json:"messages_sent"`
	MessagesReceived  int64     `json:"messages_received"`
	LastMessageAt     time.Time `json:"last_message_at"`
	BufferSize        int       `json:"buffer_size"`
	BufferUtilization float64   `json:"buffer_utilization"` // Percentage
}

// ErrorMessage represents an error message
type ErrorMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}


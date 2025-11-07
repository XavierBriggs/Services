package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/pkg/models"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512

	// Buffer size for outbound messages
	sendBufferSize = 256
)

// Client represents a WebSocket client connection
type Client struct {
	ID         string
	conn       *websocket.Conn
	Send       chan models.ServerMessage // Exported for hub access
	hub        Hub
	filter     models.SubscriptionFilter
	filterMu   sync.RWMutex
	connectedAt time.Time
	messagesSent     int64
	messagesReceived int64
	lastMessageAt    time.Time
	mu               sync.Mutex
}

// Hub defines the interface for the broadcast hub
type Hub interface {
	Unregister(client *Client)
}

// NewClient creates a new client instance
func NewClient(id string, conn *websocket.Conn, hub Hub) *Client {
	return &Client{
		ID:          id,
		conn:        conn,
		Send:        make(chan models.ServerMessage, sendBufferSize),
		hub:         hub,
		connectedAt: time.Now(),
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			var msg models.ClientMessage
			if err := c.conn.ReadJSON(&msg); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					fmt.Printf("client %s unexpected close: %v\n", c.ID, err)
				}
				return
			}

			c.updateReceived()
			c.handleClientMessage(msg)
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return

		case message, ok := <-c.Send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				fmt.Printf("client %s write error: %v\n", c.ID, err)
				return
			}

			c.updateSent()

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// TrySend sends a message to the client (non-blocking)
// Returns true if sent, false if buffer is full
func (c *Client) TrySend(msg models.ServerMessage) bool {
	select {
	case c.Send <- msg:
		return true
	default:
		// Buffer full - client is too slow
		return false
	}
}

// SetFilter updates the client's subscription filter
func (c *Client) SetFilter(filter models.SubscriptionFilter) {
	c.filterMu.Lock()
	defer c.filterMu.Unlock()
	c.filter = filter
}

// GetFilter returns the client's current filter
func (c *Client) GetFilter() models.SubscriptionFilter {
	c.filterMu.RLock()
	defer c.filterMu.RUnlock()
	return c.filter
}

// MatchesFilter checks if an odds update matches the client's filter
func (c *Client) MatchesFilter(update models.OddsUpdate) bool {
	c.filterMu.RLock()
	defer c.filterMu.RUnlock()

	// No filter = accept all
	if len(c.filter.Sports) == 0 && len(c.filter.Events) == 0 &&
		len(c.filter.Markets) == 0 && len(c.filter.Books) == 0 {
		return true
	}

	// Check sport filter
	if len(c.filter.Sports) > 0 && !contains(c.filter.Sports, update.SportKey) {
		return false
	}

	// Check event filter
	if len(c.filter.Events) > 0 && !contains(c.filter.Events, update.EventID) {
		return false
	}

	// Check market filter
	if len(c.filter.Markets) > 0 && !contains(c.filter.Markets, update.MarketKey) {
		return false
	}

	// Check book filter
	if len(c.filter.Books) > 0 && !contains(c.filter.Books, update.BookKey) {
		return false
	}

	return true
}

// GetStats returns connection statistics
func (c *Client) GetStats() models.ConnectionStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	bufferUtilization := float64(len(c.Send)) / float64(sendBufferSize) * 100.0

	return models.ConnectionStats{
		ClientID:          c.ID,
		ConnectedAt:       c.connectedAt,
		MessagesSent:      c.messagesSent,
		MessagesReceived:  c.messagesReceived,
		LastMessageAt:     c.lastMessageAt,
		BufferSize:        sendBufferSize,
		BufferUtilization: bufferUtilization,
	}
}

// handleClientMessage processes messages from the client
func (c *Client) handleClientMessage(msg models.ClientMessage) {
	switch msg.Type {
	case models.MessageTypeSubscribe:
		c.handleSubscribe(msg.Payload)
	case models.MessageTypeUnsubscribe:
		c.handleUnsubscribe()
	case models.MessageTypeHeartbeat:
		c.sendHeartbeat()
	default:
		c.sendError("unknown_message_type", fmt.Sprintf("unknown message type: %s", msg.Type))
	}
}

// handleSubscribe updates the client's filter based on subscription request
func (c *Client) handleSubscribe(payload map[string]interface{}) {
	filterJSON, err := json.Marshal(payload)
	if err != nil {
		c.sendError("invalid_filter", "failed to parse filter")
		return
	}

	var filter models.SubscriptionFilter
	if err := json.Unmarshal(filterJSON, &filter); err != nil {
		c.sendError("invalid_filter", "failed to parse filter")
		return
	}

	c.SetFilter(filter)
	fmt.Printf("client %s subscribed: sports=%v events=%v markets=%v books=%v\n",
		c.ID, filter.Sports, filter.Events, filter.Markets, filter.Books)
}

// handleUnsubscribe clears the client's filter
func (c *Client) handleUnsubscribe() {
	c.SetFilter(models.SubscriptionFilter{})
	fmt.Printf("client %s unsubscribed\n", c.ID)
}

// sendHeartbeat sends a heartbeat response
func (c *Client) sendHeartbeat() {
	stats := c.GetStats()
	c.TrySend(models.ServerMessage{
		Type:      models.MessageTypeHeartbeat,
		Payload:   stats,
		Timestamp: time.Now(),
	})
}

// sendError sends an error message to the client
func (c *Client) sendError(code, message string) {
	c.TrySend(models.ServerMessage{
		Type: models.MessageTypeError,
		Payload: models.ErrorMessage{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now(),
	})
}

// updateSent increments the sent message counter
func (c *Client) updateSent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messagesSent++
	c.lastMessageAt = time.Now()
}

// updateReceived increments the received message counter
func (c *Client) updateReceived() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messagesReceived++
	c.lastMessageAt = time.Now()
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}


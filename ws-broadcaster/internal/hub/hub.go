package hub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/client"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/pkg/models"
)

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients
	clients map[*client.Client]bool
	clientsMu sync.RWMutex

	// Inbound messages from stream consumer
	broadcast chan models.OddsUpdate

	// Register requests from clients
	register chan *client.Client

	// Unregister requests from clients
	unregister chan *client.Client

	// Metrics
	totalConnections int64
	totalMessages    int64
	metricsMu        sync.Mutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*client.Client]bool),
		broadcast:  make(chan models.OddsUpdate, 1000),
		register:   make(chan *client.Client),
		unregister: make(chan *client.Client),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) {
	fmt.Println("✓ Hub started")

	// Start metrics reporter
	go h.reportMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case update := <-h.broadcast:
			h.broadcastUpdate(update)
		}
	}
}

// Register adds a client to the hub
func (h *Hub) Register(client *client.Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *client.Client) {
	h.unregister <- client
}

// Broadcast sends an odds update to all matching clients
func (h *Hub) Broadcast(update models.OddsUpdate) {
	select {
	case h.broadcast <- update:
	default:
		// Broadcast buffer full - drop message
		fmt.Println("⚠️  Broadcast buffer full, dropping message")
	}
}

// registerClient adds a client to the active clients map
func (h *Hub) registerClient(c *client.Client) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	h.clients[c] = true
	h.incrementTotalConnections()

	fmt.Printf("client %s connected (total: %d)\n", c.ID, len(h.clients))
}

// unregisterClient removes a client from the active clients map
func (h *Hub) unregisterClient(c *client.Client) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.Send)
		fmt.Printf("client %s disconnected (total: %d)\n", c.ID, len(h.clients))
	}
}

// broadcastUpdate sends an update to all clients that match the filter
func (h *Hub) broadcastUpdate(update models.OddsUpdate) {
	h.clientsMu.RLock()
	clients := make([]*client.Client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.clientsMu.RUnlock()

	message := models.ServerMessage{
		Type:      models.MessageTypeOddsUpdate,
		Payload:   update,
		Timestamp: time.Now(),
	}

	sent := 0
	dropped := 0

	for _, c := range clients {
		// Check if client's filter matches this update
		if !c.MatchesFilter(update) {
			continue
		}

		// Try to send (non-blocking)
		if c.TrySend(message) {
			sent++
		} else {
			dropped++
			// Client buffer full - they're too slow, disconnect them
			fmt.Printf("⚠️  client %s buffer full, disconnecting\n", c.ID)
			go h.Unregister(c)
		}
	}

	if sent > 0 {
		h.incrementTotalMessages()
	}

	if dropped > 0 {
		fmt.Printf("⚠️  Dropped %d messages (slow clients)\n", dropped)
	}
}

// GetMetrics returns hub metrics
func (h *Hub) GetMetrics() map[string]interface{} {
	h.clientsMu.RLock()
	activeClients := len(h.clients)
	h.clientsMu.RUnlock()

	h.metricsMu.Lock()
	totalConnections := h.totalConnections
	totalMessages := h.totalMessages
	h.metricsMu.Unlock()

	return map[string]interface{}{
		"active_clients":     activeClients,
		"total_connections":  totalConnections,
		"total_messages":     totalMessages,
		"broadcast_capacity": cap(h.broadcast),
		"broadcast_usage":    len(h.broadcast),
	}
}

// GetClientCount returns the number of active clients
func (h *Hub) GetClientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// shutdown closes all client connections
func (h *Hub) shutdown() {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	fmt.Printf("🛑 Shutting down hub (%d active clients)\n", len(h.clients))

	for c := range h.clients {
		close(c.Send)
		delete(h.clients, c)
	}
}

// reportMetrics periodically reports hub metrics
func (h *Hub) reportMetrics(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := h.GetMetrics()
			fmt.Printf("📊 Hub Metrics: clients=%d total_connections=%d messages=%d\n",
				metrics["active_clients"],
				metrics["total_connections"],
				metrics["total_messages"])
		}
	}
}

// incrementTotalConnections safely increments the total connections counter
func (h *Hub) incrementTotalConnections() {
	h.metricsMu.Lock()
	defer h.metricsMu.Unlock()
	h.totalConnections++
}

// incrementTotalMessages safely increments the total messages counter
func (h *Hub) incrementTotalMessages() {
	h.metricsMu.Lock()
	defer h.metricsMu.Unlock()
	h.totalMessages++
}


package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/client"
	"github.com/XavierBriggs/fortuna/services/ws-broadcaster/internal/hub"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development
		// TODO: Restrict in production
		return true
	},
}

// Handler manages HTTP endpoints
type Handler struct {
	hub *hub.Hub
	ctx context.Context
}

// NewHandler creates a new handler instance
func NewHandler(h *hub.Hub, ctx context.Context) *Handler {
	return &Handler{
		hub: h,
		ctx: ctx,
	}
}

// HandleWebSocket upgrades HTTP connections to WebSocket
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("⚠️  WebSocket upgrade error: %v\n", err)
		return
	}

	// Create client
	clientID := uuid.New().String()
	c := client.NewClient(clientID, conn, h.hub)

	// Register with hub
	h.hub.Register(c)

	// Start client pumps (use handler context, not request context)
	go c.WritePump(h.ctx)
	go c.ReadPump(h.ctx)

	fmt.Printf("✓ WebSocket connection established: %s\n", clientID)
}

// HandleHealth returns service health
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":         "healthy",
		"service":        "ws-broadcaster",
		"active_clients": h.hub.GetClientCount(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// HandleMetrics returns hub metrics
func (h *Handler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.hub.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}


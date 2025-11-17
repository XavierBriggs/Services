package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// TalosClient handles HTTP communication with Talos Bot Manager
type TalosClient struct {
	baseURL    string
	httpClient *http.Client
}

// TalosBetRequest is the request format for Talos Bot Manager
type TalosBetRequest struct {
	Book      string `json:"book"`
	Team1     string `json:"team1"`     // Short name (lowercase): "lakers", "bulls"
	Team2     string `json:"team2"`     // Short name (lowercase): "heat", "knicks"
	BetTeam   string `json:"bet_team"`  // Short name (lowercase): "lakers"
	BetType   string `json:"bet_type"`  // "moneyline", "spread", "total_over", "total_under"
	BetPeriod string `json:"bet_period"` // "game", "1st_half", "1st_quarter"
	BetAmount string `json:"bet_amount"` // e.g., "96.25"
	BetOdds   string `json:"bet_odds,omitempty"` // e.g., "-110"
	Sport     string `json:"sport,omitempty"`    // "nba" or "basketball/nba"
	RequestID string `json:"request_id,omitempty"`
}

// TalosBetResponse is the response from Talos Bot Manager
type TalosBetResponse struct {
	Status     string                 `json:"status"`
	Message    string                 `json:"message"`
	RequestID  string                 `json:"request_id,omitempty"`
	BetDetails map[string]interface{} `json:"bet_details,omitempty"`
	Book       string                 `json:"book,omitempty"`
}

// TalosHealthResponse is the health check response
type TalosHealthResponse struct {
	Status string                    `json:"status"`
	Bots   map[string]TalosBotStatus `json:"bots"`
}

// TalosBotStatus represents the status of a book bot
type TalosBotStatus struct {
	Registered bool   `json:"registered"`
	Healthy    bool   `json:"healthy"`
	URL        string `json:"url"`
}

// NewTalosClient creates a new Talos client
func NewTalosClient(baseURL string, httpClient *http.Client) *TalosClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 120 * time.Second, // Bet placement can take time
		}
	}
	return &TalosClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// PlaceBet sends a bet request to Talos Bot Manager
func (c *TalosClient) PlaceBet(req TalosBetRequest) (*TalosBetResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log the request being sent to Talos
	log.Printf("[TalosClient] Sending bet request to %s/bet: %s", c.baseURL, string(jsonData))

	httpReq, err := http.NewRequest("POST", c.baseURL+"/bet", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Log the response from Talos
	log.Printf("[TalosClient] Response from Talos (status %d): %s", resp.StatusCode, string(body))

	var talosResp TalosBetResponse
	if err := json.Unmarshal(body, &talosResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("talos error (status %d): %s", resp.StatusCode, talosResp.Message)
	}

	return &talosResp, nil
}

// BotHealthResponse represents the health response from an individual bot
type BotHealthResponse struct {
	Status         string                 `json:"status"`
	LoggedIn       bool                   `json:"logged_in"`
	Balance        *string                `json:"balance"`
	SessionDuration map[string]interface{} `json:"session_duration"`
}

// CheckHealth checks if a specific book bot is healthy
func (c *TalosClient) CheckHealth(bookKey string) (bool, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, nil
	}

	var healthResp TalosHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		return false, err
	}

	botStatus, exists := healthResp.Bots[bookKey]
	if !exists {
		return false, nil
	}

	return botStatus.Healthy, nil
}

// GetBotHealthDirect gets health and balance directly from a bot's /health endpoint
func (c *TalosClient) GetBotHealthDirect(botURL string) (*BotHealthResponse, error) {
	resp, err := c.httpClient.Get(botURL + "/health")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &BotHealthResponse{
			Status:   "error",
			LoggedIn: false,
		}, nil
	}

	var healthResp BotHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		return nil, err
	}

	return &healthResp, nil
}

// ListBots returns a list of all registered bots
func (c *TalosClient) ListBots() ([]string, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/bots")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Bots []string `json:"bots"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Bots, nil
}


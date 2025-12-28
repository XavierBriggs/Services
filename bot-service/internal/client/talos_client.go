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
// Aligned with base_bot/models.py BetRequest
type TalosBetRequest struct {
	Book          string `json:"book"`
	Team1         string `json:"team1"`                     // Full name - "Los Angeles Lakers"
	Team1Mascot   string `json:"team1_mascot,omitempty"`    // Mascot (lowercase) - "lakers"
	Team2         string `json:"team2"`                     // Full name - "San Antonio Spurs"
	Team2Mascot   string `json:"team2_mascot,omitempty"`    // Mascot (lowercase) - "spurs"
	BetTeam       string `json:"bet_team"`                  // Full name of team to bet on
	BetTeamMascot string `json:"bet_team_mascot,omitempty"` // Mascot (lowercase)
	BetType       string `json:"bet_type"`                  // "moneyline", "spread", "total_over", "total_under"
	BetPeriod     string `json:"bet_period"`                // "game", "1st_half", "1st_quarter"
	BetAmount     string `json:"bet_amount"`                // e.g., "96.25"
	BetOdds       string `json:"bet_odds,omitempty"`        // e.g., "-110" or "+150"
	Sport         string `json:"sport,omitempty"`           // "nba" or "basketball/nba"
	RequestID     string `json:"request_id,omitempty"`
	EventDate     string `json:"event_date,omitempty"` // YYYY-MM-DD format for game key consistency
	GameKey       string `json:"game_key,omitempty"`   // Pre-computed game key (optional)
	League        string `json:"league,omitempty"`     // League identifier (optional)
}

// TalosBetResponse is the response from Talos Bot Manager
// Note: Bots return ok (bool) instead of status (string), plus error_code/error_message
type TalosBetResponse struct {
	// New format from bots (ok: true/false)
	OK           bool   `json:"ok"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	ToWin        string `json:"to_win,omitempty"`
	Balance      string `json:"balance,omitempty"`
	GameKey      string `json:"game_key,omitempty"`

	// Legacy fields (for backward compatibility)
	Status     string                 `json:"status,omitempty"`
	Message    string                 `json:"message,omitempty"`
	ErrorType  string                 `json:"error_type,omitempty"`
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

// AsyncBetResponse is the response from enqueueing an async bet
type AsyncBetResponse struct {
	Accepted  bool   `json:"accepted"`
	JobID     string `json:"job_id"`
	RequestID string `json:"request_id,omitempty"`
	Book      string `json:"book,omitempty"`
}

// BetStatusResponse is the response from polling bet status
type BetStatusResponse struct {
	JobID        string            `json:"job_id"`
	Status       string            `json:"status"` // "pending", "running", "done", "error"
	Result       *TalosBetResponse `json:"result,omitempty"`
	ErrorMessage string            `json:"error_message,omitempty"`
	Book         string            `json:"book,omitempty"`
}

// PlaceBet sends a bet request to Talos Bot Manager (sync - waits for result)
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

// PlaceBetAsync enqueues a bet and returns immediately with a job ID
func (c *TalosClient) PlaceBetAsync(req TalosBetRequest) (*AsyncBetResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("[TalosClient] Sending async bet request to %s/bet-async: %s", c.baseURL, string(jsonData))

	httpReq, err := http.NewRequest("POST", c.baseURL+"/bet-async", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Short timeout for async enqueue (should return quickly)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("[TalosClient] Async response (status %d): %s", resp.StatusCode, string(body))

	var asyncResp AsyncBetResponse
	if err := json.Unmarshal(body, &asyncResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("talos error (status %d): failed to enqueue bet", resp.StatusCode)
	}

	return &asyncResp, nil
}

// GetBetStatus polls the status of an async bet job
func (c *TalosClient) GetBetStatus(bookKey string, jobID string) (*BetStatusResponse, error) {
	url := fmt.Sprintf("%s/bet-status/%s/%s", c.baseURL, bookKey, jobID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var statusResp BetStatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("talos error (status %d)", resp.StatusCode)
	}

	return &statusResp, nil
}

// PlaceBetAsyncWithPolling enqueues a bet and polls until completion (defacto method)
// This is the recommended approach for bet placement
func (c *TalosClient) PlaceBetAsyncWithPolling(req TalosBetRequest, maxWaitSeconds int) (*TalosBetResponse, error) {
	// Step 1: Enqueue the bet
	asyncResp, err := c.PlaceBetAsync(req)
	if err != nil {
		return nil, fmt.Errorf("failed to enqueue bet: %w", err)
	}

	if !asyncResp.Accepted {
		return nil, fmt.Errorf("bet not accepted")
	}

	log.Printf("[TalosClient] Bet enqueued with job_id: %s, polling for result...", asyncResp.JobID)

	// Step 2: Poll for completion
	pollInterval := 500 * time.Millisecond
	maxPolls := maxWaitSeconds * 2 // 500ms interval

	for i := 0; i < maxPolls; i++ {
		time.Sleep(pollInterval)

		statusResp, err := c.GetBetStatus(req.Book, asyncResp.JobID)
		if err != nil {
			log.Printf("[TalosClient] Poll %d: error getting status: %v", i+1, err)
			continue
		}

		switch statusResp.Status {
		case "done":
			log.Printf("[TalosClient] Bet completed successfully (job_id: %s)", asyncResp.JobID)
			if statusResp.Result != nil {
				return statusResp.Result, nil
			}
			// If result is nil but status is done, construct a success response
			return &TalosBetResponse{
				OK:        true,
				RequestID: req.RequestID,
				Book:      req.Book,
			}, nil

		case "error":
			log.Printf("[TalosClient] Bet failed (job_id: %s): %s", asyncResp.JobID, statusResp.ErrorMessage)
			return &TalosBetResponse{
				OK:           false,
				ErrorCode:    "bet_error",
				ErrorMessage: statusResp.ErrorMessage,
				RequestID:    req.RequestID,
				Book:         req.Book,
			}, nil

		case "pending", "running":
			// Still processing, continue polling
			if i%10 == 0 { // Log every 5 seconds
				log.Printf("[TalosClient] Poll %d: bet still %s (job_id: %s)", i+1, statusResp.Status, asyncResp.JobID)
			}
		}
	}

	// Timeout
	return &TalosBetResponse{
		OK:           false,
		ErrorCode:    "timeout",
		ErrorMessage: fmt.Sprintf("bet timed out after %d seconds", maxWaitSeconds),
		RequestID:    req.RequestID,
		Book:         req.Book,
	}, nil
}

// BotHealthResponse represents the health response from an individual bot
type BotHealthResponse struct {
	Status          string                 `json:"status"`
	LoggedIn        bool                   `json:"logged_in"`
	Balance         *string                `json:"balance"`
	SessionDuration map[string]interface{} `json:"session_duration"`
}

// CheckHealth checks if a specific book bot is healthy
func (c *TalosClient) CheckHealth(bookKey string) (bool, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/health")
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

// GetBotHealthDirect gets health and balance directly from a bot's /api/health endpoint
func (c *TalosClient) GetBotHealthDirect(botURL string) (*BotHealthResponse, error) {
	resp, err := c.httpClient.Get(botURL + "/api/health")
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

// OpenGamePageRequest is the request format for warming a game page
type OpenGamePageRequest struct {
	Team1       string   `json:"team1"`
	Team2       string   `json:"team2"`
	Sport       string   `json:"sport,omitempty"`
	League      string   `json:"league,omitempty"`
	BetPeriod   string   `json:"bet_period,omitempty"`
	EventDate   string   `json:"event_date,omitempty"` // YYYY-MM-DD format
	TargetBooks []string `json:"target_books,omitempty"`
}

// CloseGamePageRequest is the request format for closing a game page
type CloseGamePageRequest struct {
	Book        string   `json:"book"`
	GameKey     string   `json:"game_key"`
	TargetBooks []string `json:"target_books,omitempty"`
}

// PageActionResponse is the response from open/close game page endpoints
type PageActionResponse struct {
	AllOK   bool                   `json:"all_ok"`
	AnyOK   bool                   `json:"any_ok"`
	Results map[string]interface{} `json:"results"`
}

// OpenGamePage warms a game page across all supported bots
func (c *TalosClient) OpenGamePage(req OpenGamePageRequest) (*PageActionResponse, error) {
	// Set defaults
	if req.Sport == "" {
		req.Sport = "nba"
	}
	if req.BetPeriod == "" {
		req.BetPeriod = "game"
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("[TalosClient] Opening game page: %s vs %s (date: %s)", req.Team1, req.Team2, req.EventDate)

	httpReq, err := http.NewRequest("POST", c.baseURL+"/open-game-page", bytes.NewBuffer(jsonData))
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

	var pageResp PageActionResponse
	if err := json.Unmarshal(body, &pageResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	log.Printf("[TalosClient] Open game page response (status %d): all_ok=%v, any_ok=%v",
		resp.StatusCode, pageResp.AllOK, pageResp.AnyOK)

	return &pageResp, nil
}

// CloseGamePage closes a game page for a specific book or all books
func (c *TalosClient) CloseGamePage(req CloseGamePageRequest) (*PageActionResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("[TalosClient] Closing game page: book=%s, game_key=%s", req.Book, req.GameKey)

	httpReq, err := http.NewRequest("POST", c.baseURL+"/close-game-page", bytes.NewBuffer(jsonData))
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

	var pageResp PageActionResponse
	if err := json.Unmarshal(body, &pageResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	log.Printf("[TalosClient] Close game page response (status %d): all_ok=%v", resp.StatusCode, pageResp.AllOK)

	return &pageResp, nil
}

// CloseGamePageAllBooks closes a game page across all supported books
func (c *TalosClient) CloseGamePageAllBooks(gameKey string, books []string) error {
	for _, book := range books {
		req := CloseGamePageRequest{
			Book:    book,
			GameKey: gameKey,
		}
		_, err := c.CloseGamePage(req)
		if err != nil {
			log.Printf("[TalosClient] Warning: Failed to close page for %s: %v", book, err)
			// Continue with other books even if one fails
		}
	}
	return nil
}

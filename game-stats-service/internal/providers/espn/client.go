package espn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	BaseURL = "https://site.api.espn.com/apis/site/v2/sports"
)

// Client handles ESPN API requests
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// New creates a new ESPN API client
func New() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		userAgent: "Mozilla/5.0 (compatible; FortunaBot/1.0)",
	}
}

// FetchScoreboard fetches games for a specific sport
// If date is zero, fetches whatever ESPN considers "today"
func (c *Client) FetchScoreboard(ctx context.Context, sportPath string, date time.Time) (map[string]interface{}, error) {
	var url string
	if date.IsZero() {
		// No date specified - get ESPN's "today" (includes games within ~24 hours)
		url = fmt.Sprintf("%s/%s/scoreboard", BaseURL, sportPath)
	} else {
		dateStr := date.Format("20060102")
		url = fmt.Sprintf("%s/%s/scoreboard?dates=%s", BaseURL, sportPath, dateStr)
	}

	return c.fetch(ctx, url)
}

// FetchGameSummary fetches detailed game summary with box scores
func (c *Client) FetchGameSummary(ctx context.Context, sportPath string, gameID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s/summary?event=%s", BaseURL, sportPath, gameID)

	return c.fetch(ctx, url)
}

// fetch makes an HTTP GET request and returns parsed JSON
func (c *Client) fetch(ctx context.Context, url string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ESPN API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return result, nil
}


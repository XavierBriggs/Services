package notifier

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/XavierBriggs/fortuna/services/alert-service/pkg/models"
)

// EST timezone
var estLocation *time.Location

func init() {
	var err error
	estLocation, err = time.LoadLocation("America/New_York")
	if err != nil {
		// Fallback to fixed offset if timezone data not available
		estLocation = time.FixedZone("EST", -5*60*60)
	}
}

// SlackNotifier sends alerts to Slack via webhook
type SlackNotifier struct {
	webhookURL     string
	httpClient     *http.Client
	frontendURL    string
	defaultStake   int
	alexandriaDB   *sql.DB
	booksWhitelist []string
	eventNameCache map[string]string // In-memory cache for event names
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	defaultStake := 100 // $100 default
	if stakeStr := os.Getenv("DEFAULT_STAKE"); stakeStr != "" {
		fmt.Sscanf(stakeStr, "%d", &defaultStake)
	}

	// Parse books whitelist from env (comma-separated)
	var booksWhitelist []string
	if booksStr := os.Getenv("ALERT_BOOKS_WHITELIST"); booksStr != "" {
		for _, book := range strings.Split(booksStr, ",") {
			book = strings.TrimSpace(strings.ToLower(book))
			if book != "" {
				booksWhitelist = append(booksWhitelist, book)
			}
		}
	}

	return &SlackNotifier{
		webhookURL:     webhookURL,
		frontendURL:    frontendURL,
		defaultStake:   defaultStake,
		booksWhitelist: booksWhitelist,
		eventNameCache: make(map[string]string),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetAlexandriaDB sets the Alexandria database connection for event lookups
func (s *SlackNotifier) SetAlexandriaDB(db *sql.DB) {
	s.alexandriaDB = db
}

// ShouldSendAlert checks if an alert should be sent based on book filtering
func (s *SlackNotifier) ShouldSendAlert(opp models.Opportunity) (bool, string) {
	// If no whitelist configured, allow all
	if len(s.booksWhitelist) == 0 {
		return true, ""
	}

	// Check if any leg's book is in the whitelist
	for _, leg := range opp.Legs {
		bookKey := strings.ToLower(leg.BookKey)
		for _, allowed := range s.booksWhitelist {
			if bookKey == allowed || strings.HasPrefix(bookKey, allowed) {
				return true, ""
			}
		}
	}

	// No matching book found
	bookList := make([]string, len(opp.Legs))
	for i, leg := range opp.Legs {
		bookList[i] = leg.BookKey
	}
	return false, fmt.Sprintf("no allowed books (got: %s)", strings.Join(bookList, ", "))
}

// SendAlert sends an opportunity alert to Slack with Block Kit format
func (s *SlackNotifier) SendAlert(ctx context.Context, opp models.Opportunity) error {
	// Check book filter first
	shouldSend, reason := s.ShouldSendAlert(opp)
	if !shouldSend {
		fmt.Printf("⊘ Filtered by book whitelist: opportunity %d - %s\n", opp.ID, reason)
		return nil
	}

	startTime := time.Now()

	// Build Block Kit payload
	payload := s.buildBlockKitPayload(ctx, opp)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	// Send POST request
	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack webhook returned status %d", resp.StatusCode)
	}

	latency := time.Since(startTime).Milliseconds()
	fmt.Printf("✓ Slack alert sent: opportunity_id=%d latency=%dms\n", opp.ID, latency)

	return nil
}

// getEventName fetches the event name from cache or Alexandria, returns formatted game name
func (s *SlackNotifier) getEventName(ctx context.Context, eventID string) string {
	// Check cache first (zero latency)
	if name, ok := s.eventNameCache[eventID]; ok {
		return name
	}

	if s.alexandriaDB == nil {
		return eventID // Fallback to event ID if no DB connection
	}

	// Use a short timeout to avoid blocking alerts
	queryCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	query := `
		SELECT home_team, away_team 
		FROM events 
		WHERE event_id = $1
	`

	var homeTeam, awayTeam string
	err := s.alexandriaDB.QueryRowContext(queryCtx, query, eventID).Scan(&homeTeam, &awayTeam)
	if err != nil {
		// Cache the miss to avoid repeated queries
		s.eventNameCache[eventID] = eventID
		return eventID
	}

	// Cache and return
	name := fmt.Sprintf("%s @ %s", awayTeam, homeTeam)
	s.eventNameCache[eventID] = name
	return name
}

// buildBlockKitPayload builds a Block Kit formatted Slack message with Place Bet button
func (s *SlackNotifier) buildBlockKitPayload(ctx context.Context, opp models.Opportunity) map[string]interface{} {
	// Get event name
	eventName := s.getEventName(ctx, opp.EventID)

	// Build fallback text for notifications
	emoji := s.getEmojiForType(opp.OpportunityType)
	fallbackText := fmt.Sprintf("%s %s DETECTED | Edge: %.2f%%", emoji, strings.ToUpper(opp.OpportunityType), opp.EdgePercent)

	// Build blocks
	blocks := []interface{}{}

	// Header
	header := map[string]interface{}{
		"type": "header",
		"text": map[string]interface{}{
			"type":  "plain_text",
			"text":  fmt.Sprintf("%s %s DETECTED | %.2f%% Edge", emoji, strings.ToUpper(opp.OpportunityType), opp.EdgePercent),
			"emoji": true,
		},
	}
	blocks = append(blocks, header)

	// Event and market fields
	fields := []interface{}{
		map[string]interface{}{
			"type": "mrkdwn",
			"text": fmt.Sprintf("*Event:*\n%s", eventName),
		},
		map[string]interface{}{
			"type": "mrkdwn",
			"text": fmt.Sprintf("*Market:*\n%s", s.formatMarketName(opp.MarketKey)),
		},
	}

	// Add first leg details
	if len(opp.Legs) > 0 {
		leg := opp.Legs[0]
		lineText := fmt.Sprintf("%s @ %s", leg.OutcomeName, s.formatOdds(leg.Price))
		if leg.Point != nil {
			lineText = fmt.Sprintf("%s (%.1f) @ %s", leg.OutcomeName, *leg.Point, s.formatOdds(leg.Price))
		}

		fields = append(fields,
			map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Book:*\n%s", s.bookDisplayName(leg.BookKey)),
			},
			map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Line:*\n%s", lineText),
			},
		)
	}

	section := map[string]interface{}{
		"type":   "section",
		"fields": fields,
	}
	blocks = append(blocks, section)

	// Context block with age and fair price
	ageBadge := s.getAgeBadge(opp.DataAgeSeconds)
	contextText := fmt.Sprintf("%s Age: %ds", ageBadge, opp.DataAgeSeconds)
	if opp.FairPrice != nil {
		contextText += fmt.Sprintf(" | Fair: %s", s.formatOdds(*opp.FairPrice))
	}
	contextText += fmt.Sprintf(" | Suggested: $%d", s.defaultStake)

	contextBlock := map[string]interface{}{
		"type": "context",
		"elements": []interface{}{
			map[string]interface{}{
				"type": "mrkdwn",
				"text": contextText,
			},
		},
	}
	blocks = append(blocks, contextBlock)

	// Actions block with Place Bet buttons for each leg
	actionElements := []interface{}{}

	for i, leg := range opp.Legs {
		// Build button value JSON
		buttonValue := map[string]interface{}{
			"opp_id":        opp.ID,
			"book":          leg.BookKey,
			"default_stake": s.defaultStake,
			"leg_idx":       i,
		}
		buttonValueJSON, _ := json.Marshal(buttonValue)

		buttonText := "Place Bet"
		if len(opp.Legs) > 1 {
			buttonText = fmt.Sprintf("Bet %s", s.bookDisplayName(leg.BookKey))
		}

		button := map[string]interface{}{
			"type": "button",
			"text": map[string]interface{}{
				"type":  "plain_text",
				"text":  buttonText,
				"emoji": true,
			},
			"style":     "primary",
			"action_id": fmt.Sprintf("place_bet_%d", i),
			"value":     string(buttonValueJSON),
		}
		actionElements = append(actionElements, button)
	}

	// Add View Details link button
	viewButton := map[string]interface{}{
		"type": "button",
		"text": map[string]interface{}{
			"type":  "plain_text",
			"text":  "View Details",
			"emoji": true,
		},
		"action_id": "view_details",
		"url":       fmt.Sprintf("%s/opportunities/%d", s.frontendURL, opp.ID),
	}
	actionElements = append(actionElements, viewButton)

	actionsBlock := map[string]interface{}{
		"type":     "actions",
		"elements": actionElements,
	}
	blocks = append(blocks, actionsBlock)

	// Footer context with ID and timestamp in EST
	detectedAtEST := opp.DetectedAt.In(estLocation)
	footerBlock := map[string]interface{}{
		"type": "context",
		"elements": []interface{}{
			map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("_ID: %d | Detected: %s EST_", opp.ID, detectedAtEST.Format("3:04:05 PM")),
			},
		},
	}
	blocks = append(blocks, footerBlock)

	// Return full payload
	return map[string]interface{}{
		"text":   fallbackText,
		"blocks": blocks,
	}
}

// formatMarketName returns a display-friendly market name
func (s *SlackNotifier) formatMarketName(marketKey string) string {
	names := map[string]string{
		"h2h":        "Moneyline",
		"spreads":    "Spread",
		"totals":     "Total",
		"h2h_h1":     "1H Moneyline",
		"spreads_h1": "1H Spread",
		"totals_h1":  "1H Total",
	}
	if name, ok := names[strings.ToLower(marketKey)]; ok {
		return name
	}
	return marketKey
}

// formatMessage formats an opportunity as a plain text Slack message (legacy fallback)
func (s *SlackNotifier) formatMessage(opp models.Opportunity) string {
	var sb strings.Builder

	// Title with emoji based on opportunity type
	emoji := s.getEmojiForType(opp.OpportunityType)
	sb.WriteString(fmt.Sprintf("%s *%s DETECTED* | Edge: %.2f%%\n\n",
		emoji, strings.ToUpper(opp.OpportunityType), opp.EdgePercent))

	// Event details
	sb.WriteString(fmt.Sprintf("*Event:* %s\n", opp.EventID))
	sb.WriteString(fmt.Sprintf("*Market:* %s\n", opp.MarketKey))

	// Age badge
	ageBadge := s.getAgeBadge(opp.DataAgeSeconds)
	sb.WriteString(fmt.Sprintf("*Age:* %s %ds\n\n", ageBadge, opp.DataAgeSeconds))

	// Legs
	for i, leg := range opp.Legs {
		sb.WriteString(fmt.Sprintf("*Leg %d:* %s | %s @ %s",
			i+1, leg.BookKey, leg.OutcomeName, s.formatOdds(leg.Price)))

		if leg.Point != nil {
			sb.WriteString(fmt.Sprintf(" (%.1f)", *leg.Point))
		}

		if leg.LegEdgePercent != nil {
			sb.WriteString(fmt.Sprintf(" | Edge: %.2f%%", *leg.LegEdgePercent))
		}

		sb.WriteString("\n")
	}

	// Fair price (if available)
	if opp.FairPrice != nil {
		sb.WriteString(fmt.Sprintf("\n*Fair Price:* %s", s.formatOdds(*opp.FairPrice)))
	}

	// Link to opportunities page
	sb.WriteString(fmt.Sprintf("\n\n<%s/opportunities|View Opportunities>", s.frontendURL))

	// Metadata in EST
	detectedAtEST := opp.DetectedAt.In(estLocation)
	sb.WriteString(fmt.Sprintf("\n\n_Detected: %s EST | ID: %d_",
		detectedAtEST.Format("3:04:05 PM"), opp.ID))

	return sb.String()
}

// getEmojiForType returns an emoji for the opportunity type
func (s *SlackNotifier) getEmojiForType(oppType string) string {
	switch oppType {
	case "edge":
		return "💰"
	case "middle":
		return "🎯"
	case "scalp":
		return "⚡"
	default:
		return "📊"
	}
}

// getAgeBadge returns an age badge with emoji
func (s *SlackNotifier) getAgeBadge(ageSeconds int) string {
	if ageSeconds < 5 {
		return "🟢" // Green - fresh
	} else if ageSeconds < 10 {
		return "🟡" // Yellow - moderate
	} else {
		return "🔴" // Red - stale
	}
}

// formatOdds formats American odds with sign
func (s *SlackNotifier) formatOdds(americanOdds int) string {
	if americanOdds > 0 {
		return fmt.Sprintf("+%d", americanOdds)
	}
	return fmt.Sprintf("%d", americanOdds)
}

// bookDisplayName returns a display-friendly book name
func (s *SlackNotifier) bookDisplayName(bookKey string) string {
	names := map[string]string{
		"betonline":   "BetOnline",
		"betonlineag": "BetOnline",
		"betus":       "BetUS",
		"bovada":      "Bovada",
		"fanduel":     "FanDuel",
		"draftkings":  "DraftKings",
		"betmgm":      "BetMGM",
		"caesars":     "Caesars",
		"pinnacle":    "Pinnacle",
		"pointsbet":   "PointsBet",
		"betrivers":   "BetRivers",
		"espnbet":     "ESPN BET",
		"ballybet":    "Bally Bet",
		"hardrockbet": "Hard Rock",
		"betway":      "Betway",
	}
	if name, ok := names[strings.ToLower(bookKey)]; ok {
		return name
	}
	return bookKey
}

// SendStartupNotification sends a startup notification to Slack
func (s *SlackNotifier) SendStartupNotification(ctx context.Context) error {
	if s.webhookURL == "" {
		return fmt.Errorf("no webhook URL configured")
	}

	booksInfo := "All books"
	if len(s.booksWhitelist) > 0 {
		booksInfo = strings.Join(s.booksWhitelist, ", ")
	}

	nowEST := time.Now().In(estLocation)
	message := fmt.Sprintf(
		"🚀 *Fortuna Alert System Active*\n\n"+
			"✅ Alert service is now monitoring opportunities\n"+
			"📊 Configured thresholds:\n"+
			"   • Min Edge: 1.0%%\n"+
			"   • Max Data Age: 10s\n"+
			"   • Rate Limit: 10 alerts/min\n"+
			"   • Books: %s\n\n"+
			"💡 Use `/fortuna filters` to customize your preferences\n\n"+
			"_Started: %s EST_",
		booksInfo,
		nowEST.Format("2006-01-02 3:04:05 PM"),
	)

	payload := map[string]interface{}{
		"text": message,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// SendBatchAlerts sends multiple alerts
func (s *SlackNotifier) SendBatchAlerts(ctx context.Context, opportunities []models.Opportunity) error {
	for _, opp := range opportunities {
		if err := s.SendAlert(ctx, opp); err != nil {
			return fmt.Errorf("failed to send alert for opportunity %d: %w", opp.ID, err)
		}

		// Small delay between messages to avoid rate limits
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

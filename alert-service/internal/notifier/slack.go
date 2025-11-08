package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/XavierBriggs/fortuna/services/alert-service/pkg/models"
)

// SlackNotifier sends alerts to Slack via webhook
type SlackNotifier struct {
	webhookURL string
	httpClient *http.Client
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendAlert sends an opportunity alert to Slack
func (s *SlackNotifier) SendAlert(ctx context.Context, opp models.Opportunity) error {
	startTime := time.Now()
	
	// Format the Slack message
	message := s.formatMessage(opp)

	// Create Slack webhook payload
	payload := map[string]interface{}{
		"text": message,
	}

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

// formatMessage formats an opportunity as a Slack message
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
	sb.WriteString(fmt.Sprintf("\n\n<http://localhost:3000/opportunities|View Opportunities>"))

	// Metadata
	sb.WriteString(fmt.Sprintf("\n\n_Detected: %s | ID: %d_",
		opp.DetectedAt.Format("15:04:05"), opp.ID))

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

// SendStartupNotification sends a startup notification to Slack
func (s *SlackNotifier) SendStartupNotification(ctx context.Context) error {
	if s.webhookURL == "" {
		return fmt.Errorf("no webhook URL configured")
	}

	message := fmt.Sprintf(
		"🚀 *Fortuna Alert System Active*\n\n" +
		"✅ Alert service is now monitoring opportunities\n" +
		"📊 Configured thresholds:\n" +
		"   • Min Edge: 1.0%%\n" +
		"   • Max Data Age: 10s\n" +
		"   • Rate Limit: 10 alerts/min\n\n" +
		"_Started: %s_",
		time.Now().Format("2006-01-02 15:04:05 MST"),
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


package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Event types for structured logging
const (
	EventBetButtonClicked       = "bet_button_clicked"
	EventBetModalSubmitReceived = "bet_modal_submit_received"
	EventBetBlockedByFilter     = "bet_blocked_by_filter"
	EventBetDedupBlocked        = "bet_dedup_blocked"
	EventBetRequestAccepted     = "bet_request_accepted"
	EventBetRequestFailed       = "bet_request_failed"
	EventBetCompleted           = "bet_completed"
	EventBetFailed              = "bet_failed"
	EventFiltersUpdated         = "filters_updated"
	EventFilterModalOpened      = "filter_modal_opened"
	EventAlertSent              = "alert_sent"
	EventAlertFiltered          = "alert_filtered"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp      string                 `json:"timestamp"`
	Service        string                 `json:"service"`
	Event          string                 `json:"event"`
	RequestID      string                 `json:"request_id,omitempty"`
	SlackUserID    string                 `json:"slack_user_id,omitempty"`
	SlackChannelID string                 `json:"slack_channel_id,omitempty"`
	OppID          *int64                 `json:"opp_id,omitempty"`
	Book           string                 `json:"book,omitempty"`
	EdgePercent    *float64               `json:"edge_percent,omitempty"`
	MinEdgePercent *float64               `json:"min_edge_percent,omitempty"`
	BooksWhitelist []string               `json:"books_whitelist,omitempty"`
	StakeCents     *int                   `json:"stake_cents,omitempty"`
	BetIntentID    string                 `json:"bet_intent_id,omitempty"`
	Status         string                 `json:"status,omitempty"`
	Error          string                 `json:"error,omitempty"`
	ErrorType      string                 `json:"error_type,omitempty"`
	Reason         string                 `json:"reason,omitempty"`
	LatencyMs      *int64                 `json:"latency_ms,omitempty"`
	BetAPIResult   map[string]interface{} `json:"bet_api_result,omitempty"`
	Extra          map[string]interface{} `json:"extra,omitempty"`
}

// Logger handles structured logging for the Slack Gateway
type Logger struct {
	serviceName string
}

// New creates a new logger
func New(serviceName string) *Logger {
	return &Logger{
		serviceName: serviceName,
	}
}

// Log writes a structured log entry
func (l *Logger) Log(entry LogEntry) {
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	entry.Service = l.serviceName

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal log entry: %v\n", err)
		return
	}

	fmt.Println(string(data))
}

// Info logs an informational message
func (l *Logger) Info(event string, fields map[string]interface{}) {
	entry := LogEntry{
		Event: event,
		Extra: fields,
	}
	l.Log(entry)
}

// Error logs an error message
func (l *Logger) Error(event string, err error, fields map[string]interface{}) {
	entry := LogEntry{
		Event: event,
		Error: err.Error(),
		Extra: fields,
	}
	l.Log(entry)
}

// LogBetButtonClicked logs when a user clicks Place Bet button
func (l *Logger) LogBetButtonClicked(requestID, slackUserID, channelID string, oppID int64, book string, betIntentID string) {
	l.Log(LogEntry{
		Event:          EventBetButtonClicked,
		RequestID:      requestID,
		SlackUserID:    slackUserID,
		SlackChannelID: channelID,
		OppID:          &oppID,
		Book:           book,
		BetIntentID:    betIntentID,
	})
}

// LogBetModalSubmitReceived logs when a bet modal submission is received
func (l *Logger) LogBetModalSubmitReceived(requestID, slackUserID, channelID string, oppID int64, book string, stakeCents int, betIntentID string) {
	l.Log(LogEntry{
		Event:          EventBetModalSubmitReceived,
		RequestID:      requestID,
		SlackUserID:    slackUserID,
		SlackChannelID: channelID,
		OppID:          &oppID,
		Book:           book,
		StakeCents:     &stakeCents,
		BetIntentID:    betIntentID,
	})
}

// LogBetBlockedByFilter logs when a bet is blocked by user filters
func (l *Logger) LogBetBlockedByFilter(requestID, slackUserID string, oppID int64, book string, reason string, edgePercent float64, minEdgePercent *float64, booksWhitelist []string) {
	l.Log(LogEntry{
		Event:          EventBetBlockedByFilter,
		RequestID:      requestID,
		SlackUserID:    slackUserID,
		OppID:          &oppID,
		Book:           book,
		Reason:         reason,
		EdgePercent:    &edgePercent,
		MinEdgePercent: minEdgePercent,
		BooksWhitelist: booksWhitelist,
	})
}

// LogBetDedupBlocked logs when a bet is blocked due to duplicate submission
func (l *Logger) LogBetDedupBlocked(requestID, slackUserID string, oppID int64, book string, stakeCents int, betIntentID string) {
	l.Log(LogEntry{
		Event:       EventBetDedupBlocked,
		RequestID:   requestID,
		SlackUserID: slackUserID,
		OppID:       &oppID,
		Book:        book,
		StakeCents:  &stakeCents,
		BetIntentID: betIntentID,
	})
}

// LogBetRequestAccepted logs when a bet request is accepted by the API
func (l *Logger) LogBetRequestAccepted(requestID, slackUserID string, oppID int64, book string, stakeCents int, betIntentID string, betAPIResult map[string]interface{}) {
	l.Log(LogEntry{
		Event:        EventBetRequestAccepted,
		RequestID:    requestID,
		SlackUserID:  slackUserID,
		OppID:        &oppID,
		Book:         book,
		StakeCents:   &stakeCents,
		BetIntentID:  betIntentID,
		BetAPIResult: betAPIResult,
	})
}

// LogBetRequestFailed logs when a bet request fails
func (l *Logger) LogBetRequestFailed(requestID, slackUserID string, oppID int64, book string, stakeCents int, betIntentID string, err error) {
	l.Log(LogEntry{
		Event:       EventBetRequestFailed,
		RequestID:   requestID,
		SlackUserID: slackUserID,
		OppID:       &oppID,
		Book:        book,
		StakeCents:  &stakeCents,
		BetIntentID: betIntentID,
		Error:       err.Error(),
	})
}

// LogBetCompleted logs when a bet is successfully completed
func (l *Logger) LogBetCompleted(requestID, slackUserID string, oppID int64, book string, stakeCents int, betIntentID string, betID int64, ticketNumber string, latencyMs int64) {
	l.Log(LogEntry{
		Event:       EventBetCompleted,
		RequestID:   requestID,
		SlackUserID: slackUserID,
		OppID:       &oppID,
		Book:        book,
		StakeCents:  &stakeCents,
		BetIntentID: betIntentID,
		LatencyMs:   &latencyMs,
		BetAPIResult: map[string]interface{}{
			"bet_id":        betID,
			"ticket_number": ticketNumber,
		},
	})
}

// LogBetFailed logs when a bet execution fails
func (l *Logger) LogBetFailed(requestID, slackUserID string, oppID int64, book string, stakeCents int, betIntentID string, errMsg string, latencyMs int64) {
	l.Log(LogEntry{
		Event:       EventBetFailed,
		RequestID:   requestID,
		SlackUserID: slackUserID,
		OppID:       &oppID,
		Book:        book,
		StakeCents:  &stakeCents,
		BetIntentID: betIntentID,
		Error:       errMsg,
		LatencyMs:   &latencyMs,
	})
}

// LogFiltersUpdated logs when a user updates their filters
func (l *Logger) LogFiltersUpdated(requestID, slackUserID string, booksWhitelist []string, minEdgePercent *float64, enabled bool, defaultStakeCents int) {
	l.Log(LogEntry{
		Event:          EventFiltersUpdated,
		RequestID:      requestID,
		SlackUserID:    slackUserID,
		BooksWhitelist: booksWhitelist,
		MinEdgePercent: minEdgePercent,
		StakeCents:     &defaultStakeCents,
		Extra: map[string]interface{}{
			"enabled": enabled,
		},
	})
}


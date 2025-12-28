package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/XavierBriggs/fortuna/services/slack-gateway/internal/dedup"
	"github.com/XavierBriggs/fortuna/services/slack-gateway/internal/filter"
	"github.com/XavierBriggs/fortuna/services/slack-gateway/internal/logger"
	slackclient "github.com/XavierBriggs/fortuna/services/slack-gateway/internal/slack"
	"github.com/XavierBriggs/fortuna/services/slack-gateway/internal/store"
	"github.com/XavierBriggs/fortuna/services/slack-gateway/pkg/models"
	"github.com/google/uuid"
	"github.com/slack-go/slack"
)

// InteractionsHandler handles Slack interaction callbacks
type InteractionsHandler struct {
	slackClient  *slackclient.Client
	prefStore    *store.PreferenceStore
	oppStore     *store.OpportunityStore
	deduplicator *dedup.Deduplicator
	logger       *logger.Logger
	betAPIURL    string
	httpClient   *http.Client
	alexandriaDB *sql.DB
}

// NewInteractionsHandler creates a new interactions handler
func NewInteractionsHandler(
	slackClient *slackclient.Client,
	prefStore *store.PreferenceStore,
	oppStore *store.OpportunityStore,
	deduplicator *dedup.Deduplicator,
	log *logger.Logger,
	betAPIURL string,
	alexandriaDB *sql.DB,
) *InteractionsHandler {
	return &InteractionsHandler{
		slackClient:  slackClient,
		prefStore:    prefStore,
		oppStore:     oppStore,
		deduplicator: deduplicator,
		logger:       log,
		betAPIURL:    betAPIURL,
		httpClient: &http.Client{
			Timeout: 90 * time.Second, // Long timeout for bet execution
		},
		alexandriaDB: alexandriaDB,
	}
}

// HandleInteractions handles all Slack interaction callbacks
func (h *InteractionsHandler) HandleInteractions(w http.ResponseWriter, r *http.Request) {
	// Verify signature and get body
	body, err := h.slackClient.VerifySignatureWithBody(r)
	if err != nil {
		h.logger.Error("signature_verification_failed", err, nil)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse the payload
	payloadStr, err := url.QueryUnescape(strings.TrimPrefix(string(body), "payload="))
	if err != nil {
		h.logger.Error("payload_parse_failed", err, nil)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	var callback slack.InteractionCallback
	if err := json.Unmarshal([]byte(payloadStr), &callback); err != nil {
		h.logger.Error("callback_unmarshal_failed", err, nil)
		http.Error(w, "Invalid callback", http.StatusBadRequest)
		return
	}

	// Generate request ID
	requestID := uuid.New().String()

	// Debug: Log received callback type
	h.logger.Info("interaction_received", map[string]interface{}{
		"request_id":    requestID,
		"callback_type": callback.Type,
		"user_id":       callback.User.ID,
	})

	// Route based on callback type
	switch callback.Type {
	case slack.InteractionTypeBlockActions:
		h.handleBlockActions(w, callback, requestID)
	case slack.InteractionTypeViewSubmission:
		h.handleViewSubmission(w, callback, requestID)
	default:
		h.logger.Info("unknown_callback_type", map[string]interface{}{
			"callback_type": callback.Type,
		})
		w.WriteHeader(http.StatusOK)
	}
}

// handleBlockActions handles button clicks and other block actions
func (h *InteractionsHandler) handleBlockActions(w http.ResponseWriter, callback slack.InteractionCallback, requestID string) {
	// Debug: Log all block actions received
	for _, action := range callback.ActionCallback.BlockActions {
		h.logger.Info("block_action_received", map[string]interface{}{
			"request_id": requestID,
			"action_id":  action.ActionID,
			"action_type": action.Type,
			"value":      action.Value,
		})

		// Check if this is a place_bet button (matches place_bet, place_bet_0, place_bet_1, etc.)
		if strings.HasPrefix(action.ActionID, "place_bet") {
			h.handlePlaceBetButton(w, callback, action, requestID)
			return
		}

		switch action.ActionID {
		case "view_details":
			// URL buttons are handled by Slack, no action needed
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	h.logger.Info("no_matching_action", map[string]interface{}{
		"request_id": requestID,
		"num_actions": len(callback.ActionCallback.BlockActions),
	})
	w.WriteHeader(http.StatusOK)
}

// handlePlaceBetButton handles the Place Bet button click
func (h *InteractionsHandler) handlePlaceBetButton(w http.ResponseWriter, callback slack.InteractionCallback, action *slack.BlockAction, requestID string) {
	ctx := context.Background()

	// Parse button value
	var buttonValue models.ButtonValue
	if err := json.Unmarshal([]byte(action.Value), &buttonValue); err != nil {
		h.logger.Error("button_value_parse_failed", err, nil)
		http.Error(w, "Invalid button value", http.StatusBadRequest)
		return
	}

	// Generate new bet intent ID
	betIntentID := uuid.New().String()

	// Log button click
	h.logger.LogBetButtonClicked(requestID, callback.User.ID, callback.Channel.ID, buttonValue.OppID, buttonValue.Book, betIntentID)

	// Load opportunity
	opp, err := h.oppStore.LoadOpportunity(ctx, buttonValue.OppID)
	if err != nil {
		h.logger.Error("opportunity_load_failed", err, map[string]interface{}{"opp_id": buttonValue.OppID})
		h.slackClient.SendEphemeral(callback.Channel.ID, callback.User.ID, fmt.Sprintf("❌ Failed to load opportunity: %v", err))
		w.WriteHeader(http.StatusOK)
		return
	}

	// Get event info for display
	homeTeam, awayTeam, err := h.oppStore.GetEventInfo(ctx, h.alexandriaDB, opp.EventID)
	eventName := opp.EventID
	if err == nil {
		eventName = fmt.Sprintf("%s @ %s", awayTeam, homeTeam)
	}

	// Find the leg for this book
	var leg *models.OpportunityLeg
	for i, l := range opp.Legs {
		if strings.EqualFold(l.BookKey, buttonValue.Book) || i == buttonValue.LegIdx {
			leg = &opp.Legs[i]
			break
		}
	}
	if leg == nil && len(opp.Legs) > 0 {
		leg = &opp.Legs[0]
	}

	// Format line
	line := "N/A"
	if leg != nil {
		line = fmt.Sprintf("%s @ %s", leg.OutcomeName, slackclient.FormatOdds(leg.Price))
		if leg.Point != nil {
			line = fmt.Sprintf("%s (%.1f)", line, *leg.Point)
		}
	}

	// Format fair price
	fairPrice := "N/A"
	if opp.FairPrice != nil {
		fairPrice = slackclient.FormatOdds(*opp.FairPrice)
	}

	// Load user preferences for default stake
	pref, _ := h.prefStore.LoadPreference(ctx, callback.User.ID)
	defaultStake := buttonValue.DefaultStake
	if pref != nil && pref.DefaultStakeCents > 0 {
		defaultStake = pref.DefaultStakeCents / 100
	}

	// Kelly suggestion (placeholder - could integrate with Kelly service)
	kellyStake := float64(defaultStake) * 0.9625

	// Build modal metadata
	metadata := models.ModalMetadata{
		OppID:       buttonValue.OppID,
		Book:        buttonValue.Book,
		ChannelID:   callback.Channel.ID,
		BetIntentID: betIntentID,
		LegIdx:      buttonValue.LegIdx,
		MessageTS:   callback.Message.Timestamp,
	}
	metadataJSON, _ := json.Marshal(metadata)

	// Build and open modal
	modal := h.slackClient.BuildBetConfirmationModal(
		eventName,
		opp.MarketKey,
		opp.EdgePercent,
		slackclient.BookDisplayName(buttonValue.Book),
		line,
		fairPrice,
		defaultStake,
		kellyStake,
		string(metadataJSON),
	)

	if err := h.slackClient.OpenModal(callback.TriggerID, modal); err != nil {
		h.logger.Error("modal_open_failed", err, nil)
		h.slackClient.SendEphemeral(callback.Channel.ID, callback.User.ID, "❌ Failed to open bet confirmation modal")
	}

	w.WriteHeader(http.StatusOK)
}

// handleViewSubmission handles modal form submissions
func (h *InteractionsHandler) handleViewSubmission(w http.ResponseWriter, callback slack.InteractionCallback, requestID string) {
	switch callback.View.CallbackID {
	case "bet_confirmation":
		h.handleBetConfirmation(w, callback, requestID)
	case "filter_settings":
		h.handleFilterSettings(w, callback, requestID)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

// handleBetConfirmation handles bet confirmation modal submission
func (h *InteractionsHandler) handleBetConfirmation(w http.ResponseWriter, callback slack.InteractionCallback, requestID string) {
	ctx := context.Background()

	// Parse metadata
	var metadata models.ModalMetadata
	if err := json.Unmarshal([]byte(callback.View.PrivateMetadata), &metadata); err != nil {
		h.logger.Error("metadata_parse_failed", err, nil)
		h.respondWithError(w, "Invalid request")
		return
	}

	// Extract stake from form
	stakeStr := ""
	if stakeBlock, ok := callback.View.State.Values["stake_block"]; ok {
		if stakeInput, ok := stakeBlock["stake_input"]; ok {
			stakeStr = stakeInput.Value
		}
	}

	// Parse and validate stake
	stakeCents, err := filter.ParseStakeDollars(stakeStr)
	if err != nil {
		h.respondWithValidationError(w, "stake_block", err.Error())
		return
	}

	if err := filter.ValidateStake(stakeCents, 100, 100000); err != nil { // $1 min, $1000 max
		h.respondWithValidationError(w, "stake_block", err.Error())
		return
	}

	// Log modal submission
	h.logger.LogBetModalSubmitReceived(requestID, callback.User.ID, metadata.ChannelID, metadata.OppID, metadata.Book, stakeCents, metadata.BetIntentID)

	// Load opportunity for filter check
	opp, err := h.oppStore.LoadOpportunity(ctx, metadata.OppID)
	if err != nil {
		h.logger.Error("opportunity_load_failed", err, nil)
		h.respondWithError(w, "Failed to load opportunity")
		return
	}

	// Load user preferences
	pref, _ := h.prefStore.LoadPreference(ctx, callback.User.ID)

	// Check filter
	filterResult := filter.ShouldAllow(pref, opp, metadata.Book)
	if !filterResult.Allowed {
		h.logger.LogBetBlockedByFilter(requestID, callback.User.ID, metadata.OppID, metadata.Book, filterResult.Reason, opp.EdgePercent, nil, nil)

		// Send ephemeral and ack
		go h.slackClient.SendEphemeral(metadata.ChannelID, callback.User.ID,
			fmt.Sprintf("🚫 Bet blocked by your filters\n\nReason: %s\n\nUpdate your filters with `/fortuna filters`", filterResult.Reason))

		w.WriteHeader(http.StatusOK)
		return
	}

	// Acquire dedup lock
	acquired, err := h.deduplicator.AcquireIntent(ctx, metadata.BetIntentID)
	if err != nil {
		h.logger.Error("dedup_acquire_failed", err, nil)
	}
	if !acquired {
		h.logger.LogBetDedupBlocked(requestID, callback.User.ID, metadata.OppID, metadata.Book, stakeCents, metadata.BetIntentID)

		go h.slackClient.SendEphemeral(metadata.ChannelID, callback.User.ID,
			"⚠️ Duplicate submission detected\n\nThis bet confirmation was already processed.\nIf you want to place another bet, tap \"Place Bet\" again to start a new confirmation.")

		w.WriteHeader(http.StatusOK)
		return
	}

	// Acknowledge Slack immediately
	w.WriteHeader(http.StatusOK)

	// Execute bet in background
	go h.executeBetAsync(ctx, callback.User.ID, metadata, stakeCents, opp, requestID)
}

// executeBetAsync executes the bet placement asynchronously
func (h *InteractionsHandler) executeBetAsync(ctx context.Context, slackUserID string, metadata models.ModalMetadata, stakeCents int, opp *models.Opportunity, requestID string) {
	startTime := time.Now()

	// Send processing message
	h.slackClient.SendEphemeral(metadata.ChannelID, slackUserID,
		fmt.Sprintf("⏳ Processing bet...\n\nPlacing $%.2f on opportunity %d via %s",
			float64(stakeCents)/100, metadata.OppID, slackclient.BookDisplayName(metadata.Book)))

	// Find the leg
	var leg *models.OpportunityLeg
	for i, l := range opp.Legs {
		if strings.EqualFold(l.BookKey, metadata.Book) || i == metadata.LegIdx {
			leg = &opp.Legs[i]
			break
		}
	}
	if leg == nil {
		h.logger.LogBetFailed(requestID, slackUserID, metadata.OppID, metadata.Book, stakeCents, metadata.BetIntentID, "leg not found", 0)
		h.postBetFailedReply(metadata.ChannelID, metadata.MessageTS, "Bet leg not found")
		return
	}

	// Build bet request
	betReq := models.PlaceBetRequest{
		OpportunityID: metadata.OppID,
		Legs: []models.LegRequest{
			{
				BookKey:      leg.BookKey,
				OutcomeName:  leg.OutcomeName,
				Stake:        float64(stakeCents) / 100,
				ExpectedOdds: leg.Price,
			},
		},
	}

	// Call bet API
	betResp, err := h.callBetAPI(ctx, betReq)
	latencyMs := time.Since(startTime).Milliseconds()

	if err != nil {
		h.logger.LogBetFailed(requestID, slackUserID, metadata.OppID, metadata.Book, stakeCents, metadata.BetIntentID, err.Error(), latencyMs)
		h.postBetFailedReply(metadata.ChannelID, metadata.MessageTS, err.Error())
		return
	}

	// Check response
	if !betResp.Success || len(betResp.Results) == 0 || !betResp.Results[0].Success {
		errMsg := "Unknown error"
		if len(betResp.Results) > 0 && betResp.Results[0].Error != "" {
			errMsg = betResp.Results[0].Error
		}
		h.logger.LogBetFailed(requestID, slackUserID, metadata.OppID, metadata.Book, stakeCents, metadata.BetIntentID, errMsg, latencyMs)
		h.postBetFailedReply(metadata.ChannelID, metadata.MessageTS, errMsg)
		return
	}

	// Success!
	result := betResp.Results[0]
	betID := int64(0)
	if result.BetID != nil {
		betID = *result.BetID
	}

	h.logger.LogBetCompleted(requestID, slackUserID, metadata.OppID, metadata.Book, stakeCents, metadata.BetIntentID, betID, result.TicketNumber, latencyMs)

	// Post success reply
	h.postBetSuccessReply(metadata.ChannelID, metadata.MessageTS, stakeCents, leg.OutcomeName, leg.Price, betID, result.TicketNumber, float64(latencyMs)/1000)

	// Mark intent as completed
	h.deduplicator.MarkIntentCompleted(ctx, metadata.BetIntentID, fmt.Sprintf("success:bet_id=%d", betID))
}

// callBetAPI calls the bet placement API
func (h *InteractionsHandler) callBetAPI(ctx context.Context, req models.PlaceBetRequest) (*models.PlaceBetResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", h.betAPIURL+"/api/v1/bets/place-with-bot", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bet API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bet API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var betResp models.PlaceBetResponse
	if err := json.Unmarshal(respBody, &betResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &betResp, nil
}

// postBetSuccessReply posts a success reply in the thread
func (h *InteractionsHandler) postBetSuccessReply(channelID, threadTS string, stakeCents int, outcomeName string, odds int, betID int64, ticketNumber string, latencySec float64) {
	text := fmt.Sprintf("✅ *Bet Placed Successfully*\n\n"+
		"*Stake:* $%.2f\n"+
		"*Line:* %s @ %s\n"+
		"*Bet ID:* %d\n",
		float64(stakeCents)/100,
		outcomeName,
		slackclient.FormatOdds(odds),
		betID,
	)

	if ticketNumber != "" {
		text += fmt.Sprintf("*Ticket:* %s\n", ticketNumber)
	}
	text += fmt.Sprintf("*Latency:* %.1fs", latencySec)

	if threadTS != "" {
		h.slackClient.PostThreadReply(channelID, threadTS, text)
	} else {
		h.slackClient.PostMessage(channelID, text)
	}
}

// postBetFailedReply posts a failure reply in the thread
func (h *InteractionsHandler) postBetFailedReply(channelID, threadTS string, errMsg string) {
	text := fmt.Sprintf("❌ *Bet Failed*\n\n"+
		"*Error:* %s\n\n"+
		"The opportunity may no longer be available.",
		errMsg,
	)

	if threadTS != "" {
		h.slackClient.PostThreadReply(channelID, threadTS, text)
	} else {
		h.slackClient.PostMessage(channelID, text)
	}
}

// handleFilterSettings handles filter settings modal submission
func (h *InteractionsHandler) handleFilterSettings(w http.ResponseWriter, callback slack.InteractionCallback, requestID string) {
	ctx := context.Background()

	// Extract values from form
	var booksWhitelist []string
	var minEdgePercent *float64
	defaultStakeCents := 10000 // Default $100
	enabled := true

	// Books
	if booksBlock, ok := callback.View.State.Values["books_block"]; ok {
		if booksSelect, ok := booksBlock["books_select"]; ok {
			for _, opt := range booksSelect.SelectedOptions {
				booksWhitelist = append(booksWhitelist, opt.Value)
			}
		}
	}

	// Min edge
	if minEdgeBlock, ok := callback.View.State.Values["min_edge_block"]; ok {
		if minEdgeInput, ok := minEdgeBlock["min_edge_input"]; ok {
			if minEdgeInput.Value != "" {
				var edge float64
				if _, err := fmt.Sscanf(minEdgeInput.Value, "%f", &edge); err == nil {
					minEdgePercent = &edge
				}
			}
		}
	}

	// Default stake
	if stakeBlock, ok := callback.View.State.Values["stake_block"]; ok {
		if stakeInput, ok := stakeBlock["default_stake_input"]; ok {
			if stakeInput.Value != "" {
				if dollars, err := strconv.ParseFloat(stakeInput.Value, 64); err == nil {
					defaultStakeCents = int(dollars * 100)
				}
			}
		}
	}

	// Enabled
	if enabledBlock, ok := callback.View.State.Values["enabled_block"]; ok {
		if enabledSelect, ok := enabledBlock["enabled_select"]; ok {
			enabled = enabledSelect.SelectedOption.Value == "true"
		}
	}

	// Save preferences
	pref := &models.SlackFilterPreference{
		SlackUserID:       callback.User.ID,
		BooksWhitelist:    booksWhitelist,
		MinEdgePercent:    minEdgePercent,
		Enabled:           enabled,
		DefaultStakeCents: defaultStakeCents,
	}

	if err := h.prefStore.SavePreference(ctx, pref); err != nil {
		h.logger.Error("preference_save_failed", err, nil)
		h.respondWithError(w, "Failed to save preferences")
		return
	}

	// Log update
	h.logger.LogFiltersUpdated(requestID, callback.User.ID, booksWhitelist, minEdgePercent, enabled, defaultStakeCents)

	// Acknowledge
	w.WriteHeader(http.StatusOK)

	// Send confirmation
	booksStr := "All books"
	if len(booksWhitelist) > 0 {
		booksStr = strings.Join(booksWhitelist, ", ")
	}

	edgeStr := "No minimum"
	if minEdgePercent != nil {
		edgeStr = fmt.Sprintf("%.1f%%", *minEdgePercent)
	}

	enabledStr := "Enabled"
	if !enabled {
		enabledStr = "Disabled"
	}

	// Get channel ID from the command context (stored earlier)
	// For now, we'll skip the confirmation message since we don't have channel context
	// The modal close is sufficient feedback
	_ = fmt.Sprintf("✅ *Filters updated!*\n\n"+
		"• *Books:* %s\n"+
		"• *Min Edge:* %s\n"+
		"• *Default Stake:* $%.2f\n"+
		"• *Alerts:* %s",
		booksStr, edgeStr, float64(defaultStakeCents)/100, enabledStr)
}

// respondWithError responds with an error message
func (h *InteractionsHandler) respondWithError(w http.ResponseWriter, message string) {
	response := map[string]interface{}{
		"response_action": "errors",
		"errors": map[string]string{
			"stake_block": message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// respondWithValidationError responds with a validation error for a specific block
func (h *InteractionsHandler) respondWithValidationError(w http.ResponseWriter, blockID, message string) {
	response := map[string]interface{}{
		"response_action": "errors",
		"errors": map[string]string{
			blockID: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}












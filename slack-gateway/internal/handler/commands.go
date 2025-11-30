package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/XavierBriggs/fortuna/services/slack-gateway/internal/logger"
	slackclient "github.com/XavierBriggs/fortuna/services/slack-gateway/internal/slack"
	"github.com/XavierBriggs/fortuna/services/slack-gateway/internal/store"
	"github.com/google/uuid"
)

// CommandsHandler handles Slack slash commands
type CommandsHandler struct {
	slackClient *slackclient.Client
	prefStore   *store.PreferenceStore
	logger      *logger.Logger
}

// NewCommandsHandler creates a new commands handler
func NewCommandsHandler(
	slackClient *slackclient.Client,
	prefStore *store.PreferenceStore,
	log *logger.Logger,
) *CommandsHandler {
	return &CommandsHandler{
		slackClient: slackClient,
		prefStore:   prefStore,
		logger:      log,
	}
}

// HandleCommands handles all Slack slash commands
func (h *CommandsHandler) HandleCommands(w http.ResponseWriter, r *http.Request) {
	// Verify signature
	body, err := h.slackClient.VerifySignatureWithBody(r)
	if err != nil {
		h.logger.Error("signature_verification_failed", err, nil)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse form data from the body we already read
	formValues, err := url.ParseQuery(string(body))
	if err != nil {
		h.logger.Error("form_parse_failed", err, nil)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Get command details
	command := formValues.Get("command")
	text := formValues.Get("text")
	userID := formValues.Get("user_id")
	triggerID := formValues.Get("trigger_id")
	channelID := formValues.Get("channel_id")

	// Generate request ID
	requestID := uuid.New().String()

	h.logger.Info("command_received", map[string]interface{}{
		"request_id": requestID,
		"command":    command,
		"text":       text,
		"user_id":    userID,
		"channel_id": channelID,
	})

	// Route based on command
	switch command {
	case "/fortuna":
		h.handleFortunaCommand(w, text, userID, triggerID, channelID, requestID)
	default:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Unknown command: %s", command)))
	}
}

// handleFortunaCommand handles the /fortuna command
func (h *CommandsHandler) handleFortunaCommand(w http.ResponseWriter, text, userID, triggerID, channelID, requestID string) {
	// Parse subcommand
	parts := strings.Fields(text)
	subcommand := ""
	if len(parts) > 0 {
		subcommand = strings.ToLower(parts[0])
	}

	switch subcommand {
	case "filters", "filter", "settings", "prefs", "preferences":
		h.openFilterSettingsModal(w, userID, triggerID, channelID, requestID)
	case "help", "":
		h.showHelp(w)
	case "status":
		h.showStatus(w, userID, channelID, requestID)
	default:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Unknown subcommand: %s\n\nUse `/fortuna help` for available commands.", subcommand)))
	}
}

// openFilterSettingsModal opens the filter settings modal
func (h *CommandsHandler) openFilterSettingsModal(w http.ResponseWriter, userID, triggerID, channelID, requestID string) {
	ctx := context.Background()

	// Load current preferences
	pref, err := h.prefStore.LoadPreference(ctx, userID)
	if err != nil {
		h.logger.Error("preference_load_failed", err, nil)
	}

	// Use defaults if no preference exists
	var selectedBooks []string
	var minEdge string
	defaultStake := 100 // $100 default
	enabled := true

	if pref != nil {
		selectedBooks = pref.BooksWhitelist
		if pref.MinEdgePercent != nil {
			minEdge = fmt.Sprintf("%.1f", *pref.MinEdgePercent)
		}
		defaultStake = pref.DefaultStakeCents / 100
		enabled = pref.Enabled
	}

	// Build and open modal
	modal := h.slackClient.BuildFilterSettingsModal(
		selectedBooks,
		minEdge,
		defaultStake*100, // Convert to cents for the function
		enabled,
	)

	if err := h.slackClient.OpenModal(triggerID, modal); err != nil {
		h.logger.Error("modal_open_failed", err, nil)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("❌ Failed to open filter settings modal. Please try again."))
		return
	}

	h.logger.Info(logger.EventFilterModalOpened, map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"channel_id": channelID,
	})

	// Acknowledge command
	w.WriteHeader(http.StatusOK)
}

// showHelp displays help information
func (h *CommandsHandler) showHelp(w http.ResponseWriter) {
	helpText := "*Fortuna Slack Bot Commands*\n\n" +
		"• `/fortuna filters` - Open filter settings modal to configure alerts and betting preferences\n" +
		"• `/fortuna status` - Show your current filter settings\n" +
		"• `/fortuna help` - Show this help message\n\n" +
		"*Alert Buttons*\n" +
		"When you receive an opportunity alert, you can:\n" +
		"• Click *Place Bet* to open a bet confirmation modal\n" +
		"• Click *View Details* to see the opportunity in the web UI\n\n" +
		"*Filter Settings*\n" +
		"• *Allowed Books* - Only receive alerts and allow bets for selected books\n" +
		"• *Minimum Edge* - Only receive alerts for opportunities above this edge percentage\n" +
		"• *Default Stake* - Pre-filled stake amount in bet confirmation modal\n" +
		"• *Alerts Enabled* - Toggle alerts and Slack betting on/off"

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(helpText))
}

// showStatus displays current filter settings
func (h *CommandsHandler) showStatus(w http.ResponseWriter, userID, channelID, requestID string) {
	ctx := context.Background()

	pref, err := h.prefStore.LoadPreference(ctx, userID)
	if err != nil {
		h.logger.Error("preference_load_failed", err, nil)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("❌ Failed to load your settings. Please try again."))
		return
	}

	var statusText string
	if pref == nil {
		statusText = "*Your Current Settings*\n\n" +
			"You're using default settings:\n" +
			"• *Books:* All books allowed\n" +
			"• *Min Edge:* No minimum\n" +
			"• *Default Stake:* $100\n" +
			"• *Alerts:* Enabled\n\n" +
			"Use `/fortuna filters` to customize your preferences."
	} else {
		booksStr := "All books"
		if len(pref.BooksWhitelist) > 0 {
			booksStr = strings.Join(pref.BooksWhitelist, ", ")
		}

		edgeStr := "No minimum"
		if pref.MinEdgePercent != nil {
			edgeStr = fmt.Sprintf("%.1f%%", *pref.MinEdgePercent)
		}

		enabledStr := "Enabled ✅"
		if !pref.Enabled {
			enabledStr = "Disabled ❌"
		}

		statusText = fmt.Sprintf("*Your Current Settings*\n\n"+
			"• *Books:* %s\n"+
			"• *Min Edge:* %s\n"+
			"• *Default Stake:* $%.2f\n"+
			"• *Alerts:* %s\n\n"+
			"Use `/fortuna filters` to update your preferences.",
			booksStr, edgeStr, float64(pref.DefaultStakeCents)/100, enabledStr)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(statusText))
}

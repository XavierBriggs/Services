package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

// Client wraps the Slack API client with signature verification
type Client struct {
	api           *slack.Client
	signingSecret string
}

// NewClient creates a new Slack client wrapper
func NewClient(botToken, signingSecret string) *Client {
	return &Client{
		api:           slack.New(botToken),
		signingSecret: signingSecret,
	}
}

// VerifySignature validates the Slack request signature
// Returns nil if valid, error otherwise
func (c *Client) VerifySignature(r *http.Request) error {
	// Get timestamp header
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	if timestamp == "" {
		return fmt.Errorf("missing X-Slack-Request-Timestamp header")
	}

	// Verify timestamp is not too old (prevent replay attacks)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	if abs(time.Now().Unix()-ts) > 300 {
		return fmt.Errorf("request timestamp too old")
	}

	// Get signature header
	signature := r.Header.Get("X-Slack-Signature")
	if signature == "" {
		return fmt.Errorf("missing X-Slack-Signature header")
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// Compute expected signature
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	h := hmac.New(sha256.New, []byte(c.signingSecret))
	h.Write([]byte(baseString))
	expectedSignature := "v0=" + hex.EncodeToString(h.Sum(nil))

	// Compare signatures
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// VerifySignatureWithBody validates the Slack request signature and returns the body
func (c *Client) VerifySignatureWithBody(r *http.Request) ([]byte, error) {
	// Get timestamp header
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	if timestamp == "" {
		return nil, fmt.Errorf("missing X-Slack-Request-Timestamp header")
	}

	// Verify timestamp is not too old (prevent replay attacks)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	if abs(time.Now().Unix()-ts) > 300 {
		return nil, fmt.Errorf("request timestamp too old")
	}

	// Get signature header
	signature := r.Header.Get("X-Slack-Signature")
	if signature == "" {
		return nil, fmt.Errorf("missing X-Slack-Signature header")
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Compute expected signature
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	h := hmac.New(sha256.New, []byte(c.signingSecret))
	h.Write([]byte(baseString))
	expectedSignature := "v0=" + hex.EncodeToString(h.Sum(nil))

	// Compare signatures
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, fmt.Errorf("invalid signature")
	}

	return body, nil
}

// OpenModal opens a modal view in Slack
func (c *Client) OpenModal(triggerID string, view slack.ModalViewRequest) error {
	_, err := c.api.OpenView(triggerID, view)
	if err != nil {
		return fmt.Errorf("failed to open modal: %w", err)
	}
	return nil
}

// SendEphemeral sends an ephemeral message visible only to the specified user
func (c *Client) SendEphemeral(channelID, userID, text string) error {
	_, err := c.api.PostEphemeral(channelID, userID, slack.MsgOptionText(text, false))
	if err != nil {
		return fmt.Errorf("failed to send ephemeral message: %w", err)
	}
	return nil
}

// SendEphemeralBlocks sends an ephemeral message with blocks
func (c *Client) SendEphemeralBlocks(channelID, userID string, blocks []slack.Block) error {
	_, err := c.api.PostEphemeral(channelID, userID, slack.MsgOptionBlocks(blocks...))
	if err != nil {
		return fmt.Errorf("failed to send ephemeral message: %w", err)
	}
	return nil
}

// PostMessage posts a message to a channel
func (c *Client) PostMessage(channelID, text string) (string, error) {
	_, ts, err := c.api.PostMessage(channelID, slack.MsgOptionText(text, false))
	if err != nil {
		return "", fmt.Errorf("failed to post message: %w", err)
	}
	return ts, nil
}

// PostThreadReply posts a reply in a thread
func (c *Client) PostThreadReply(channelID, threadTS, text string) error {
	_, _, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionText(text, false),
		slack.MsgOptionTS(threadTS),
	)
	if err != nil {
		return fmt.Errorf("failed to post thread reply: %w", err)
	}
	return nil
}

// PostThreadReplyBlocks posts a reply in a thread with blocks
func (c *Client) PostThreadReplyBlocks(channelID, threadTS string, blocks []slack.Block) error {
	_, _, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionTS(threadTS),
	)
	if err != nil {
		return fmt.Errorf("failed to post thread reply: %w", err)
	}
	return nil
}

// BuildBetConfirmationModal builds the bet confirmation modal view
func (c *Client) BuildBetConfirmationModal(
	event string,
	market string,
	edgePercent float64,
	book string,
	line string,
	fairPrice string,
	defaultStake int,
	kellyStake float64,
	privateMetadata string,
) slack.ModalViewRequest {
	// Create stake input element with initial value
	stakeInput := &slack.PlainTextInputBlockElement{
		Type:         slack.METPlainTextInput,
		ActionID:     "stake_input",
		Placeholder:  slack.NewTextBlockObject(slack.PlainTextType, "Enter stake amount", false, false),
		InitialValue: fmt.Sprintf("%d", defaultStake),
	}

	return slack.ModalViewRequest{
		Type:            slack.VTModal,
		CallbackID:      "bet_confirmation",
		Title:           slack.NewTextBlockObject(slack.PlainTextType, "Confirm Bet", false, false),
		Submit:          slack.NewTextBlockObject(slack.PlainTextType, "Confirm Bet", false, false),
		Close:           slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		PrivateMetadata: privateMetadata,
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				// Opportunity Summary Header
				slack.NewSectionBlock(
					slack.NewTextBlockObject(slack.MarkdownType, "*📊 Opportunity Summary*", false, false),
					nil,
					nil,
				),
				// Opportunity Fields
				slack.NewSectionBlock(
					nil,
					[]*slack.TextBlockObject{
						slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Event:*\n%s", event), false, false),
						slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Market:*\n%s", market), false, false),
						slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Edge:*\n%.2f%%", edgePercent), false, false),
					},
					nil,
				),
				slack.NewDividerBlock(),
				// Bet Details Header
				slack.NewSectionBlock(
					slack.NewTextBlockObject(slack.MarkdownType, "*🎯 Bet Details*", false, false),
					nil,
					nil,
				),
				// Bet Fields
				slack.NewSectionBlock(
					nil,
					[]*slack.TextBlockObject{
						slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Book:*\n%s", book), false, false),
						slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Line:*\n%s", line), false, false),
						slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("*Fair Price:*\n%s", fairPrice), false, false),
					},
					nil,
				),
				slack.NewDividerBlock(),
				// Stake Input
				&slack.InputBlock{
					Type:    slack.MBTInput,
					BlockID: "stake_block",
					Label:   slack.NewTextBlockObject(slack.PlainTextType, "💵 Stake Amount ($)", false, false),
					Element: stakeInput,
					Hint:    slack.NewTextBlockObject(slack.PlainTextType, fmt.Sprintf("Kelly suggests: $%.2f", kellyStake), false, false),
				},
				// Warning Context
				slack.NewContextBlock(
					"warning_block",
					slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("⚠️ This will place a real bet on %s", book), false, false),
				),
			},
		},
	}
}

// BuildFilterSettingsModal builds the filter settings modal view
func (c *Client) BuildFilterSettingsModal(
	selectedBooks []string,
	minEdge string,
	defaultStake int,
	enabled bool,
) slack.ModalViewRequest {
	// Build book options
	bookOptions := []*slack.OptionBlockObject{
		slack.NewOptionBlockObject("betonline", slack.NewTextBlockObject(slack.PlainTextType, "BetOnline", false, false), nil),
		slack.NewOptionBlockObject("betus", slack.NewTextBlockObject(slack.PlainTextType, "BetUS", false, false), nil),
		slack.NewOptionBlockObject("bovada", slack.NewTextBlockObject(slack.PlainTextType, "Bovada", false, false), nil),
	}

	// Build initial book selections
	var initialBooks []*slack.OptionBlockObject
	for _, book := range selectedBooks {
		for _, opt := range bookOptions {
			if opt.Value == book {
				initialBooks = append(initialBooks, opt)
			}
		}
	}

	// Enabled options
	enabledOptions := []*slack.OptionBlockObject{
		slack.NewOptionBlockObject("true", slack.NewTextBlockObject(slack.PlainTextType, "On", false, false), nil),
		slack.NewOptionBlockObject("false", slack.NewTextBlockObject(slack.PlainTextType, "Off", false, false), nil),
	}

	enabledValue := "true"
	if !enabled {
		enabledValue = "false"
	}
	var initialEnabled *slack.OptionBlockObject
	for _, opt := range enabledOptions {
		if opt.Value == enabledValue {
			initialEnabled = opt
		}
	}

	// Build multi-select element for books
	booksSelect := &slack.MultiSelectBlockElement{
		Type:           slack.MultiOptTypeStatic,
		ActionID:       "books_select",
		Placeholder:    slack.NewTextBlockObject(slack.PlainTextType, "Select books...", false, false),
		Options:        bookOptions,
		InitialOptions: initialBooks,
	}

	// Min edge input
	minEdgeInput := &slack.PlainTextInputBlockElement{
		Type:        slack.METPlainTextInput,
		ActionID:    "min_edge_input",
		Placeholder: slack.NewTextBlockObject(slack.PlainTextType, "e.g., 2.0", false, false),
	}
	if minEdge != "" {
		minEdgeInput.InitialValue = minEdge
	}

	// Default stake input
	stakeInput := &slack.PlainTextInputBlockElement{
		Type:         slack.METPlainTextInput,
		ActionID:     "default_stake_input",
		Placeholder:  slack.NewTextBlockObject(slack.PlainTextType, "e.g., 100", false, false),
		InitialValue: fmt.Sprintf("%d", defaultStake/100), // Convert cents to dollars
	}

	// Enabled select
	enabledSelect := &slack.SelectBlockElement{
		Type:          slack.OptTypeStatic,
		ActionID:      "enabled_select",
		Options:       enabledOptions,
		InitialOption: initialEnabled,
	}

	return slack.ModalViewRequest{
		Type:       slack.VTModal,
		CallbackID: "filter_settings",
		Title:      slack.NewTextBlockObject(slack.PlainTextType, "Alert & Bet Filters", false, false),
		Submit:     slack.NewTextBlockObject(slack.PlainTextType, "Save", false, false),
		Close:      slack.NewTextBlockObject(slack.PlainTextType, "Cancel", false, false),
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				// Books multi-select
				&slack.InputBlock{
					Type:     slack.MBTInput,
					BlockID:  "books_block",
					Optional: true,
					Label:    slack.NewTextBlockObject(slack.PlainTextType, "📚 Allowed Books", false, false),
					Element:  booksSelect,
					Hint:     slack.NewTextBlockObject(slack.PlainTextType, "Leave empty to allow all books", false, false),
				},
				// Min edge input
				&slack.InputBlock{
					Type:     slack.MBTInput,
					BlockID:  "min_edge_block",
					Optional: true,
					Label:    slack.NewTextBlockObject(slack.PlainTextType, "📈 Minimum Edge (%)", false, false),
					Element:  minEdgeInput,
					Hint:     slack.NewTextBlockObject(slack.PlainTextType, "Leave empty for no minimum", false, false),
				},
				// Default stake input
				&slack.InputBlock{
					Type:    slack.MBTInput,
					BlockID: "stake_block",
					Label:   slack.NewTextBlockObject(slack.PlainTextType, "💵 Default Stake ($)", false, false),
					Element: stakeInput,
				},
				// Enabled select
				&slack.InputBlock{
					Type:    slack.MBTInput,
					BlockID: "enabled_block",
					Label:   slack.NewTextBlockObject(slack.PlainTextType, "🔔 Alerts Enabled", false, false),
					Element: enabledSelect,
				},
			},
		},
	}
}

// FormatOdds formats American odds with sign
func FormatOdds(americanOdds int) string {
	if americanOdds > 0 {
		return fmt.Sprintf("+%d", americanOdds)
	}
	return fmt.Sprintf("%d", americanOdds)
}

// BookDisplayName returns a display-friendly book name
func BookDisplayName(bookKey string) string {
	names := map[string]string{
		"betonline":  "BetOnline",
		"betus":      "BetUS",
		"bovada":     "Bovada",
		"fanduel":    "FanDuel",
		"draftkings": "DraftKings",
		"betmgm":     "BetMGM",
		"caesars":    "Caesars",
	}
	if name, ok := names[strings.ToLower(bookKey)]; ok {
		return name
	}
	return bookKey
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

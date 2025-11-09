package settler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Settler handles bet settlement
type Settler struct {
	alexandriaDB *sql.DB
	holocronDB   *sql.DB
	oddsAPIKey   string
	pollInterval time.Duration
	httpClient   *http.Client
}

// NewSettler creates a new settler
func NewSettler(alexandriaDB, holocronDB *sql.DB, oddsAPIKey string, pollInterval time.Duration) *Settler {
	return &Settler{
		alexandriaDB: alexandriaDB,
		holocronDB:   holocronDB,
		oddsAPIKey:   oddsAPIKey,
		pollInterval: pollInterval,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start begins the settlement polling loop
func (s *Settler) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	// Run immediately on start
	if err := s.settlePendingBets(ctx); err != nil {
		fmt.Printf("[Settlement] initial run error: %v\n", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := s.settlePendingBets(ctx); err != nil {
				fmt.Printf("[Settlement] error: %v\n", err)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// settlePendingBets finds and settles all pending bets for completed events
func (s *Settler) settlePendingBets(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in settlePendingBets: %v", r)
			fmt.Printf("[Settlement] PANIC: %v\n", r)
		}
	}()

	// Get all unique event IDs with pending bets
	query := `
		SELECT DISTINCT b.event_id, b.sport_key
		FROM bets b
		WHERE b.result = 'pending'
		  AND b.placed_at < NOW() - INTERVAL '1 hour'
	`

	rows, err := s.holocronDB.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query pending events: %w", err)
	}
	defer rows.Close()

	var events []struct {
		EventID  string
		SportKey string
	}

	for rows.Next() {
		var e struct {
			EventID  string
			SportKey string
		}
		if err := rows.Scan(&e.EventID, &e.SportKey); err != nil {
			return fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}

	if len(events) == 0 {
		return nil
	}

	fmt.Printf("[Settlement] Found %d events with pending bets\n", len(events))

	// Process each event
	for i, event := range events {
		fmt.Printf("[Settlement] Processing event %d/%d: %s (%s)\n", i+1, len(events), event.EventID, event.SportKey)
		if err := s.settleEvent(ctx, event.EventID, event.SportKey); err != nil {
			fmt.Printf("[Settlement] error settling event %s: %v\n", event.EventID, err)
			continue
		}
	}

	return nil
}

// settleEvent settles all bets for a single event
func (s *Settler) settleEvent(ctx context.Context, eventID, sportKey string) error {
	// Fetch event scores from The Odds API
	fmt.Printf("[Settlement] Checking event %s (%s) for completion...\n", eventID, sportKey)
	scores, err := s.fetchEventScores(ctx, sportKey, eventID)
	if err != nil {
		return fmt.Errorf("fetch scores: %w", err)
	}

	if scores == nil {
		fmt.Printf("[Settlement] Event %s: No score data available yet\n", eventID)
		return nil
	}

	// Check if event is completed
	if !scores.Completed {
		fmt.Printf("[Settlement] Event %s: In progress, not completed\n", eventID)
		return nil
	}

	fmt.Printf("[Settlement] Event %s completed - settling bets\n", eventID)

	// Get all pending bets for this event
	bets, err := s.getPendingBets(ctx, eventID)
	if err != nil {
		return fmt.Errorf("get pending bets: %w", err)
	}

	// Settle each bet
	settled := 0
	for _, bet := range bets {
		result, payout := s.determineBetOutcome(bet, scores)
		
		if err := s.updateBetResult(ctx, bet.ID, result, payout); err != nil {
			fmt.Printf("[Settlement] failed to update bet %d: %v\n", bet.ID, err)
			continue
		}
		
		settled++
	}

	fmt.Printf("[Settlement] Settled %d/%d bets for event %s\n", settled, len(bets), eventID)

	return nil
}

// fetchEventScores fetches scores from The Odds API
func (s *Settler) fetchEventScores(ctx context.Context, sportKey, eventID string) (*EventScore, error) {
	url := fmt.Sprintf("https://api.the-odds-api.com/v4/sports/%s/scores/?apiKey=%s&daysFrom=3&eventIds=%s",
		sportKey, s.oddsAPIKey, eventID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var scores []EventScore
	if err := json.NewDecoder(resp.Body).Decode(&scores); err != nil {
		return nil, err
	}

	if len(scores) == 0 {
		return nil, nil
	}

	return &scores[0], nil
}

// EventScore represents score data from API
type EventScore struct {
	ID        string `json:"id"`
	SportKey  string `json:"sport_key"`
	HomeTeam  string `json:"home_team"`
	AwayTeam  string `json:"away_team"`
	Completed bool   `json:"completed"`
	Scores    []struct {
		Name  string `json:"name"`
		Score string `json:"score"`
	} `json:"scores"`
}

// Bet represents a bet record
type Bet struct {
	ID          int64
	MarketKey   string
	OutcomeName string
	BetPrice    int
	Point       *float64
	StakeAmount float64
}

func (s *Settler) getPendingBets(ctx context.Context, eventID string) ([]Bet, error) {
	query := `
		SELECT id, market_key, outcome_name, bet_price, point, stake_amount
		FROM bets
		WHERE event_id = $1 AND result = 'pending'
	`

	rows, err := s.holocronDB.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bets []Bet
	for rows.Next() {
		var bet Bet
		err := rows.Scan(&bet.ID, &bet.MarketKey, &bet.OutcomeName, &bet.BetPrice, &bet.Point, &bet.StakeAmount)
		if err != nil {
			return nil, err
		}
		bets = append(bets, bet)
	}

	return bets, nil
}

// determineBetOutcome determines if a bet won, lost, or pushed
func (s *Settler) determineBetOutcome(bet Bet, scores *EventScore) (string, float64) {
	// Get team scores
	var homeScore, awayScore int
	for _, score := range scores.Scores {
		if score.Name == scores.HomeTeam {
			fmt.Sscanf(score.Score, "%d", &homeScore)
		} else if score.Name == scores.AwayTeam {
			fmt.Sscanf(score.Score, "%d", &awayScore)
		}
	}

	switch bet.MarketKey {
	case "h2h":
		return s.settleMoneyline(bet, scores, homeScore, awayScore)
	case "spreads":
		return s.settleSpread(bet, scores, homeScore, awayScore)
	case "totals":
		return s.settleTotal(bet, homeScore, awayScore)
	default:
		// Unknown market - mark as void
		return "void", bet.StakeAmount
	}
}

func (s *Settler) settleMoneyline(bet Bet, scores *EventScore, homeScore, awayScore int) (string, float64) {
	var winner string
	if homeScore > awayScore {
		winner = scores.HomeTeam
	} else if awayScore > homeScore {
		winner = scores.AwayTeam
	} else {
		// Tie - moneylines push
		return "push", bet.StakeAmount
	}

	if bet.OutcomeName == winner {
		payout := bet.StakeAmount * americanToDecimal(bet.BetPrice)
		return "win", payout
	}

	return "loss", 0.0
}

func (s *Settler) settleSpread(bet Bet, scores *EventScore, homeScore, awayScore int) (string, float64) {
	if bet.Point == nil {
		return "void", bet.StakeAmount
	}

	spread := *bet.Point
	var adjustedScore float64

	// Determine which team we bet on and apply spread
	if bet.OutcomeName == scores.HomeTeam {
		adjustedScore = float64(homeScore) + spread
		if adjustedScore > float64(awayScore) {
			return "win", bet.StakeAmount * americanToDecimal(bet.BetPrice)
		} else if adjustedScore == float64(awayScore) {
			return "push", bet.StakeAmount
		}
		return "loss", 0.0
	} else {
		adjustedScore = float64(awayScore) + spread
		if adjustedScore > float64(homeScore) {
			return "win", bet.StakeAmount * americanToDecimal(bet.BetPrice)
		} else if adjustedScore == float64(homeScore) {
			return "push", bet.StakeAmount
		}
		return "loss", 0.0
	}
}

func (s *Settler) settleTotal(bet Bet, homeScore, awayScore int) (string, float64) {
	if bet.Point == nil {
		return "void", bet.StakeAmount
	}

	totalPoints := float64(homeScore + awayScore)
	line := *bet.Point

	if bet.OutcomeName == "Over" {
		if totalPoints > line {
			return "win", bet.StakeAmount * americanToDecimal(bet.BetPrice)
		} else if totalPoints == line {
			return "push", bet.StakeAmount
		}
		return "loss", 0.0
	} else { // Under
		if totalPoints < line {
			return "win", bet.StakeAmount * americanToDecimal(bet.BetPrice)
		} else if totalPoints == line {
			return "push", bet.StakeAmount
		}
		return "loss", 0.0
	}
}

func (s *Settler) updateBetResult(ctx context.Context, betID int64, result string, payout float64) error {
	query := `
		UPDATE bets
		SET result = $1, payout_amount = $2, settled_at = NOW()
		WHERE id = $3
	`

	_, err := s.holocronDB.ExecContext(ctx, query, result, payout, betID)
	return err
}

func americanToDecimal(american int) float64 {
	if american > 0 {
		return (float64(american) / 100.0) + 1.0
	}
	return (100.0 / float64(-american)) + 1.0
}


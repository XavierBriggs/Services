package executor

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/bot-service/internal/client"
	"github.com/XavierBriggs/fortuna/services/bot-service/internal/logger"
	"github.com/XavierBriggs/fortuna/services/bot-service/internal/retry"
	"github.com/XavierBriggs/fortuna/services/bot-service/internal/transformer"
	_ "github.com/lib/pq"
)

// Executor orchestrates bet placement
type Executor struct {
	talosClient  *client.TalosClient
	transformer  *transformer.Transformer
	betLogger    *logger.BetLogger
	retryPolicy  *retry.RetryPolicy
	holocronDB   *sql.DB
	alexandriaDB *sql.DB
}

// PlaceBetRequest is the request to place bets (supports both enriched and non-enriched)
type PlaceBetRequest struct {
	OpportunityID int64        `json:"opportunity_id"`
	Legs          []LegRequest `json:"legs"`
	
	// Enriched data (optional - if provided, skips DB reads)
	EventInfo   *EventInfo   `json:"event_info,omitempty"`
	Opportunity *Opportunity `json:"opportunity,omitempty"`
}

// EventInfo contains pre-fetched event and team data
type EventInfo struct {
	EventID       string `json:"event_id"`
	SportKey      string `json:"sport_key"`
	HomeTeam      string `json:"home_team"`
	AwayTeam      string `json:"away_team"`
	HomeTeamShort string `json:"home_team_short"`
	AwayTeamShort string `json:"away_team_short"`
}

// Opportunity contains pre-fetched opportunity data
type Opportunity struct {
	OpportunityType string  `json:"opportunity_type"`
	MarketKey       string  `json:"market_key"`
	EdgePercent     float64 `json:"edge_pct"`
}

// LegRequest contains bet details for a single leg
type LegRequest struct {
	BookKey      string  `json:"book_key"`
	OutcomeName  string  `json:"outcome_name"`
	Stake        float64 `json:"stake"`
	ExpectedOdds int     `json:"expected_odds"`
}

// PlaceBetResponse is the response from bet placement
type PlaceBetResponse struct {
	Success bool
	Results []LegResult
}

// LegResult contains the result for a single leg
type LegResult struct {
	BookKey      string
	Success      bool
	BetID        *int64
	TicketNumber string
	LatencyMs    int64
	Error        string
}

// NewExecutor creates a new executor
func NewExecutor(
	talosClient *client.TalosClient,
	transformer *transformer.Transformer,
	betLogger *logger.BetLogger,
	retryPolicy *retry.RetryPolicy,
	holocronDB *sql.DB,
	alexandriaDB *sql.DB,
) *Executor {
	return &Executor{
		talosClient:  talosClient,
		transformer:  transformer,
		betLogger:    betLogger,
		retryPolicy:  retryPolicy,
		holocronDB:   holocronDB,
		alexandriaDB: alexandriaDB,
	}
}

// PlaceBet executes bet placement for all legs
func (e *Executor) PlaceBet(ctx context.Context, req PlaceBetRequest) (*PlaceBetResponse, error) {
	// Use enriched data if provided, otherwise fetch from DB
	var opportunity *transformer.Opportunity
	var eventInfo *transformer.EventInfo
	
	if req.Opportunity != nil && req.EventInfo != nil {
		// Use enriched data - no DB reads needed!
		opportunity = &transformer.Opportunity{
			ID:              req.OpportunityID,
			OpportunityType: req.Opportunity.OpportunityType,
			SportKey:        req.EventInfo.SportKey,
			EventID:         req.EventInfo.EventID,
			MarketKey:       req.Opportunity.MarketKey,
			EdgePercent:     req.Opportunity.EdgePercent,
		}
		
		eventInfo = &transformer.EventInfo{
			HomeTeam:      req.EventInfo.HomeTeam,
			AwayTeam:      req.EventInfo.AwayTeam,
			EventName:     fmt.Sprintf("%s @ %s", req.EventInfo.AwayTeam, req.EventInfo.HomeTeam),
			HomeTeamShort: req.EventInfo.HomeTeamShort,
			AwayTeamShort: req.EventInfo.AwayTeamShort,
		}
		
		// Still need to fetch legs from DB (they're not in enriched payload)
		legs, err := e.fetchOpportunityLegs(ctx, req.OpportunityID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch opportunity legs: %w", err)
		}
		opportunity.Legs = legs
	} else {
		// Fallback: fetch from DB (backward compatibility)
		var err error
		opportunity, err = e.fetchOpportunity(ctx, req.OpportunityID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch opportunity: %w", err)
		}
		eventInfo = nil // Will be fetched in transformer if needed
	}

	// Process each leg (can be parallelized in future)
	results := []LegResult{}
	for _, legReq := range req.Legs {
		result := e.executeLeg(ctx, opportunity, legReq, eventInfo)
		results = append(results, result)
	}

	success := true
	for _, r := range results {
		if !r.Success {
			success = false
			break
		}
	}

	return &PlaceBetResponse{
		Success: success,
		Results: results,
	}, nil
}

// executeLeg executes a single leg
func (e *Executor) executeLeg(
	ctx context.Context,
	opportunity *transformer.Opportunity,
	legReq LegRequest,
	eventInfo *transformer.EventInfo,
) LegResult {
	startTime := time.Now()

	// Find matching opportunity leg
	oppLeg := e.findLeg(opportunity, legReq.BookKey)
	if oppLeg == nil {
		return LegResult{
			BookKey: legReq.BookKey,
			Success: false,
			Error:   "leg not found in opportunity",
		}
	}

	// Log start
	e.betLogger.LogStart(ctx, opportunity.ID, legReq.BookKey, "manual")

	// Convert executor.LegRequest to transformer.LegRequest
	transformerLegReq := transformer.LegRequest{
		BookKey:     legReq.BookKey,
		OutcomeName: legReq.OutcomeName,
		Stake:       legReq.Stake,
		ExpectedOdds: legReq.ExpectedOdds,
	}

	// Transform to Talos format (pass eventInfo if available to skip DB reads)
	var talosReq *transformer.TalosBetRequest
	var err error
	if eventInfo != nil {
		// Use enriched data - no DB reads in transformer!
		talosReq, err = e.transformer.TransformWithEventInfo(*opportunity, *eventInfo, *oppLeg, transformerLegReq)
	} else {
		// Fallback: transformer will fetch from DB
		talosReq, err = e.transformer.Transform(*opportunity, *oppLeg, transformerLegReq)
	}
	if err != nil {
		latency := time.Since(startTime).Milliseconds()
		e.betLogger.LogFailure(ctx, opportunity.ID, legReq.BookKey, "manual", int(latency), err.Error())
		return LegResult{
			BookKey: legReq.BookKey,
			Success: false,
			Error:   fmt.Sprintf("transformation failed: %v", err),
		}
	}

	// Log the transformed request
	fmt.Printf("[Executor] Transformed request - Original book_key: %s, Mapped book: %s\n", legReq.BookKey, talosReq.Book)

	// Check bot health before attempting (use mapped book name from talosReq)
	healthy, err := e.talosClient.CheckHealth(talosReq.Book)
	if err != nil || !healthy {
		latency := time.Since(startTime).Milliseconds()
		e.betLogger.LogFailure(ctx, opportunity.ID, legReq.BookKey, "manual", int(latency), "bot not available or not logged in")
		return LegResult{
			BookKey: legReq.BookKey,
			Success: false,
			Error:   "bot not available or not logged in",
		}
	}

	// Convert transformer.TalosBetRequest to client.TalosBetRequest
	clientTalosReq := client.TalosBetRequest{
		Book:      talosReq.Book,
		Team1:     talosReq.Team1,
		Team2:     talosReq.Team2,
		BetTeam:   talosReq.BetTeam,
		BetType:   talosReq.BetType,
		BetPeriod: talosReq.BetPeriod,
		BetAmount: talosReq.BetAmount,
		BetOdds:   talosReq.BetOdds,
		Sport:     talosReq.Sport,
		RequestID: talosReq.RequestID,
	}

	// Execute with retry
	var talosResp *client.TalosBetResponse
	var execErr error
	
	err = e.retryPolicy.Execute(func() error {
		talosResp, execErr = e.talosClient.PlaceBet(clientTalosReq)
		return execErr
	})

	if err != nil {
		latency := time.Since(startTime).Milliseconds()
		e.betLogger.LogFailure(ctx, opportunity.ID, legReq.BookKey, "manual", int(latency), err.Error())
		return LegResult{
			BookKey: legReq.BookKey,
			Success: false,
			Error:   err.Error(),
		}
	}

	latency := time.Since(startTime).Milliseconds()

	// Create bet record in Holocron
	betID, err := e.createBetRecord(ctx, opportunity, oppLeg, legReq, latency)
	if err != nil {
		// Log but don't fail - bet was placed successfully
		e.betLogger.LogExecution(ctx, &logger.ExecutionLog{
			OpportunityID:  &opportunity.ID,
			BookKey:        legReq.BookKey,
			TriggerSource:  "manual",
			ExecutionStage: "bet_record_creation",
			Status:         "failed",
			LatencyMs:      int(latency),
			ErrorMessage:   err.Error(),
		})
	}

	// Log successful execution
	if betID != nil {
		e.betLogger.LogSuccess(ctx, *betID, opportunity.ID, legReq.BookKey, "manual", int(latency))
	}

	ticketNumber := ""
	if talosResp.BetDetails != nil {
		if tn, ok := talosResp.BetDetails["ticket_number"].(string); ok {
			ticketNumber = tn
		}
	}

	return LegResult{
		BookKey:      legReq.BookKey,
		Success:      true,
		BetID:        betID,
		TicketNumber: ticketNumber,
		LatencyMs:    latency,
	}
}

// fetchOpportunity fetches opportunity from Holocron
func (e *Executor) fetchOpportunity(ctx context.Context, opportunityID int64) (*transformer.Opportunity, error) {
	// Fetch opportunity
	oppQuery := `
		SELECT id, opportunity_type, sport_key, event_id, market_key, edge_pct
		FROM opportunities
		WHERE id = $1
	`

	var opp transformer.Opportunity
	err := e.holocronDB.QueryRowContext(ctx, oppQuery, opportunityID).Scan(
		&opp.ID,
		&opp.OpportunityType,
		&opp.SportKey,
		&opp.EventID,
		&opp.MarketKey,
		&opp.EdgePercent,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch opportunity: %w", err)
	}

	// Fetch legs
	legsQuery := `
		SELECT book_key, outcome_name, price, point
		FROM opportunity_legs
		WHERE opportunity_id = $1
		ORDER BY id
	`

	rows, err := e.holocronDB.QueryContext(ctx, legsQuery, opportunityID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch legs: %w", err)
	}
	defer rows.Close()

	var legs []transformer.OpportunityLeg
	for rows.Next() {
		var leg transformer.OpportunityLeg
		var point sql.NullFloat64
		err := rows.Scan(&leg.BookKey, &leg.OutcomeName, &leg.Price, &point)
		if err != nil {
			continue
		}
		if point.Valid {
			leg.Point = &point.Float64
		}
		legs = append(legs, leg)
	}

	opp.Legs = legs
	return &opp, nil
}

// fetchOpportunityLegs fetches only the legs for an opportunity
func (e *Executor) fetchOpportunityLegs(ctx context.Context, opportunityID int64) ([]transformer.OpportunityLeg, error) {
	legsQuery := `
		SELECT book_key, outcome_name, price, point
		FROM opportunity_legs
		WHERE opportunity_id = $1
		ORDER BY id
	`

	rows, err := e.holocronDB.QueryContext(ctx, legsQuery, opportunityID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch legs: %w", err)
	}
	defer rows.Close()

	var legs []transformer.OpportunityLeg
	for rows.Next() {
		var leg transformer.OpportunityLeg
		var point sql.NullFloat64
		err := rows.Scan(&leg.BookKey, &leg.OutcomeName, &leg.Price, &point)
		if err != nil {
			continue
		}
		if point.Valid {
			leg.Point = &point.Float64
		}
		legs = append(legs, leg)
	}

	return legs, nil
}

// findLeg finds a leg by book key
func (e *Executor) findLeg(opportunity *transformer.Opportunity, bookKey string) *transformer.OpportunityLeg {
	for _, leg := range opportunity.Legs {
		if leg.BookKey == bookKey {
			return &leg
		}
	}
	return nil
}

// createBetRecord creates a bet record in Holocron
func (e *Executor) createBetRecord(
	ctx context.Context,
	opportunity *transformer.Opportunity,
	leg *transformer.OpportunityLeg,
	legReq LegRequest,
	latencyMs int64,
) (*int64, error) {
	query := `
		INSERT INTO bets (
			opportunity_id, sport_key, event_id, market_key, book_key,
			outcome_name, bet_type, stake_amount, bet_price, point,
			placed_at, execution_method, bot_latency_ms, result
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`

	var betID int64
	var pointValue interface{}
	if leg.Point != nil {
		pointValue = *leg.Point
	} else {
		pointValue = nil
	}

	err := e.holocronDB.QueryRowContext(ctx, query,
		opportunity.ID,
		opportunity.SportKey,
		opportunity.EventID,
		opportunity.MarketKey,
		leg.BookKey,
		leg.OutcomeName,
		mapOppTypeToBetType(opportunity.OpportunityType),
		legReq.Stake,
		leg.Price,
		pointValue,
		time.Now(),
		"automated",
		latencyMs,
		"pending",
	).Scan(&betID)

	if err != nil {
		return nil, err
	}

	return &betID, nil
}

// mapOppTypeToBetType maps opportunity type to bet type
func mapOppTypeToBetType(oppType string) string {
	if oppType == "edge" {
		return "straight"
	}
	return oppType // middle, scalp
}


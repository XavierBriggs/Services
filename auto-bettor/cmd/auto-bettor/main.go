package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/config"
	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/execution"
	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/filters"
	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/logger"
	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/sizing"
	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/state"
)

func main() {
	fmt.Println("🤖 Auto-Bettor Service Starting...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize databases
	holocronDB, err := sql.Open("postgres", cfg.HolocronDSN)
	if err != nil {
		fmt.Printf("Failed to connect to Holocron: %v\n", err)
		os.Exit(1)
	}
	defer holocronDB.Close()

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	defer redisClient.Close()

	// Test connections
	if err := holocronDB.Ping(); err != nil {
		fmt.Printf("Failed to ping Holocron: %v\n", err)
		os.Exit(1)
	}

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		fmt.Printf("Failed to ping Redis: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Database connections established")

	// Initialize components
	stateManager := state.NewStateManager(holocronDB, cfg.SettingsCacheTTL)
	decisionLogger := logger.NewDecisionLogger()
	bankrollManager := sizing.NewBankrollManager(cfg.BotServiceURL, cfg.BalanceCacheTTL)
	kellyCalculator := sizing.NewKellyCalculator(cfg.KellyCalculatorURL)
	scalpSizer := sizing.NewScalpSizer()

	botClient := execution.NewBotServiceClient(cfg.BotServiceURL)
	edgeStrategy := execution.NewEdgeStrategy(botClient)
	middleStrategy := execution.NewMiddleStrategy(botClient)
	scalpStrategy := execution.NewScalpStrategy(botClient)

	// Initialize filters (ordered by cost: cheapest/fastest failures first)
	filterChain := filters.NewFilterChain(
		filters.NewBookHealthFilter(cfg.BotServiceURL), // 1st - HTTP call, but critical (bot must be available)
		filters.NewUserPreferencesFilter(),             // 2nd - memory checks (markets, types)
		filters.NewTimingFilter(),                      // 3rd - time-based checks (game start time)
		filters.NewRiskManagementFilter(),              // 4th - exposure and limit checks
		filters.NewCorrelationFilter(),                 // 5th - most expensive (checks recent bets)
	)

	streamConsumer := consumer.NewStreamConsumer(redisClient, cfg.ConsumerGroup, cfg.ConsumerID)

	fmt.Println("✓ Components initialized")

	// Start consuming
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutting down gracefully...")
		cancel()
	}()

	// Initialize consumer group
	if err := streamConsumer.ConsumeStream(ctx, cfg.StreamKey, cfg.BatchSize, cfg.BlockTime); err != nil {
		fmt.Printf("Failed to initialize consumer: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🚀 Auto-Bettor Service Running")
	fmt.Println("   Stream:", cfg.StreamKey)
	fmt.Println("   Consumer Group:", cfg.ConsumerGroup)
	fmt.Println("   Consumer ID:", cfg.ConsumerID)
	fmt.Println()

	// Main processing loop
	ticker := time.NewTicker(cfg.BlockTime)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("✓ Shutdown complete")
			return

		case <-ticker.C:
			// Read opportunities from stream
			opportunities, err := streamConsumer.ReadMessages(ctx, cfg.StreamKey, int64(cfg.BatchSize), cfg.BlockTime)
			if err != nil {
				fmt.Printf("Error reading messages: %v\n", err)
				continue
			}

			if len(opportunities) == 0 {
				continue
			}

			// Process each opportunity
			for _, opp := range opportunities {
				processOpportunity(
					ctx,
					opp,
					stateManager,
					decisionLogger,
					bankrollManager,
					kellyCalculator,
					scalpSizer,
					filterChain,
					edgeStrategy,
					middleStrategy,
					scalpStrategy,
				)
			}
		}
	}
}

func processOpportunity(
	ctx context.Context,
	opp models.Opportunity,
	stateManager *state.StateManager,
	decisionLogger *logger.DecisionLogger,
	bankrollManager *sizing.BankrollManager,
	kellyCalculator *sizing.KellyCalculator,
	scalpSizer *sizing.ScalpSizer,
	filterChain *filters.FilterChain,
	edgeStrategy *execution.EdgeStrategy,
	middleStrategy *execution.MiddleStrategy,
	scalpStrategy *execution.ScalpStrategy,
) {
	startTime := time.Now()
	decisionLogger.LogOpportunityReceived(opp)

	// 1. Load user settings
	settings, err := stateManager.GetUserSettings(ctx, "default")
	if err != nil {
		decisionLogger.LogError(opp, "load_settings", err)
		return
	}

	// 2. Check if auto-betting is enabled
	if !settings.AutoBettingEnabled {
		decisionLogger.LogSkipped(opp, "auto-betting disabled")
		return
	}

	// 3. Check opportunity type specific toggles
	if opp.OpportunityType == "middle" && !settings.AutoMiddleEnabled {
		decisionLogger.LogSkipped(opp, "middle betting disabled")
		return
	}

	if opp.OpportunityType == "scalp" && !settings.AutoScalpEnabled {
		decisionLogger.LogSkipped(opp, "scalp betting disabled")
		return
	}

	// 3a. Early book availability check - avoid processing opportunities for unavailable books
	for _, leg := range opp.Legs {
		bookKey := leg.BookKey

		// Check if book is explicitly disabled
		for _, disabledBook := range settings.AutoDisabledBooks {
			if disabledBook == bookKey {
				decisionLogger.LogSkipped(opp, fmt.Sprintf("book '%s' is disabled", bookKey))
				return
			}
		}

		// If enabled books list is configured, check if this book is in it
		if len(settings.AutoEnabledBooks) > 0 {
			bookEnabled := false
			for _, enabledBook := range settings.AutoEnabledBooks {
				if enabledBook == bookKey {
					bookEnabled = true
					break
				}
			}
			if !bookEnabled {
				decisionLogger.LogSkipped(opp, fmt.Sprintf("book '%s' not in enabled books list", bookKey))
				return
			}
		}
	}

	// 4. Load current state
	autoBettingState, err := stateManager.GetState(ctx, "default")
	if err != nil {
		decisionLogger.LogError(opp, "load_state", err)
		return
	}

	// 5. Check circuit breakers
	if autoBettingState.IsPaused {
		decisionLogger.LogSkipped(opp, fmt.Sprintf("auto-betting paused: %s", autoBettingState.PauseReason))
		return
	}

	// 6. Run filter chain
	passed, filterResults := filterChain.Evaluate(opp, *settings, *autoBettingState)

	// Log filter results
	for name, result := range filterResults {
		decisionLogger.LogFilterResult(name, result)
	}

	if !passed {
		// Log skipped decision
		for name, result := range filterResults {
			if !result.Passed {
				stateManager.LogDecision(ctx, &models.AutoBettingDecision{
					UserID:          "default",
					OpportunityID:   opp.ID,
					Decision:        "skipped_filter",
					DecisionReason:  fmt.Sprintf("%s: %s", name, result.Reason),
					FilterResults:   filterResults,
					CurrentExposure: autoBettingState.TotalExposure,
					BetsPlacedToday: autoBettingState.BetsPlacedToday,
					CreatedAt:       time.Now(),
				})
				return
			}
		}
	}

	// 7. Get live bankroll from bot
	if len(opp.Legs) == 0 {
		decisionLogger.LogError(opp, "no_legs", fmt.Errorf("opportunity has no legs"))
		return
	}

	bankroll, err := bankrollManager.GetBankroll(opp.Legs[0].BookKey)
	if err != nil {
		decisionLogger.LogError(opp, "fetch_bankroll", err)
		stateManager.LogDecision(ctx, &models.AutoBettingDecision{
			UserID:          "default",
			OpportunityID:   opp.ID,
			Decision:        "error",
			DecisionReason:  fmt.Sprintf("failed to fetch bankroll: %v", err),
			FilterResults:   filterResults,
			CurrentExposure: autoBettingState.TotalExposure,
			BetsPlacedToday: autoBettingState.BetsPlacedToday,
			CreatedAt:       time.Now(),
		})
		return
	}

	// 8. Calculate position size
	var stakes map[string]float64
	var kellyResp *sizing.KellyResponse

	// Get correlation discount from filter results
	correlationDiscount := 0.0
	if corrResult, exists := filterResults["correlation"]; exists {
		if isCorr, ok := corrResult.Metadata["is_correlated"].(bool); ok && isCorr {
			if discount, ok := corrResult.Metadata["correlation_discount"].(float64); ok {
				correlationDiscount = discount
			}
		}
	}

	if opp.OpportunityType == "scalp" {
		// Calculate scalp stakes
		totalStake := bankroll.Balance * (settings.AutoScalpBankrollPct / 100.0)
		stakes, err = scalpSizer.CalculateStakes(opp, totalStake)
		if err != nil {
			decisionLogger.LogError(opp, "scalp_sizing", err)
			return
		}
	} else {
		// Use Kelly Calculator
		kellyResp, err = kellyCalculator.CalculateStake(opp, bankroll.Balance, settings.KellyFraction, *settings, correlationDiscount)
		if err != nil {
			decisionLogger.LogError(opp, "kelly_calculation", err)
			return
		}
		stakes = kellyResp.Stakes
	}

	// 9. Select execution strategy
	var strategy execution.Strategy
	switch opp.OpportunityType {
	case "edge":
		strategy = edgeStrategy
	case "middle":
		strategy = middleStrategy
	case "scalp":
		strategy = scalpStrategy
	default:
		decisionLogger.LogError(opp, "unknown_type", fmt.Errorf("unknown opportunity type: %s", opp.OpportunityType))
		return
	}

	// 10. Create execution plan
	plan, err := strategy.Plan(opp, *settings, *autoBettingState, bankroll.Balance, stakes)
	if err != nil {
		decisionLogger.LogError(opp, "planning", err)
		return
	}

	decisionLogger.LogExecutionPlan(plan)

	// 11. Execute
	result, err := strategy.Execute(plan)
	if err != nil {
		decisionLogger.LogError(opp, "execution", err)
		return
	}

	decisionLogger.LogExecutionResult(result)

	// 12. Handle rollback if needed
	if result.RollbackNeeded {
		if err := strategy.Rollback(plan, result); err != nil {
			decisionLogger.LogError(opp, "rollback", err)
		}
	}

	// 13. Update state for placed bets
	for _, legResult := range result.LegsPlaced {
		if err := stateManager.RecordBetPlaced(ctx, "default", legResult.Stake, opp.EventID, legResult.BookKey); err != nil {
			fmt.Printf("Failed to update state: %v\n", err)
		}

		// Invalidate balance cache
		bankrollManager.InvalidateCache(legResult.BookKey)
	}

	// 14. Log decision
	betIDs := make([]int64, 0)
	for _, leg := range result.LegsPlaced {
		if leg.BetID != nil {
			betIDs = append(betIDs, *leg.BetID)
		}
	}

	decision := "placed"
	if result.Status == "failed" {
		decision = "error"
	} else if result.Status == "partial" {
		decision = "partial"
	}

	executionTimeMs := int(result.TotalDuration.Milliseconds())
	calculatedStake := plan.TotalStake
	calculatedEdge := opp.EdgePct

	stateManager.LogDecision(ctx, &models.AutoBettingDecision{
		UserID:            "default",
		OpportunityID:     opp.ID,
		Decision:          decision,
		DecisionReason:    fmt.Sprintf("executed with status: %s", result.Status),
		FilterResults:     filterResults,
		CalculatedStake:   &calculatedStake,
		CalculatedEdge:    &calculatedEdge,
		KellyFractionUsed: &settings.KellyFraction,
		FullKellyPct:      nil, // From Kelly response if available
		BetIDs:            betIDs,
		ExecutionTimeMs:   &executionTimeMs,
		CurrentExposure:   autoBettingState.TotalExposure,
		CurrentBankroll:   bankroll.Balance,
		BetsPlacedToday:   autoBettingState.BetsPlacedToday,
		CreatedAt:         time.Now(),
	})

	totalTime := time.Since(startTime)
	fmt.Printf("✓ Completed in %v\n\n", totalTime)
}

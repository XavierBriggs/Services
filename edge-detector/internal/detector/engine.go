package detector

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/XavierBriggs/fortuna/services/edge-detector/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/edge-detector/internal/publisher"
	"github.com/XavierBriggs/fortuna/services/edge-detector/internal/writer"
	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/contracts"
	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/models"
)

// Deduplication default settings
// - Cooldown aligns with auto-bettor max_data_age (30s) and Mercury poll (40-60s)
// - Re-emit on significant price/edge change even within cooldown
// - All configurable via environment variables
var (
	dedupCooldown    = getDurationEnv("DEDUP_COOLDOWN", 30*time.Second) // Re-emit after cooldown
	dedupPriceChange = getIntEnv("DEDUP_PRICE_CHANGE", 3)               // Re-emit if price changes by ≥N cents
	dedupEdgeChange  = getFloatEnv("DEDUP_EDGE_CHANGE", 0.5)            // Re-emit if edge changes by ≥N%
	dedupCacheTTL    = getDurationEnv("DEDUP_CACHE_TTL", 5*time.Minute) // Clean up cache entries after this
)

// CachedOpportunity tracks the last emitted state of an opportunity
type CachedOpportunity struct {
	OpportunityID int64     // Database ID of the opportunity
	LastPrice     int       // Last emitted price (first leg)
	LastEdgePct   float64   // Last emitted edge percentage
	LastEmittedAt time.Time // When it was last published to stream
	FirstSeenAt   time.Time // When first detected (for finalization)
}

// Engine orchestrates opportunity detection
type Engine struct {
	consumer          *consumer.StreamConsumer
	holocronWriter    *writer.HolocronWriter
	streamPublisher   *publisher.StreamPublisher
	sharpBookProvider contracts.SharpBookProvider
	config            contracts.DetectorConfig

	// Detectors
	edgeDetector   contracts.OpportunityDetector
	middleDetector contracts.OpportunityDetector
	scalpDetector  contracts.OpportunityDetector

	// Market cache for grouping odds
	marketCache sync.Map // key: "eventID:marketKey" -> []models.NormalizedOdds

	// Deduplication cache
	dedupCache sync.Map // key: opportunity signature -> CachedOpportunity

	// Metrics
	detectedCount      int64
	skippedCount       int64 // Deduplicated opportunities
	errorCount         int64
	totalLatencyMs     int64 // Cumulative latency in milliseconds
	detectionLatencyMs int64 // Cumulative detection-only latency
	mu                 sync.Mutex
}

// NewEngine creates a new detection engine
func NewEngine(
	consumer *consumer.StreamConsumer,
	holocronWriter *writer.HolocronWriter,
	streamPublisher *publisher.StreamPublisher,
	sharpBookProvider contracts.SharpBookProvider,
	config contracts.DetectorConfig,
) *Engine {
	e := &Engine{
		consumer:          consumer,
		holocronWriter:    holocronWriter,
		streamPublisher:   streamPublisher,
		sharpBookProvider: sharpBookProvider,
		config:            config,
		edgeDetector:      NewEdgeDetector(config, sharpBookProvider),
		middleDetector:    NewMiddleDetector(config, sharpBookProvider),
		scalpDetector:     NewScalpDetector(config),
	}

	return e
}

// Start begins processing normalized odds for a sport
func (e *Engine) Start(ctx context.Context, sportKey string) error {
	streamKey := fmt.Sprintf("odds.normalized.%s", sportKey)

	fmt.Printf("✓ Starting detection engine for stream: %s\n", streamKey)

	// Subscribe to normalized odds stream
	messageCh, errorCh := e.consumer.ConsumeStream(ctx, streamKey)

	for {
		select {
		case <-ctx.Done():
			return nil

		case err := <-errorCh:
			if err != nil {
				fmt.Printf("stream error: %v\n", err)
			}

		case msg, ok := <-messageCh:
			if !ok {
				return nil
			}

			// Process the message
			if err := e.processMessage(ctx, msg); err != nil {
				fmt.Printf("error processing message %s: %v\n", msg.ID, err)
				e.incrementErrorCount()
			}

			// Acknowledge the message
			if err := e.consumer.AckMessage(ctx, msg.StreamKey, msg.ID); err != nil {
				fmt.Printf("error acknowledging message %s: %v\n", msg.ID, err)
			}
		}
	}
}

// processMessage processes a single normalized odds message
func (e *Engine) processMessage(ctx context.Context, msg consumer.Message) error {
	startTime := time.Now()
	odds := msg.NormalizedOdds

	// Update market cache with this odds
	e.updateMarketCache(odds)

	// Get all odds for this market
	marketOdds := e.getMarketOdds(odds)

	// Run all enabled detectors
	detectionStart := time.Now()
	allOpportunities := make([]models.Opportunity, 0)

	// 1. Edge detector (always enabled)
	if opportunities, err := e.edgeDetector.Detect(ctx, odds, marketOdds); err == nil {
		allOpportunities = append(allOpportunities, opportunities...)
	} else {
		fmt.Printf("edge detector error: %v\n", err)
	}

	// 2. Middle detector (if enabled)
	if e.middleDetector.IsEnabled() {
		if opportunities, err := e.middleDetector.Detect(ctx, odds, marketOdds); err == nil {
			allOpportunities = append(allOpportunities, opportunities...)
		} else {
			fmt.Printf("middle detector error: %v\n", err)
		}
	}

	// 3. Scalp detector (if enabled)
	if e.scalpDetector.IsEnabled() {
		if opportunities, err := e.scalpDetector.Detect(ctx, odds, marketOdds); err == nil {
			allOpportunities = append(allOpportunities, opportunities...)
		} else {
			fmt.Printf("scalp detector error: %v\n", err)
		}
	}

	// Process detected opportunities
	for _, opportunity := range allOpportunities {
		emitted, err := e.processOpportunity(ctx, opportunity)
		if err != nil {
			fmt.Printf("error processing opportunity: %v\n", err)
			continue
		}

		if emitted {
			e.incrementDetectedCount()

			// Calculate latencies
			detectionLatency := time.Since(detectionStart).Milliseconds()
			totalLatency := time.Since(startTime).Milliseconds()

			e.recordLatency(totalLatency, detectionLatency)

			fmt.Printf("✓ Detected %s opportunity: event=%s market=%s edge=%.2f%% (detection=%dms, total=%dms)\n",
				opportunity.OpportunityType, opportunity.EventID, opportunity.MarketKey,
				opportunity.EdgePercent, detectionLatency, totalLatency)
		}
	}

	return nil
}

// processOpportunity handles DB writes and stream publishing separately:
// - DB: Insert once (new opportunity) or update last_seen (existing)
// - Stream: Publish based on cooldown/price change (for real-time consumers)
// Returns (published, error) - published=true if sent to stream
func (e *Engine) processOpportunity(ctx context.Context, opportunity models.Opportunity) (bool, error) {
	signature := e.buildOpportunitySignature(opportunity)

	// Step 1: DB write - upsert (insert if new, update last_seen if exists)
	opportunityID, isNew, err := e.holocronWriter.UpsertOpportunity(ctx, opportunity, signature)
	if err != nil {
		return false, fmt.Errorf("failed to upsert to Holocron: %w", err)
	}

	// Update opportunity with database ID
	opportunity.ID = opportunityID

	// Step 2: Check if we should publish to stream (separate from DB logic)
	shouldPublish, cached := e.shouldPublishToStream(opportunity, signature)

	if !shouldPublish {
		e.incrementSkippedCount()
		return false, nil
	}

	// Step 3: Publish to stream
	if err := e.streamPublisher.Publish(ctx, opportunity); err != nil {
		return false, fmt.Errorf("failed to publish to stream: %w", err)
	}

	// Step 4: Update cache with new publish time
	e.updateStreamCache(opportunity, signature, opportunityID, isNew, cached)

	return true, nil
}

// buildOpportunitySignature creates a unique key for an opportunity
// For edge opportunities: "edge:eventID:marketKey:bookKey:outcomeName"
// For scalp/middle: "scalp:eventID:marketKey:book1:book2:outcome1:outcome2"
func (e *Engine) buildOpportunitySignature(opp models.Opportunity) string {
	parts := []string{
		string(opp.OpportunityType),
		opp.EventID,
		opp.MarketKey,
	}

	// Sort legs by book key to ensure consistent signature regardless of leg order
	legKeys := make([]string, 0, len(opp.Legs))
	for _, leg := range opp.Legs {
		legKeys = append(legKeys, fmt.Sprintf("%s:%s", leg.BookKey, leg.OutcomeName))
	}
	sort.Strings(legKeys)
	parts = append(parts, legKeys...)

	return strings.Join(parts, "|")
}

// shouldPublishToStream checks if an opportunity should be published to stream
// Returns (shouldPublish, cachedOpportunity)
// Stream publishing uses cooldown to give auto-bettor fresh data
func (e *Engine) shouldPublishToStream(opp models.Opportunity, signature string) (bool, *CachedOpportunity) {
	// Check if we've published this opportunity before
	cached, exists := e.dedupCache.Load(signature)
	if !exists {
		// First time seeing this opportunity - publish it
		return true, nil
	}

	cachedOpp, ok := cached.(CachedOpportunity)
	if !ok {
		// Cache corruption - publish to be safe
		return true, nil
	}

	// Get current price (from first leg)
	currentPrice := 0
	if len(opp.Legs) > 0 {
		currentPrice = opp.Legs[0].Price
	}

	// Check cooldown - re-publish if enough time has passed
	if time.Since(cachedOpp.LastEmittedAt) >= dedupCooldown {
		return true, &cachedOpp
	}

	// Check price change - re-publish if price moved significantly
	priceDiff := abs(currentPrice - cachedOpp.LastPrice)
	if priceDiff >= dedupPriceChange {
		return true, &cachedOpp
	}

	// Check edge change - re-publish if edge changed significantly
	edgeDiff := math.Abs(opp.EdgePercent - cachedOpp.LastEdgePct)
	if edgeDiff >= dedupEdgeChange {
		return true, &cachedOpp
	}

	// Within cooldown and no significant change - skip stream publish
	return false, &cachedOpp
}

// updateStreamCache updates the cache after publishing to stream
func (e *Engine) updateStreamCache(opp models.Opportunity, signature string, opportunityID int64, isNew bool, existing *CachedOpportunity) {
	// Get price from first leg
	price := 0
	if len(opp.Legs) > 0 {
		price = opp.Legs[0].Price
	}

	now := time.Now()

	cached := CachedOpportunity{
		OpportunityID: opportunityID,
		LastPrice:     price,
		LastEdgePct:   opp.EdgePercent,
		LastEmittedAt: now,
		FirstSeenAt:   now,
	}

	// Preserve FirstSeenAt if this is an existing opportunity
	if existing != nil && !existing.FirstSeenAt.IsZero() {
		cached.FirstSeenAt = existing.FirstSeenAt
	}

	e.dedupCache.Store(signature, cached)

	// Schedule cache cleanup and opportunity finalization after TTL
	go func(sig string, oppID int64) {
		time.Sleep(dedupCacheTTL)

		// Check if this signature is still in cache and not updated
		if val, ok := e.dedupCache.Load(sig); ok {
			cachedOpp := val.(CachedOpportunity)
			// Only delete and finalize if this is the same opportunity
			if cachedOpp.OpportunityID == oppID {
				e.dedupCache.Delete(sig)
				// Finalize the opportunity in DB (set duration)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = e.holocronWriter.FinalizeOpportunity(ctx, oppID)
			}
		}
	}(signature, opportunityID)
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// getMarketOdds retrieves all odds for the same event+market from cache
func (e *Engine) getMarketOdds(odds models.NormalizedOdds) []models.NormalizedOdds {
	cacheKey := e.buildMarketKey(odds)

	if value, ok := e.marketCache.Load(cacheKey); ok {
		if cached, ok := value.([]models.NormalizedOdds); ok {
			return cached
		}
	}

	return []models.NormalizedOdds{}
}

// updateMarketCache adds or updates odds in the market cache
func (e *Engine) updateMarketCache(odds models.NormalizedOdds) {
	cacheKey := e.buildMarketKey(odds)

	// Load existing odds
	var marketOdds []models.NormalizedOdds
	if value, ok := e.marketCache.Load(cacheKey); ok {
		if cached, ok := value.([]models.NormalizedOdds); ok {
			marketOdds = cached
		}
	}

	// Update or append this odds
	found := false
	for i, existingOdds := range marketOdds {
		if existingOdds.BookKey == odds.BookKey && existingOdds.OutcomeName == odds.OutcomeName {
			// Update existing
			marketOdds[i] = odds
			found = true
			break
		}
	}

	if !found {
		// Append new
		marketOdds = append(marketOdds, odds)
	}

	// Store back in cache
	e.marketCache.Store(cacheKey, marketOdds)

	// Schedule cache cleanup (remove stale entries after 5 minutes)
	go func(key string) {
		time.Sleep(5 * time.Minute)
		e.marketCache.Delete(key)
	}(cacheKey)
}

// buildMarketKey creates a cache key for event+market
func (e *Engine) buildMarketKey(odds models.NormalizedOdds) string {
	return fmt.Sprintf("%s:%s", odds.EventID, odds.MarketKey)
}

// incrementDetectedCount increments the detected opportunities counter
func (e *Engine) incrementDetectedCount() {
	e.mu.Lock()
	e.detectedCount++
	e.mu.Unlock()
}

// incrementSkippedCount increments the deduplicated opportunities counter
func (e *Engine) incrementSkippedCount() {
	e.mu.Lock()
	e.skippedCount++
	e.mu.Unlock()
}

// incrementErrorCount increments the error counter
func (e *Engine) incrementErrorCount() {
	e.mu.Lock()
	e.errorCount++
	e.mu.Unlock()
}

// recordLatency records latency metrics
func (e *Engine) recordLatency(totalMs, detectionMs int64) {
	e.mu.Lock()
	e.totalLatencyMs += totalMs
	e.detectionLatencyMs += detectionMs
	e.mu.Unlock()
}

// GetMetrics returns current detection metrics
func (e *Engine) GetMetrics() (detected, skipped, errors int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.detectedCount, e.skippedCount, e.errorCount
}

// GetLatencyMetrics returns latency statistics
func (e *Engine) GetLatencyMetrics() (avgTotalMs, avgDetectionMs float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.detectedCount == 0 {
		return 0, 0
	}

	avgTotalMs = float64(e.totalLatencyMs) / float64(e.detectedCount)
	avgDetectionMs = float64(e.detectionLatencyMs) / float64(e.detectedCount)
	return
}

// Environment variable helpers for deduplication config
func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func getIntEnv(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getFloatEnv(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

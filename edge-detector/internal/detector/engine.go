package detector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/XavierBriggs/fortuna/services/edge-detector/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/edge-detector/internal/publisher"
	"github.com/XavierBriggs/fortuna/services/edge-detector/internal/writer"
	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/contracts"
	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/models"
)

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

	// Metrics
	detectedCount      int64
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
		if err := e.processOpportunity(ctx, opportunity); err != nil {
			fmt.Printf("error processing opportunity: %v\n", err)
			continue
		}

		e.incrementDetectedCount()
		
		// Calculate latencies
		detectionLatency := time.Since(detectionStart).Milliseconds()
		totalLatency := time.Since(startTime).Milliseconds()
		
		e.recordLatency(totalLatency, detectionLatency)
		
		fmt.Printf("✓ Detected %s opportunity: event=%s market=%s edge=%.2f%% (detection=%dms, total=%dms)\n",
			opportunity.OpportunityType, opportunity.EventID, opportunity.MarketKey, 
			opportunity.EdgePercent, detectionLatency, totalLatency)
	}

	return nil
}

// processOpportunity writes an opportunity to Holocron and publishes to stream
func (e *Engine) processOpportunity(ctx context.Context, opportunity models.Opportunity) error {
	// Write to Holocron
	opportunityID, err := e.holocronWriter.WriteOpportunity(ctx, opportunity)
	if err != nil {
		return fmt.Errorf("failed to write to Holocron: %w", err)
	}

	// Update opportunity with database ID
	opportunity.ID = opportunityID

	// Publish to stream
	if err := e.streamPublisher.Publish(ctx, opportunity); err != nil {
		return fmt.Errorf("failed to publish to stream: %w", err)
	}

	return nil
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
func (e *Engine) GetMetrics() (detected, errors int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.detectedCount, e.errorCount
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


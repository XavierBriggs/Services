package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/XavierBriggs/fortuna/services/normalizer/internal/consumer"
	"github.com/XavierBriggs/fortuna/services/normalizer/internal/publisher"
	"github.com/XavierBriggs/fortuna/services/normalizer/internal/registry"
	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/models"
)

// Processor orchestrates normalization of raw odds
type Processor struct {
	consumer  *consumer.StreamConsumer
	publisher *publisher.StreamPublisher
	registry  *registry.NormalizerRegistry
	
	// Market state cache for grouping odds by event+market
	marketCache sync.Map // key: "eventID:marketKey" -> []models.RawOdds
	
	// Metrics
	processedCount int64
	errorCount     int64
	mu             sync.Mutex
}

// NewProcessor creates a new processor
func NewProcessor(
	consumer *consumer.StreamConsumer,
	publisher *publisher.StreamPublisher,
	registry *registry.NormalizerRegistry,
) *Processor {
	return &Processor{
		consumer:  consumer,
		publisher: publisher,
		registry:  registry,
	}
}

// Start begins processing messages from all registered sports
func (p *Processor) Start(ctx context.Context) error {
	// Start processing for each registered sport
	normalizers := p.registry.GetAll()
	if len(normalizers) == 0 {
		return fmt.Errorf("no sport normalizers registered")
	}

	var wg sync.WaitGroup

	for _, norm := range normalizers {
		wg.Add(1)
		go func(norm interface{ GetSportKey() string }) {
			defer wg.Done()
			streamKey := fmt.Sprintf("odds.raw.%s", norm.GetSportKey())
			p.processStream(ctx, streamKey)
		}(norm)
	}

	wg.Wait()
	return nil
}

// processStream processes messages from a single stream
func (p *Processor) processStream(ctx context.Context, streamKey string) {
	fmt.Printf("✓ Started processing stream: %s\n", streamKey)

	messageCh, errorCh := p.consumer.ConsumeStream(ctx, streamKey)

	for {
		select {
		case <-ctx.Done():
			return

		case err := <-errorCh:
			if err != nil {
				fmt.Printf("stream error: %v\n", err)
			}

		case msg, ok := <-messageCh:
			if !ok {
				return
			}

			// Process the message
			if err := p.processMessage(ctx, msg); err != nil {
				fmt.Printf("error processing message %s: %v\n", msg.ID, err)
				p.incrementErrorCount()
			} else {
				p.incrementProcessedCount()
			}

			// Acknowledge the message
			if err := p.consumer.AckMessage(ctx, msg.StreamKey, msg.ID); err != nil {
				fmt.Printf("error acknowledging message %s: %v\n", msg.ID, err)
			}
		}
	}
}

// processMessage processes a single raw odds message
func (p *Processor) processMessage(ctx context.Context, msg consumer.Message) error {
	// DEBUG: Print what we received
	fmt.Printf("PROCESSOR DEBUG %s: sport_key='%s' (len=%d) event_id='%s' book='%s'\n",
		msg.ID, msg.RawOdds.SportKey, len(msg.RawOdds.SportKey), msg.RawOdds.EventID, msg.RawOdds.BookKey)
	
	// Get the appropriate normalizer for this sport
	normalizer, exists := p.registry.Get(msg.RawOdds.SportKey)
	if !exists {
		return fmt.Errorf("no normalizer registered for sport: %s", msg.RawOdds.SportKey)
	}

	// Get market context (all odds for this event+market)
	marketOdds := p.getMarketOdds(msg.RawOdds)

	// Normalize the odds
	normalized, err := normalizer.Normalize(ctx, msg.RawOdds, marketOdds)
	if err != nil {
		return fmt.Errorf("normalization error: %w", err)
	}

	// Update market cache with this odds
	p.updateMarketCache(msg.RawOdds)

	// Publish normalized odds
	if err := p.publisher.Publish(ctx, normalized); err != nil {
		return fmt.Errorf("publish error: %w", err)
	}

	return nil
}

// getMarketOdds retrieves all odds for the same event+market from cache
func (p *Processor) getMarketOdds(odds models.RawOdds) []models.RawOdds {
	cacheKey := p.buildMarketKey(odds)
	
	if value, ok := p.marketCache.Load(cacheKey); ok {
		if cached, ok := value.([]models.RawOdds); ok {
			return cached
		}
	}
	
	return []models.RawOdds{}
}

// updateMarketCache adds or updates odds in the market cache
func (p *Processor) updateMarketCache(odds models.RawOdds) {
	cacheKey := p.buildMarketKey(odds)
	
	// Load existing odds
	var marketOdds []models.RawOdds
	if value, ok := p.marketCache.Load(cacheKey); ok {
		if cached, ok := value.([]models.RawOdds); ok {
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
	p.marketCache.Store(cacheKey, marketOdds)
	
	// Schedule cache cleanup (remove stale entries after 5 minutes)
	go func(key string) {
		time.Sleep(5 * time.Minute)
		p.marketCache.Delete(key)
	}(cacheKey)
}

// buildMarketKey creates a cache key for event+market
func (p *Processor) buildMarketKey(odds models.RawOdds) string {
	return fmt.Sprintf("%s:%s", odds.EventID, odds.MarketKey)
}

// incrementProcessedCount increments the processed message counter
func (p *Processor) incrementProcessedCount() {
	p.mu.Lock()
	p.processedCount++
	p.mu.Unlock()
}

// incrementErrorCount increments the error counter
func (p *Processor) incrementErrorCount() {
	p.mu.Lock()
	p.errorCount++
	p.mu.Unlock()
}

// GetMetrics returns current processing metrics
func (p *Processor) GetMetrics() (processed, errors int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.processedCount, p.errorCount
}


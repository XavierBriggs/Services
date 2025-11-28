package aggregator

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/XavierBriggs/fortuna/services/opportunity-analytics/pkg/models"
)

// BucketKey uniquely identifies a stats bucket
type BucketKey struct {
	TimestampBucket time.Time
	BookKey         string
	OpportunityType string
	GameStatus      string // "pregame" or "live"
	SportKey        string // Sport identifier
	MarketKey       string // Market type
}

// BookPairKey uniquely identifies a book pair bucket (for scalps/middles)
type BookPairKey struct {
	TimestampBucket time.Time
	BookKey1        string // First book (alphabetically sorted)
	BookKey2        string // Second book (alphabetically sorted)
	OpportunityType string
	GameStatus      string
	SportKey        string
	MarketKey       string
}

// BucketStats holds the aggregated statistics for a bucket
type BucketStats struct {
	OpportunityCount int
	TotalEdgePct     float64 // Sum for calculating average

	// Edge distribution tracking (Phase 3)
	EdgeList []float64 // Store all edges for distribution calculation

	// Hold time tracking (Phase 2) - using data_age as proxy
	TotalDataAgeSeconds int // Sum for averaging
	DataAgeCount        int // Count of opps with data age

	// Stale data tracking
	StaleDataCount int // Count of opportunities with stale data (>30s)

	// Edge threshold counts
	Edge5PctCount  int // Edges >= 5%
	Edge10PctCount int // Edges >= 10%
	Edge20PctCount int // Edges >= 20%

	// Tracking first and last for velocity calculation
	FirstOpportunityAt time.Time
	LastOpportunityAt  time.Time

	// Deduplication: track unique opportunity IDs
	SeenIDs map[int64]bool
}

// BookPairStats holds aggregated statistics for book pairs (scalps/middles)
type BookPairStats struct {
	OpportunityCount int
	TotalEdgePct     float64
	EdgeList         []float64

	// Hold time tracking
	TotalDataAgeSeconds int
	DataAgeCount        int

	// Tracking first and last
	FirstOpportunityAt time.Time
	LastOpportunityAt  time.Time

	// Deduplication: track unique opportunity IDs
	SeenIDs map[int64]bool
}

// Aggregator aggregates opportunity statistics in memory
type Aggregator struct {
	mu               sync.RWMutex
	buckets          map[BucketKey]*BucketStats
	bookPairBuckets  map[BookPairKey]*BookPairStats // NEW: Track book pairs
	bucketResolution time.Duration
	excludeLive      bool // Whether to exclude live games
}

// NewAggregator creates a new aggregator
func NewAggregator(bucketResolution time.Duration, excludeLive bool) *Aggregator {
	return &Aggregator{
		buckets:          make(map[BucketKey]*BucketStats),
		bookPairBuckets:  make(map[BookPairKey]*BookPairStats),
		bucketResolution: bucketResolution,
		excludeLive:      excludeLive,
	}
}

// sortBookPair returns books in alphabetical order for consistent key generation
func sortBookPair(book1, book2 string) (string, string) {
	if book1 < book2 {
		return book1, book2
	}
	return book2, book1
}

// ProcessOpportunity processes an opportunity and updates aggregated stats
// gameStatus should be "pregame" or "live"
func (a *Aggregator) ProcessOpportunity(opp models.Opportunity, gameStatus string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Skip live games if configured to exclude them
	if a.excludeLive && gameStatus == "live" {
		return
	}

	// Round timestamp to bucket
	bucket := a.roundToBucket(opp.DetectedAt)

	// Process each leg (for middles/scalps with multiple books)
	for _, leg := range opp.Legs {
		key := BucketKey{
			TimestampBucket: bucket,
			BookKey:         leg.BookKey,
			OpportunityType: opp.OpportunityType,
			GameStatus:      gameStatus,
			SportKey:        opp.SportKey,
			MarketKey:       opp.MarketKey,
		}

		stats, exists := a.buckets[key]
		if !exists {
			stats = &BucketStats{
				EdgeList:           make([]float64, 0, 100), // Pre-allocate
				FirstOpportunityAt: opp.DetectedAt,
				SeenIDs:            make(map[int64]bool),
			}
			a.buckets[key] = stats
		}

		// Skip if we've already counted this opportunity (deduplication)
		if stats.SeenIDs == nil {
			stats.SeenIDs = make(map[int64]bool)
		}
		if stats.SeenIDs[opp.ID] {
			continue
		}
		stats.SeenIDs[opp.ID] = true

		// Update core counts
		stats.OpportunityCount++
		stats.TotalEdgePct += opp.EdgePercent

		// Track edge for distribution (Phase 3)
		stats.EdgeList = append(stats.EdgeList, opp.EdgePercent)

		// Track data age for hold time proxy (Phase 2)
		if opp.DataAgeSeconds > 0 {
			stats.TotalDataAgeSeconds += opp.DataAgeSeconds
			stats.DataAgeCount++
		}

		// Track stale data (>30 seconds)
		if opp.DataAgeSeconds > 30 {
			stats.StaleDataCount++
		}

		// Track edge threshold hits
		if opp.EdgePercent >= 5.0 {
			stats.Edge5PctCount++
		}
		if opp.EdgePercent >= 10.0 {
			stats.Edge10PctCount++
		}
		if opp.EdgePercent >= 20.0 {
			stats.Edge20PctCount++
		}

		// Update last opportunity time
		stats.LastOpportunityAt = opp.DetectedAt
	}

	// For scalps and middles, also track book pairs
	if (opp.OpportunityType == "scalp" || opp.OpportunityType == "middle") && len(opp.Legs) >= 2 {
		a.processBookPair(opp, gameStatus, bucket)
	}
}

// processBookPair tracks statistics for book pairs in scalps/middles
func (a *Aggregator) processBookPair(opp models.Opportunity, gameStatus string, bucket time.Time) {
	// Extract unique books from legs
	bookSet := make(map[string]bool)
	for _, leg := range opp.Legs {
		bookSet[leg.BookKey] = true
	}

	// Convert to slice
	books := make([]string, 0, len(bookSet))
	for book := range bookSet {
		books = append(books, book)
	}

	// For each pair of books, track the pair
	for i := 0; i < len(books); i++ {
		for j := i + 1; j < len(books); j++ {
			book1, book2 := sortBookPair(books[i], books[j])

			pairKey := BookPairKey{
				TimestampBucket: bucket,
				BookKey1:        book1,
				BookKey2:        book2,
				OpportunityType: opp.OpportunityType,
				GameStatus:      gameStatus,
				SportKey:        opp.SportKey,
				MarketKey:       opp.MarketKey,
			}

			pairStats, exists := a.bookPairBuckets[pairKey]
			if !exists {
				pairStats = &BookPairStats{
					EdgeList:           make([]float64, 0, 100),
					FirstOpportunityAt: opp.DetectedAt,
					SeenIDs:            make(map[int64]bool),
				}
				a.bookPairBuckets[pairKey] = pairStats
			}

			// Skip if we've already counted this opportunity (deduplication)
			if pairStats.SeenIDs == nil {
				pairStats.SeenIDs = make(map[int64]bool)
			}
			if pairStats.SeenIDs[opp.ID] {
				continue
			}
			pairStats.SeenIDs[opp.ID] = true

			pairStats.OpportunityCount++
			pairStats.TotalEdgePct += opp.EdgePercent
			pairStats.EdgeList = append(pairStats.EdgeList, opp.EdgePercent)

			if opp.DataAgeSeconds > 0 {
				pairStats.TotalDataAgeSeconds += opp.DataAgeSeconds
				pairStats.DataAgeCount++
			}

			pairStats.LastOpportunityAt = opp.DetectedAt
		}
	}
}

// GetAndClearBuckets retrieves all buckets and clears them (for flushing)
func (a *Aggregator) GetAndClearBuckets() map[BucketKey]*BucketStats {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Copy buckets
	result := make(map[BucketKey]*BucketStats, len(a.buckets))
	for k, v := range a.buckets {
		result[k] = v
	}

	// Clear buckets
	a.buckets = make(map[BucketKey]*BucketStats)

	return result
}

// GetAndClearBookPairBuckets retrieves all book pair buckets and clears them
func (a *Aggregator) GetAndClearBookPairBuckets() map[BookPairKey]*BookPairStats {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Copy buckets
	result := make(map[BookPairKey]*BookPairStats, len(a.bookPairBuckets))
	for k, v := range a.bookPairBuckets {
		result[k] = v
	}

	// Clear buckets
	a.bookPairBuckets = make(map[BookPairKey]*BookPairStats)

	return result
}

// GetBuckets returns a copy of current buckets (for read-only access)
func (a *Aggregator) GetBuckets() map[BucketKey]*BucketStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[BucketKey]*BucketStats, len(a.buckets))
	for k, v := range a.buckets {
		result[k] = v
	}

	return result
}

// roundToBucket rounds a timestamp to the nearest bucket boundary
func (a *Aggregator) roundToBucket(t time.Time) time.Time {
	// Truncate to bucket resolution
	return t.Truncate(a.bucketResolution)
}

// capDecimal6_3 caps a value to fit in DECIMAL(6,3) column (-999.999 to 999.999)
func capDecimal6_3(val float64) float64 {
	if val > 999.999 {
		return 999.999
	}
	if val < -999.999 {
		return -999.999
	}
	return val
}

// calculateEdgeDistribution calculates edge distribution statistics
func calculateEdgeDistribution(edges []float64) (min, max, median, stddev float64) {
	if len(edges) == 0 {
		return 0, 0, 0, 0
	}

	// Sort for median calculation
	sorted := make([]float64, len(edges))
	copy(sorted, edges)
	sort.Float64s(sorted)

	// Min and max (capped to fit in DECIMAL(6,3))
	min = capDecimal6_3(sorted[0])
	max = capDecimal6_3(sorted[len(sorted)-1])

	// Median
	n := len(sorted)
	if n%2 == 0 {
		median = capDecimal6_3((sorted[n/2-1] + sorted[n/2]) / 2)
	} else {
		median = capDecimal6_3(sorted[n/2])
	}

	// Standard deviation
	var sum, mean float64
	for _, e := range edges {
		sum += e
	}
	mean = sum / float64(len(edges))

	var varianceSum float64
	for _, e := range edges {
		varianceSum += (e - mean) * (e - mean)
	}
	stddev = capDecimal6_3(math.Sqrt(varianceSum / float64(len(edges))))

	return min, max, median, stddev
}

// ConvertToBookStats converts aggregated buckets to BookStats slice
func (a *Aggregator) ConvertToBookStats(buckets map[BucketKey]*BucketStats) []models.BookStats {
	stats := make([]models.BookStats, 0, len(buckets))

	for key, bucket := range buckets {
		// Calculate average edge (capped to fit in DECIMAL(6,3))
		avgEdge := 0.0
		if bucket.OpportunityCount > 0 {
			avgEdge = capDecimal6_3(bucket.TotalEdgePct / float64(bucket.OpportunityCount))
		}

		// Calculate edge distribution (Phase 3)
		minEdge, maxEdge, medianEdge, edgeStddev := calculateEdgeDistribution(bucket.EdgeList)

		// Calculate average data age (proxy for hold time - Phase 2)
		avgDataAge := 0
		if bucket.DataAgeCount > 0 {
			avgDataAge = bucket.TotalDataAgeSeconds / bucket.DataAgeCount
		}

		// Calculate opportunities per minute (velocity)
		oppsPerMinute := 0.0
		if !bucket.FirstOpportunityAt.IsZero() && !bucket.LastOpportunityAt.IsZero() {
			duration := bucket.LastOpportunityAt.Sub(bucket.FirstOpportunityAt)
			// Use at least 1 second duration to avoid huge numbers from bursts
			durationMins := duration.Minutes()
			if durationMins < 1.0/60.0 { // Less than 1 second
				durationMins = a.bucketResolution.Minutes() // Use bucket resolution instead
			}
			if durationMins > 0 {
				oppsPerMinute = float64(bucket.OpportunityCount) / durationMins
			}
		}
		// Cap oppsPerMinute to fit in DECIMAL(10,2) - max reasonable value is ~10000/min
		if oppsPerMinute > 99999.99 {
			oppsPerMinute = 99999.99
		}

		stat := models.BookStats{
			TimestampBucket:  key.TimestampBucket,
			BookKey:          key.BookKey,
			OpportunityType:  key.OpportunityType,
			GameStatus:       key.GameStatus,
			SportKey:         key.SportKey,
			MarketKey:        key.MarketKey,
			OpportunityCount: bucket.OpportunityCount,
			AvgEdgePct:       avgEdge,

			// Edge distribution (Phase 3)
			MinEdgePct:    minEdge,
			MaxEdgePct:    maxEdge,
			MedianEdgePct: medianEdge,
			EdgeStddev:    edgeStddev,

			// Data quality metrics
			AvgDataAgeSeconds: avgDataAge,
			StaleDataCount:    bucket.StaleDataCount,

			// Velocity metrics
			OppsPerMinute: oppsPerMinute,

			// Edge threshold counts
			Edge5PctCount:  bucket.Edge5PctCount,
			Edge10PctCount: bucket.Edge10PctCount,
			Edge20PctCount: bucket.Edge20PctCount,

			// Bet metrics will be populated by DB writer from joins
			TotalBets: 0,
			Wins:      0,
			Losses:    0,
			AvgCLV:    0,
			NetProfit: 0,
			ROI:       0,
		}

		stats = append(stats, stat)
	}

	return stats
}

// ConvertToBookPairStats converts aggregated book pair buckets to BookPairStats slice
func (a *Aggregator) ConvertToBookPairStats(buckets map[BookPairKey]*BookPairStats) []models.BookPairStats {
	stats := make([]models.BookPairStats, 0, len(buckets))

	for key, bucket := range buckets {
		avgEdge := 0.0
		if bucket.OpportunityCount > 0 {
			avgEdge = bucket.TotalEdgePct / float64(bucket.OpportunityCount)
		}

		minEdge, maxEdge, _, _ := calculateEdgeDistribution(bucket.EdgeList)

		avgDataAge := 0
		if bucket.DataAgeCount > 0 {
			avgDataAge = bucket.TotalDataAgeSeconds / bucket.DataAgeCount
		}

		stat := models.BookPairStats{
			TimestampBucket:    key.TimestampBucket,
			BookKey1:           key.BookKey1,
			BookKey2:           key.BookKey2,
			OpportunityType:    key.OpportunityType,
			GameStatus:         key.GameStatus,
			SportKey:           key.SportKey,
			MarketKey:          key.MarketKey,
			OpportunityCount:   bucket.OpportunityCount,
			AvgEdgePct:         avgEdge,
			MinEdgePct:         minEdge,
			MaxEdgePct:         maxEdge,
			AvgHoldTimeSeconds: avgDataAge,
		}

		stats = append(stats, stat)
	}

	return stats
}

// GetBucketResolution returns the configured bucket resolution
func (a *Aggregator) GetBucketResolution() time.Duration {
	return a.bucketResolution
}

// Stats returns current statistics about the aggregator
func (a *Aggregator) Stats() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	totalOpps := 0
	for _, bucket := range a.buckets {
		totalOpps += bucket.OpportunityCount
	}

	totalPairs := len(a.bookPairBuckets)

	return fmt.Sprintf("Active buckets: %d, Book pairs: %d, Total opportunities: %d",
		len(a.buckets), totalPairs, totalOpps)
}

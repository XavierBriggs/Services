package basketball_nba

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/XavierBriggs/fortuna/services/edge-detector/pkg/models"
)

// SharpBookProvider dynamically identifies sharp books from Alexandria database
type SharpBookProvider struct {
	db             *sql.DB
	sportKey       string
	configuredBooks []string // Configured sharp books from env/config
	mu             sync.RWMutex
	sharpBooks     map[string]bool // book_key -> is_sharp
	lastRefresh    int64           // Unix timestamp
}

// NewSharpBookProvider creates a new sharp book provider
func NewSharpBookProvider(db *sql.DB, configuredSharpBooks []string) *SharpBookProvider {
	return &SharpBookProvider{
		db:              db,
		sportKey:        "basketball_nba",
		configuredBooks: configuredSharpBooks,
		sharpBooks:      make(map[string]bool),
	}
}

// GetSharpBooks returns the list of book keys considered "sharp"
func (s *SharpBookProvider) GetSharpBooks(ctx context.Context, sportKey string) ([]string, error) {
	// Refresh cache if needed (every 5 minutes)
	if err := s.refreshIfNeeded(ctx); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var sharpBooks []string
	for bookKey, isSharp := range s.sharpBooks {
		if isSharp {
			sharpBooks = append(sharpBooks, bookKey)
		}
	}

	if len(sharpBooks) == 0 {
		return nil, fmt.Errorf("no sharp books found for sport %s", sportKey)
	}

	return sharpBooks, nil
}

// IsSharpBook returns whether a given book is considered sharp
func (s *SharpBookProvider) IsSharpBook(bookKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sharpBooks[bookKey]
}

// GetSharpConsensus calculates the average fair probability from sharp books
func (s *SharpBookProvider) GetSharpConsensus(ctx context.Context, marketOdds []models.NormalizedOdds) (map[string]float64, error) {
	if len(marketOdds) == 0 {
		return nil, fmt.Errorf("no market odds provided")
	}

	// Refresh sharp books if needed
	if err := s.refreshIfNeeded(ctx); err != nil {
		return nil, err
	}

	// Group odds by outcome, filtering for sharp books only
	outcomeProbs := make(map[string][]float64)

	for _, odds := range marketOdds {
		if !s.IsSharpBook(odds.BookKey) {
			continue
		}

		// Use no-vig probability if available, otherwise implied probability
		prob := odds.ImpliedProbability
		if odds.NoVigProbability != nil {
			prob = *odds.NoVigProbability
		}

		outcomeProbs[odds.OutcomeName] = append(outcomeProbs[odds.OutcomeName], prob)
	}

	// Calculate average (consensus) for each outcome
	consensus := make(map[string]float64)
	for outcome, probs := range outcomeProbs {
		if len(probs) == 0 {
			continue
		}

		sum := 0.0
		for _, prob := range probs {
			sum += prob
		}
		consensus[outcome] = sum / float64(len(probs))
	}

	if len(consensus) == 0 {
		return nil, fmt.Errorf("no sharp consensus available (no sharp books in market)")
	}

	return consensus, nil
}

// refreshIfNeeded refreshes the sharp books cache if it's stale (>5 minutes)
func (s *SharpBookProvider) refreshIfNeeded(ctx context.Context) error {
	s.mu.RLock()
	needsRefresh := len(s.sharpBooks) == 0 || (getCurrentTimestamp()-s.lastRefresh) > 300
	s.mu.RUnlock()

	if !needsRefresh {
		return nil
	}

	return s.refresh(ctx)
}

// refresh queries Alexandria for sharp books and updates the cache
func (s *SharpBookProvider) refresh(ctx context.Context) error {
	sharpBooks := make(map[string]bool)
	sharpCount := 0

	// PRIORITY 1: Use configured sharp books if provided
	if len(s.configuredBooks) > 0 {
		fmt.Printf("✓ Using configured sharp books: %v\n", s.configuredBooks)
		
		// Mark configured books as sharp
		for _, bookKey := range s.configuredBooks {
			sharpBooks[bookKey] = true
			sharpCount++
		}

		// Query database to get all available books for reference
		query := `
			SELECT book_key
			FROM books
			WHERE active = true
			  AND $1 = ANY(supported_sports)
			ORDER BY book_key
		`

		rows, err := s.db.QueryContext(ctx, query, s.sportKey)
		if err != nil {
			return fmt.Errorf("failed to query books: %w", err)
		}
		defer rows.Close()

		// Mark non-sharp books as false
		for rows.Next() {
			var bookKey string
			if err := rows.Scan(&bookKey); err != nil {
				return fmt.Errorf("failed to scan book row: %w", err)
			}

			// If not already marked as sharp, mark as false
			if _, exists := sharpBooks[bookKey]; !exists {
				sharpBooks[bookKey] = false
			}
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating book rows: %w", err)
		}

	} else {
		// PRIORITY 2: Fallback to database book_type field
		fmt.Println("ℹ️  No sharp books configured, using database book_type field")

		query := `
			SELECT book_key, book_type, active
			FROM books
			WHERE active = true
			  AND $1 = ANY(supported_sports)
			ORDER BY book_key
		`

		rows, err := s.db.QueryContext(ctx, query, s.sportKey)
		if err != nil {
			return fmt.Errorf("failed to query sharp books: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var bookKey, bookType string
			var active bool

			if err := rows.Scan(&bookKey, &bookType, &active); err != nil {
				return fmt.Errorf("failed to scan book row: %w", err)
			}

			// Mark as sharp if book_type is 'sharp'
			isSharp := bookType == "sharp"
			sharpBooks[bookKey] = isSharp

			if isSharp {
				sharpCount++
			}
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating book rows: %w", err)
		}

		if sharpCount == 0 {
			fmt.Println("⚠️  WARNING: No sharp books found in database! Using all books for consensus.")
		}
	}

	// Update cache
	s.mu.Lock()
	s.sharpBooks = sharpBooks
	s.lastRefresh = getCurrentTimestamp()
	s.mu.Unlock()

	fmt.Printf("✓ Sharp books loaded: %d sharp / %d total books\n", sharpCount, len(sharpBooks))

	// Log sharp books for debugging
	if sharpCount > 0 {
		s.mu.RLock()
		fmt.Print("  Sharp books: ")
		first := true
		for bookKey, isSharp := range s.sharpBooks {
			if isSharp {
				if !first {
					fmt.Print(", ")
				}
				fmt.Print(bookKey)
				first = false
			}
		}
		fmt.Println()
		s.mu.RUnlock()
	}

	return nil
}

// getCurrentTimestamp returns the current Unix timestamp
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// GetSharpConsensusForOutcome calculates sharp consensus for a specific outcome
func (s *SharpBookProvider) GetSharpConsensusForOutcome(ctx context.Context, outcome string, marketOdds []models.NormalizedOdds) (float64, error) {
	consensus, err := s.GetSharpConsensus(ctx, marketOdds)
	if err != nil {
		return 0, err
	}

	prob, exists := consensus[outcome]
	if !exists {
		return 0, fmt.Errorf("outcome %s not found in sharp consensus", outcome)
	}

	return prob, nil
}

// FallbackVigAnalysis identifies likely sharp books by analyzing vig percentages
// This is used as a fallback if no books are marked as "sharp" in the database
func (s *SharpBookProvider) FallbackVigAnalysis(ctx context.Context, marketOdds []models.NormalizedOdds) []string {
	// Group by book and calculate average vig
	bookVigs := make(map[string][]float64)

	for _, odds := range marketOdds {
		// Calculate vig from implied probability
		// For a two-way market, vig = (prob1 + prob2 - 1) * 100
		// We approximate by looking at individual odds
		vig := (odds.ImpliedProbability - 0.5) * 100 // Simplified
		bookVigs[odds.BookKey] = append(bookVigs[odds.BookKey], vig)
	}

	// Find books with consistently low vig (<3%)
	var likelySharpBooks []string
	for bookKey, vigs := range bookVigs {
		avgVig := average(vigs)
		if avgVig < 3.0 {
			likelySharpBooks = append(likelySharpBooks, bookKey)
		}
	}

	return likelySharpBooks
}

// average calculates the average of a slice of float64
func average(nums []float64) float64 {
	if len(nums) == 0 {
		return 0
	}

	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	return sum / float64(len(nums))
}


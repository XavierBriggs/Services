package basketball_nba

import (
	"context"
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/models"
	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/oddsmath"
)

// Normalizer implements SportNormalizer for NBA Basketball
type Normalizer struct {
	config *Config
}

// NewNormalizer creates a new NBA normalizer
func NewNormalizer() *Normalizer {
	return &Normalizer{
		config: DefaultConfig(),
	}
}

// GetSportKey returns the sport identifier
func (n *Normalizer) GetSportKey() string {
	return n.config.SportKey
}

// GetDisplayName returns the human-readable name
func (n *Normalizer) GetDisplayName() string {
	return n.config.DisplayName
}

// GetMarketType classifies the market
func (n *Normalizer) GetMarketType(marketKey string) models.MarketType {
	return n.config.GetMarketType(marketKey)
}

// GetVigMethod returns the vig removal method for this market type
func (n *Normalizer) GetVigMethod(marketType models.MarketType) models.VigMethod {
	switch marketType {
	case models.MarketTypeTwoWay:
		return models.VigMethodMultiplicative
	case models.MarketTypeThreeWay:
		return models.VigMethodAdditive
	case models.MarketTypeProps:
		return models.VigMethodNone
	default:
		return models.VigMethodNone
	}
}

// GetSharpBooks returns the sharp book list
func (n *Normalizer) GetSharpBooks() []string {
	return n.config.SharpBooks
}

// IsSharpBook checks if a book is sharp
func (n *Normalizer) IsSharpBook(bookKey string) bool {
	return n.config.IsSharpBook(bookKey)
}

// Normalize processes raw odds and returns normalized odds with fair prices and edges
func (n *Normalizer) Normalize(ctx context.Context, raw models.RawOdds, marketOdds []models.RawOdds) (*models.NormalizedOdds, error) {
	startTime := time.Now()

	// Convert American odds to decimal and implied probability
	decimal, err := oddsmath.AmericanToDecimal(raw.Price)
	if err != nil {
		return nil, fmt.Errorf("error converting American odds: %w", err)
	}

	impliedProb, err := oddsmath.DecimalToImpliedProbability(decimal)
	if err != nil {
		return nil, fmt.Errorf("error calculating implied probability: %w", err)
	}

	// Initialize normalized odds
	normalized := &models.NormalizedOdds{
		RawOdds:            raw,
		DecimalOdds:        decimal,
		ImpliedProbability: impliedProb,
		NormalizedAt:       time.Now(),
	}

	// Get market type and vig method
	marketType := n.GetMarketType(raw.MarketKey)
	vigMethod := n.GetVigMethod(marketType)

	normalized.MarketType = string(marketType)
	normalized.VigMethod = string(vigMethod)

	// Process based on market type
	switch marketType {
	case models.MarketTypeTwoWay:
		// Two-way markets (spreads, totals) - remove vig and calculate fair price
		if err := n.normalizeTwoWayMarket(normalized, raw, marketOdds); err != nil {
			return nil, fmt.Errorf("error normalizing two-way market: %w", err)
		}

	case models.MarketTypeThreeWay:
		// Three-way markets (moneyline) - compare to sharp consensus
		if err := n.normalizeThreeWayMarket(normalized, raw, marketOdds); err != nil {
			return nil, fmt.Errorf("error normalizing three-way market: %w", err)
		}

	case models.MarketTypeProps:
		// Props - compare across books
		if err := n.normalizePropsMarket(normalized, raw, marketOdds); err != nil {
			return nil, fmt.Errorf("error normalizing props market: %w", err)
		}
	}

	// Calculate processing latency
	normalized.ProcessingLatency = time.Since(startTime).Milliseconds()

	return normalized, nil
}

// normalizeTwoWayMarket handles spreads and totals
func (n *Normalizer) normalizeTwoWayMarket(normalized *models.NormalizedOdds, raw models.RawOdds, marketOdds []models.RawOdds) error {
	// Find the opposite side of this market
	oppositeSide := n.findOppositeSide(raw, marketOdds)
	if oppositeSide == nil {
		// Can't calculate fair price without opposite side
		return nil
	}

	// Convert opposite side to probability
	oppositeDecimal, err := oddsmath.AmericanToDecimal(oppositeSide.Price)
	if err != nil {
		return err
	}

	oppositeProb, err := oddsmath.DecimalToImpliedProbability(oppositeDecimal)
	if err != nil {
		return err
	}

	// Remove vig using multiplicative method
	fairProb1, _, err := oddsmath.RemoveVigMultiplicative(normalized.ImpliedProbability, oppositeProb)
	if err != nil {
		// If vig removal fails, just use implied probability
		return nil
	}

	normalized.NoVigProbability = &fairProb1

	// Convert fair probability to American odds
	fairPrice, err := oddsmath.ProbabilityToAmerican(fairProb1)
	if err == nil {
		normalized.FairPrice = &fairPrice
	}

	// Calculate edge vs fair price
	edge, err := oddsmath.CalculateEdge(fairProb1, normalized.ImpliedProbability)
	if err == nil {
		normalized.Edge = &edge
	}

	// Calculate sharp consensus if this is a soft book
	if !n.IsSharpBook(raw.BookKey) {
		consensus := n.calculateSharpConsensus(raw, marketOdds)
		if consensus != nil {
			normalized.SharpConsensus = consensus

			// Recalculate edge vs sharp consensus
			edgeVsSharp, err := oddsmath.CalculateEdge(*consensus, normalized.ImpliedProbability)
			if err == nil {
				normalized.Edge = &edgeVsSharp
			}
		}
	}

	return nil
}

// normalizeThreeWayMarket handles moneylines
func (n *Normalizer) normalizeThreeWayMarket(normalized *models.NormalizedOdds, raw models.RawOdds, marketOdds []models.RawOdds) error {
	// For NBA moneylines, just calculate sharp consensus (no vig removal for 3-way)
	if !n.IsSharpBook(raw.BookKey) {
		consensus := n.calculateSharpConsensus(raw, marketOdds)
		if consensus != nil {
			normalized.SharpConsensus = consensus

			// Calculate edge vs sharp consensus
			edge, err := oddsmath.CalculateEdge(*consensus, normalized.ImpliedProbability)
			if err == nil {
				normalized.Edge = &edge
			}
		}
	}

	return nil
}

// normalizePropsMarket handles player props
func (n *Normalizer) normalizePropsMarket(normalized *models.NormalizedOdds, raw models.RawOdds, marketOdds []models.RawOdds) error {
	// For props, compare to sharp consensus
	if !n.IsSharpBook(raw.BookKey) {
		consensus := n.calculateSharpConsensus(raw, marketOdds)
		if consensus != nil {
			normalized.SharpConsensus = consensus

			// Calculate edge vs sharp consensus
			edge, err := oddsmath.CalculateEdge(*consensus, normalized.ImpliedProbability)
			if err == nil {
				normalized.Edge = &edge
			}
		}
	}

	return nil
}

// findOppositeSide finds the opposite outcome in a two-way market
func (n *Normalizer) findOppositeSide(raw models.RawOdds, marketOdds []models.RawOdds) *models.RawOdds {
	for _, odds := range marketOdds {
		// Must be same event, market, and book but different outcome
		if odds.EventID == raw.EventID &&
			odds.MarketKey == raw.MarketKey &&
			odds.BookKey == raw.BookKey &&
			odds.OutcomeName != raw.OutcomeName {

			// For spreads/totals, also check point values match
			if raw.Point != nil && odds.Point != nil {
				if *raw.Point == -*odds.Point { // Opposite point
					return &odds
				}
			} else if raw.Point == nil && odds.Point == nil {
				return &odds
			}
		}
	}
	return nil
}

// calculateSharpConsensus calculates average probability from sharp books
func (n *Normalizer) calculateSharpConsensus(raw models.RawOdds, marketOdds []models.RawOdds) *float64 {
	var sharpProbs []float64

	for _, odds := range marketOdds {
		// Must be same event, market, outcome but from a sharp book
		if odds.EventID == raw.EventID &&
			odds.MarketKey == raw.MarketKey &&
			odds.OutcomeName == raw.OutcomeName &&
			n.IsSharpBook(odds.BookKey) {

			// Convert to probability
			decimal, err := oddsmath.AmericanToDecimal(odds.Price)
			if err != nil {
				continue
			}

			prob, err := oddsmath.DecimalToImpliedProbability(decimal)
			if err != nil {
				continue
			}

			sharpProbs = append(sharpProbs, prob)
		}
	}

	if len(sharpProbs) == 0 {
		return nil
	}

	// Average sharp probabilities
	sum := 0.0
	for _, prob := range sharpProbs {
		sum += prob
	}

	consensus := sum / float64(len(sharpProbs))
	return &consensus
}


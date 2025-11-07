package models

import "time"

// RawOdds represents raw odds from Mercury (matches Mercury's model)
type RawOdds struct {
	EventID          string    `json:"event_id"`
	SportKey         string    `json:"sport_key"`
	MarketKey        string    `json:"market_key"`
	BookKey          string    `json:"book_key"`
	OutcomeName      string    `json:"outcome_name"`
	Price            int       `json:"price"`              // American odds
	Point            *float64  `json:"point,omitempty"`    // For spreads/totals
	VendorLastUpdate time.Time `json:"vendor_last_update"`
	ReceivedAt       time.Time `json:"received_at"`
}

// NormalizedOdds represents odds after normalization with fair prices and edges
type NormalizedOdds struct {
	// Original odds data
	RawOdds

	// Normalized values
	DecimalOdds        float64   `json:"decimal_odds"`         // Converted from American
	ImpliedProbability float64   `json:"implied_probability"`  // 1 / decimal
	NoVigProbability   *float64  `json:"novig_probability"`    // After vig removal (two-way markets)
	FairPrice          *int      `json:"fair_price"`           // American odds equivalent of fair probability
	Edge               *float64  `json:"edge"`                 // Percentage edge vs fair price
	
	// Sharp consensus (for soft books)
	SharpConsensus     *float64  `json:"sharp_consensus"`      // Average sharp book probability
	
	// Market classification
	MarketType         string    `json:"market_type"`          // two_way, three_way, props
	VigMethod          string    `json:"vig_method"`           // multiplicative, additive
	
	// Metadata
	NormalizedAt       time.Time `json:"normalized_at"`
	ProcessingLatency  int64     `json:"processing_latency_ms"` // Milliseconds
}

// MarketType defines the type of betting market
type MarketType string

const (
	MarketTypeTwoWay   MarketType = "two_way"   // spreads, totals
	MarketTypeThreeWay MarketType = "three_way" // moneyline (home, away, draw)
	MarketTypeProps    MarketType = "props"     // player props
)

// VigMethod defines how to remove vig
type VigMethod string

const (
	VigMethodMultiplicative VigMethod = "multiplicative" // Two-way markets
	VigMethodAdditive       VigMethod = "additive"       // Three-way markets
	VigMethodNone           VigMethod = "none"           // Props (book-vs-book comparison)
)

// BookType classifies sportsbook
type BookType string

const (
	BookTypeSharp BookType = "sharp" // Pinnacle, Circa, Bookmaker
	BookTypeSoft  BookType = "soft"  // FanDuel, DraftKings, etc.
)


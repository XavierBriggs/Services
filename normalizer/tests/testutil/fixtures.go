package testutil

import (
	"time"

	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/models"
)

// RawOddsFixture creates a test RawOdds with sensible defaults
func RawOddsFixture(overrides ...func(*models.RawOdds)) models.RawOdds {
	point := 7.5
	
	odds := models.RawOdds{
		EventID:          "test-event-1",
		SportKey:         "basketball_nba",
		MarketKey:        "spreads",
		BookKey:          "fanduel",
		OutcomeName:      "Los Angeles Lakers",
		Price:            -110,
		Point:            &point,
		VendorLastUpdate: time.Now(),
		ReceivedAt:       time.Now(),
	}

	// Apply overrides
	for _, override := range overrides {
		override(&odds)
	}

	return odds
}

// SharpBookOdds creates test odds from a sharp book
func SharpBookOdds(bookKey string, price int) models.RawOdds {
	return RawOddsFixture(func(o *models.RawOdds) {
		o.BookKey = bookKey
		o.Price = price
	})
}

// SoftBookOdds creates test odds from a soft book
func SoftBookOdds(bookKey string, price int) models.RawOdds {
	return RawOddsFixture(func(o *models.RawOdds) {
		o.BookKey = bookKey
		o.Price = price
	})
}

// SpreadOdds creates spread odds
func SpreadOdds(bookKey string, outcome string, price int, point float64) models.RawOdds {
	return RawOddsFixture(func(o *models.RawOdds) {
		o.MarketKey = "spreads"
		o.BookKey = bookKey
		o.OutcomeName = outcome
		o.Price = price
		o.Point = &point
	})
}

// TotalOdds creates total (over/under) odds
func TotalOdds(bookKey string, outcome string, price int, point float64) models.RawOdds {
	return RawOddsFixture(func(o *models.RawOdds) {
		o.MarketKey = "totals"
		o.BookKey = bookKey
		o.OutcomeName = outcome
		o.Price = price
		o.Point = &point
	})
}

// MoneylineOdds creates moneyline odds
func MoneylineOdds(bookKey string, outcome string, price int) models.RawOdds {
	return RawOddsFixture(func(o *models.RawOdds) {
		o.MarketKey = "h2h"
		o.BookKey = bookKey
		o.OutcomeName = outcome
		o.Price = price
		o.Point = nil
	})
}

// PropOdds creates player prop odds
func PropOdds(bookKey string, outcome string, price int, point float64) models.RawOdds {
	return RawOddsFixture(func(o *models.RawOdds) {
		o.MarketKey = "player_points"
		o.BookKey = bookKey
		o.OutcomeName = outcome
		o.Price = price
		o.Point = &point
	})
}

// StandardVigMarket creates a standard two-way market with -110/-110 pricing
func StandardVigMarket() (side1, side2 models.RawOdds) {
	point1 := -7.5
	point2 := 7.5
	
	side1 = RawOddsFixture(func(o *models.RawOdds) {
		o.OutcomeName = "Los Angeles Lakers"
		o.Price = -110
		o.Point = &point1
	})

	side2 = RawOddsFixture(func(o *models.RawOdds) {
		o.OutcomeName = "Boston Celtics"
		o.Price = -110
		o.Point = &point2
	})

	return
}

// MiddleOpportunity creates a middle opportunity (both sides +EV)
func MiddleOpportunity() (book1Side1, book2Side2 models.RawOdds) {
	point := 7.5

	// Book 1: Lakers -7.5 at +105 (over-priced underdog)
	book1Side1 = RawOddsFixture(func(o *models.RawOdds) {
		o.BookKey = "fanduel"
		o.OutcomeName = "Los Angeles Lakers"
		o.Price = 105
		o.Point = &point
	})

	// Book 2: Celtics +7.5 at +105 (over-priced favorite)
	negPoint := -point
	book2Side2 = RawOddsFixture(func(o *models.RawOdds) {
		o.BookKey = "draftkings"
		o.OutcomeName = "Boston Celtics"
		o.Price = 105
		o.Point = &negPoint
	})

	return
}

// ScalpOpportunity creates a scalp (arbitrage) opportunity
func ScalpOpportunity() (book1, book2 models.RawOdds) {
	// Book 1: Over 220.5 at +110
	point := 220.5
	book1 = TotalOdds("fanduel", "Over", 110, point)

	// Book 2: Under 220.5 at +110
	book2 = TotalOdds("draftkings", "Under", 110, point)

	return
}

// Float64Ptr returns a pointer to a float64
func Float64Ptr(v float64) *float64 {
	return &v
}

// IntPtr returns a pointer to an int
func IntPtr(v int) *int {
	return &v
}






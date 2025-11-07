package sports_test

import (
	"context"
	"testing"

	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/models"
	"github.com/XavierBriggs/fortuna/services/normalizer/sports/basketball_nba"
	"github.com/XavierBriggs/fortuna/services/normalizer/tests/testutil"
)

func TestNBANormalizer_GetSportKey(t *testing.T) {
	normalizer := basketball_nba.NewNormalizer()

	want := "basketball_nba"
	got := normalizer.GetSportKey()

	if got != want {
		t.Errorf("GetSportKey() = %s, want %s", got, want)
	}
}

func TestNBANormalizer_GetDisplayName(t *testing.T) {
	normalizer := basketball_nba.NewNormalizer()

	want := "NBA Basketball"
	got := normalizer.GetDisplayName()

	if got != want {
		t.Errorf("GetDisplayName() = %s, want %s", got, want)
	}
}

func TestNBANormalizer_GetMarketType(t *testing.T) {
	normalizer := basketball_nba.NewNormalizer()

	tests := []struct {
		marketKey string
		want      models.MarketType
	}{
		{"spreads", models.MarketTypeTwoWay},
		{"totals", models.MarketTypeTwoWay},
		{"h2h", models.MarketTypeThreeWay},
		{"player_points", models.MarketTypeProps},
		{"player_rebounds", models.MarketTypeProps},
		{"unknown_market", models.MarketTypeProps}, // Default to props
	}

	for _, tt := range tests {
		t.Run(tt.marketKey, func(t *testing.T) {
			got := normalizer.GetMarketType(tt.marketKey)
			if got != tt.want {
				t.Errorf("GetMarketType(%s) = %v, want %v", tt.marketKey, got, tt.want)
			}
		})
	}
}

func TestNBANormalizer_IsSharpBook(t *testing.T) {
	normalizer := basketball_nba.NewNormalizer()

	tests := []struct {
		bookKey string
		want    bool
	}{
		{"pinnacle", true},
		{"circa", true},
		{"bookmaker", true},
		{"fanduel", false},
		{"draftkings", false},
		{"betmgm", false},
	}

	for _, tt := range tests {
		t.Run(tt.bookKey, func(t *testing.T) {
			got := normalizer.IsSharpBook(tt.bookKey)
			if got != tt.want {
				t.Errorf("IsSharpBook(%s) = %v, want %v", tt.bookKey, got, tt.want)
			}
		})
	}
}

func TestNBANormalizer_GetVigMethod(t *testing.T) {
	normalizer := basketball_nba.NewNormalizer()

	tests := []struct {
		marketType models.MarketType
		want       models.VigMethod
	}{
		{models.MarketTypeTwoWay, models.VigMethodMultiplicative},
		{models.MarketTypeThreeWay, models.VigMethodAdditive},
		{models.MarketTypeProps, models.VigMethodNone},
	}

	for _, tt := range tests {
		t.Run(string(tt.marketType), func(t *testing.T) {
			got := normalizer.GetVigMethod(tt.marketType)
			if got != tt.want {
				t.Errorf("GetVigMethod(%v) = %v, want %v", tt.marketType, got, tt.want)
			}
		})
	}
}

func TestNBANormalizer_NormalizeTwoWayMarket(t *testing.T) {
	normalizer := basketball_nba.NewNormalizer()
	ctx := context.Background()

	// Create a standard -110/-110 spread market
	lakers := testutil.SpreadOdds("fanduel", "Los Angeles Lakers", -110, -7.5)
	celtics := testutil.SpreadOdds("fanduel", "Boston Celtics", -110, 7.5)
	celtics.EventID = lakers.EventID // Same event

	marketOdds := []models.RawOdds{lakers, celtics}

	normalized, err := normalizer.Normalize(ctx, lakers, marketOdds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify basic fields
	if normalized.EventID != lakers.EventID {
		t.Errorf("EventID mismatch")
	}

	if normalized.MarketType != "two_way" {
		t.Errorf("MarketType = %s, want two_way", normalized.MarketType)
	}

	if normalized.VigMethod != "multiplicative" {
		t.Errorf("VigMethod = %s, want multiplicative", normalized.VigMethod)
	}

	// Verify no-vig probability is calculated
	if normalized.NoVigProbability == nil {
		t.Error("NoVigProbability should not be nil for two-way market")
	} else {
		// Standard -110/-110 should be 50% after vig removal
		if *normalized.NoVigProbability < 0.49 || *normalized.NoVigProbability > 0.51 {
			t.Errorf("NoVigProbability = %f, want ~0.50", *normalized.NoVigProbability)
		}
	}

	// Verify fair price is calculated
	if normalized.FairPrice == nil {
		t.Error("FairPrice should not be nil for two-way market")
	}

	// Verify edge is calculated
	if normalized.Edge == nil {
		t.Error("Edge should not be nil")
	}

	// Verify processing latency is set (>= 0 is valid, ultra-fast operations can be 0ms)
	if normalized.ProcessingLatency < 0 {
		t.Error("ProcessingLatency should be >= 0")
	}
}

func TestNBANormalizer_NormalizeWithSharpConsensus(t *testing.T) {
	normalizer := basketball_nba.NewNormalizer()
	ctx := context.Background()

	// Create soft book odds and sharp book odds
	softOdds := testutil.SpreadOdds("fanduel", "Los Angeles Lakers", -105, -7.5)
	sharpOdds1 := testutil.SpreadOdds("pinnacle", "Los Angeles Lakers", -110, -7.5)
	sharpOdds2 := testutil.SpreadOdds("circa", "Los Angeles Lakers", -108, -7.5)
	
	// Make them all same event
	sharpOdds1.EventID = softOdds.EventID
	sharpOdds2.EventID = softOdds.EventID

	// Add opposite sides for vig removal
	softOpposite := testutil.SpreadOdds("fanduel", "Boston Celtics", -115, 7.5)
	sharpOpposite1 := testutil.SpreadOdds("pinnacle", "Boston Celtics", -110, 7.5)
	sharpOpposite2 := testutil.SpreadOdds("circa", "Boston Celtics", -112, 7.5)
	softOpposite.EventID = softOdds.EventID
	sharpOpposite1.EventID = softOdds.EventID
	sharpOpposite2.EventID = softOdds.EventID

	marketOdds := []models.RawOdds{
		softOdds, softOpposite,
		sharpOdds1, sharpOpposite1,
		sharpOdds2, sharpOpposite2,
	}

	normalized, err := normalizer.Normalize(ctx, softOdds, marketOdds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Soft book should have sharp consensus calculated
	if normalized.SharpConsensus == nil {
		t.Error("SharpConsensus should not be nil for soft book")
	} else {
		// Should be around 50% for balanced line
		if *normalized.SharpConsensus < 0.45 || *normalized.SharpConsensus > 0.55 {
			t.Errorf("SharpConsensus = %f, want ~0.50", *normalized.SharpConsensus)
		}
	}

	// Edge should be calculated vs sharp consensus
	if normalized.Edge == nil {
		t.Error("Edge should not be nil")
	}
}

func TestNBANormalizer_NormalizeMoneyline(t *testing.T) {
	normalizer := basketball_nba.NewNormalizer()
	ctx := context.Background()

	// Create moneyline odds
	lakersML := testutil.MoneylineOdds("fanduel", "Los Angeles Lakers", -150)
	celticsML := testutil.MoneylineOdds("fanduel", "Boston Celtics", 130)
	celticsML.EventID = lakersML.EventID

	marketOdds := []models.RawOdds{lakersML, celticsML}

	normalized, err := normalizer.Normalize(ctx, lakersML, marketOdds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify market type
	if normalized.MarketType != "three_way" {
		t.Errorf("MarketType = %s, want three_way", normalized.MarketType)
	}

	// Verify vig method
	if normalized.VigMethod != "additive" {
		t.Errorf("VigMethod = %s, want additive", normalized.VigMethod)
	}
}

func TestNBANormalizer_NormalizePlayerProp(t *testing.T) {
	normalizer := basketball_nba.NewNormalizer()
	ctx := context.Background()

	// Create player prop odds
	propOver := testutil.PropOdds("fanduel", "Over", -110, 25.5)
	propUnder := testutil.PropOdds("fanduel", "Under", -110, 25.5)
	propOver.MarketKey = "player_points"
	propUnder.MarketKey = "player_points"
	propUnder.EventID = propOver.EventID

	marketOdds := []models.RawOdds{propOver, propUnder}

	normalized, err := normalizer.Normalize(ctx, propOver, marketOdds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify market type
	if normalized.MarketType != "props" {
		t.Errorf("MarketType = %s, want props", normalized.MarketType)
	}

	// Verify vig method
	if normalized.VigMethod != "none" {
		t.Errorf("VigMethod = %s, want none", normalized.VigMethod)
	}
}

func TestNBANormalizer_MissingOppositeSide(t *testing.T) {
	normalizer := basketball_nba.NewNormalizer()
	ctx := context.Background()

	// Create spread odds without opposite side
	lakers := testutil.SpreadOdds("fanduel", "Los Angeles Lakers", -110, -7.5)
	marketOdds := []models.RawOdds{lakers}

	// Should not error, but won't calculate no-vig
	normalized, err := normalizer.Normalize(ctx, lakers, marketOdds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// NoVigProbability should be nil when opposite side is missing
	if normalized.NoVigProbability != nil {
		t.Error("NoVigProbability should be nil when opposite side is missing")
	}
}


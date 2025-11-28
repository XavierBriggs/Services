package sizing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// KellyCalculator wraps the Kelly Calculator service
type KellyCalculator struct {
	kellyCalculatorURL string
	httpClient         *http.Client
}

// NewKellyCalculator creates a new Kelly calculator client
func NewKellyCalculator(kellyCalculatorURL string) *KellyCalculator {
	return &KellyCalculator{
		kellyCalculatorURL: kellyCalculatorURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// KellyRequest represents a request to the Kelly Calculator service
type KellyRequest struct {
	Opportunity struct {
		OpportunityType string                `json:"opportunity_type"`
		EdgePct         float64               `json:"edge_pct"`
		Legs            []KellyOpportunityLeg `json:"legs"`
	} `json:"opportunity"`
	Bankroll      float64 `json:"bankroll"`
	KellyFraction float64 `json:"kelly_fraction"`
}

// KellyOpportunityLeg represents an opportunity leg for Kelly calculation
type KellyOpportunityLeg struct {
	BookKey     string   `json:"book_key"`
	OutcomeName string   `json:"outcome_name"`
	Price       int      `json:"price"`
	Point       *float64 `json:"point,omitempty"`
	LegEdgePct  float64  `json:"leg_edge_pct"`
}

// KellyResponse represents the Kelly Calculator response
type KellyResponse struct {
	Stakes          map[string]float64 `json:"stakes"` // Map of book_key -> stake
	TotalStake      float64            `json:"total_stake"`
	FullKellyPct    float64            `json:"full_kelly_pct"`
	FractionalKelly float64            `json:"fractional_kelly"`
	ExpectedValue   float64            `json:"expected_value"`
	EVPerDollar     float64            `json:"ev_per_dollar"`
}

// CalculateStake calculates optimal stake using Kelly Criterion
func (kc *KellyCalculator) CalculateStake(
	opp models.Opportunity,
	bankroll float64,
	kellyFraction float64,
	settings models.UserSettings,
	correlationDiscount float64,
) (*KellyResponse, error) {
	// Build request
	req := KellyRequest{
		Bankroll:      bankroll,
		KellyFraction: kellyFraction,
	}

	req.Opportunity.OpportunityType = opp.OpportunityType
	req.Opportunity.EdgePct = opp.EdgePct
	req.Opportunity.Legs = make([]KellyOpportunityLeg, len(opp.Legs))

	for i, leg := range opp.Legs {
		req.Opportunity.Legs[i] = KellyOpportunityLeg{
			BookKey:     leg.BookKey,
			OutcomeName: leg.OutcomeName,
			Price:       leg.Price,
			Point:       leg.Point,
			LegEdgePct:  leg.LegEdgePct,
		}
	}

	// Make request to Kelly Calculator
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := kc.httpClient.Post(
		kc.kellyCalculatorURL+"/api/v1/calculate-from-opportunity",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call Kelly Calculator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Kelly Calculator returned status %d", resp.StatusCode)
	}

	var kellyResp KellyResponse
	if err := json.NewDecoder(resp.Body).Decode(&kellyResp); err != nil {
		return nil, fmt.Errorf("failed to parse Kelly response: %w", err)
	}

	// Apply correlation discount if needed
	if correlationDiscount > 0 {
		for bookKey, stake := range kellyResp.Stakes {
			kellyResp.Stakes[bookKey] = stake * (1 - correlationDiscount)
		}
		kellyResp.TotalStake = kellyResp.TotalStake * (1 - correlationDiscount)
	}

	// Apply max Kelly percentage cap
	maxStake := bankroll * (settings.AutoMaxKellyPct / 100.0)
	if kellyResp.TotalStake > maxStake {
		// Scale down proportionally
		scale := maxStake / kellyResp.TotalStake
		for bookKey, stake := range kellyResp.Stakes {
			kellyResp.Stakes[bookKey] = stake * scale
		}
		kellyResp.TotalStake = maxStake
	}

	// Apply absolute max stake per bet cap
	if kellyResp.TotalStake > settings.AutoMaxStakePerBet {
		// Scale down proportionally
		scale := settings.AutoMaxStakePerBet / kellyResp.TotalStake
		for bookKey, stake := range kellyResp.Stakes {
			kellyResp.Stakes[bookKey] = stake * scale
		}
		kellyResp.TotalStake = settings.AutoMaxStakePerBet
	}

	// Ensure stakes meet minimum
	for bookKey, stake := range kellyResp.Stakes {
		if stake < settings.AutoMinStake {
			// Stake too small - set to minimum or skip
			if kellyResp.TotalStake < settings.AutoMinStake {
				return nil, fmt.Errorf("calculated stake $%.2f below minimum $%.2f",
					kellyResp.TotalStake, settings.AutoMinStake)
			}
			kellyResp.Stakes[bookKey] = settings.AutoMinStake
		}
	}

	return &kellyResp, nil
}

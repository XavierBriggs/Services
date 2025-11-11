# Kelly Calculator Service

Intelligent position sizing service for all opportunity types: edges (Kelly Criterion), scalps (arbitrage distribution), and middles (dual Kelly).

## Features

- **Edge Bets**: Kelly Criterion with fractional sizing (default 1/4 Kelly)
- **Scalp Bets**: Optimal arbitrage stake distribution for guaranteed profit
- **Middle Bets**: Dual independent Kelly for both legs
- Type-specific warnings and recommendations
- Configurable bankroll, Kelly fraction, and risk parameters

## API

### POST /api/v1/calculate-from-opportunity

Calculate stake recommendations for an opportunity.

**Request:**
```json
{
  "opportunity": {
    "id": 42,
    "opportunity_type": "edge|middle|scalp",
    "edge_pct": 2.38,
    "legs": [
      {
        "book_key": "fanduel",
        "outcome_name": "LAL -3.5",
        "price": -110,
        "leg_edge_pct": 2.38
      }
    ]
  },
  "bankroll": 10000,
  "kelly_fraction": 0.25
}
```

**Response (Edge):**
```json
{
  "type": "edge",
  "total_stake": 96.25,
  "legs": [
    {
      "book": "fanduel",
      "outcome": "LAL -3.5 @ -110",
      "stake": 96.25,
      "full_kelly": 385.00,
      "fractional_kelly": 96.25,
      "edge_pct": 2.38,
      "ev_per_dollar": 0.0238,
      "explanation": "1/4 Kelly sizing (conservative)"
    }
  ],
  "confidence": "medium",
  "warnings": []
}
```

**Response (Scalp):**
```json
{
  "type": "scalp",
  "total_stake": 1000.00,
  "guaranteed_profit": 15.50,
  "profit_pct": 1.55,
  "legs": [
    {
      "book": "fanduel",
      "outcome": "LAL +3.5 @ +115",
      "stake": 535.71,
      "potential_return": 1015.50,
      "explanation": "Stake to guarantee $15.50 profit"
    },
    {
      "book": "draftkings",
      "outcome": "LAL -3.5 @ -105",
      "stake": 464.29,
      "potential_return": 1015.50,
      "explanation": "Stake to guarantee $15.50 profit"
    }
  ],
  "instructions": "Place both bets simultaneously for guaranteed profit",
  "warnings": ["Book limits may prevent full stake"]
}
```

## Configuration

Environment variables:

```bash
KELLY_SERVICE_PORT=8084           # Default: 8084
DEFAULT_BANKROLL=10000            # Default: $10,000
KELLY_DEFAULT_FRACTION=0.25       # Default: 0.25 (1/4 Kelly)
KELLY_MIN_EDGE_PCT=1.0            # Default: 1.0%
KELLY_MAX_PCT=10.0                # Default: 10% (max stake cap)
```

## Kelly Criterion Formula

For edge bets:

```
Kelly % = (bp - q) / b

where:
  b = decimal odds - 1 (net odds)
  p = fair win probability
  q = fair loss probability (1 - p)
```

Fractional Kelly (recommended):
- 1/4 Kelly: Conservative, 50% of growth with 25% of variance
- 1/2 Kelly: Moderate, 75% of growth with 50% of variance
- Full Kelly: Aggressive, maximum growth but high variance (NOT RECOMMENDED)

## Arbitrage Stake Distribution

For scalp bets:

```
Stake(i) = TotalStake × (1/decimal(i)) / ΣInverse
```

This ensures equal profit regardless of outcome.

## Running

### Local Development
```bash
# Install dependencies
go mod download

# Run service
make run

# Or with custom config
DEFAULT_BANKROLL=20000 KELLY_DEFAULT_FRACTION=0.5 go run ./cmd/kelly-calculator/main.go
```

### Docker
```bash
# Build image
make docker-build

# Run container
docker run -p 8084:8084 \
  -e DEFAULT_BANKROLL=10000 \
  -e KELLY_DEFAULT_FRACTION=0.25 \
  kelly-calculator:latest
```

## Testing

```bash
# Run all tests
make test

# Test edge calculation
curl -X POST http://localhost:8084/api/v1/calculate-from-opportunity \
  -H "Content-Type: application/json" \
  -d '{
    "opportunity": {
      "id": 1,
      "opportunity_type": "edge",
      "edge_pct": 2.5,
      "legs": [{
        "book_key": "fanduel",
        "outcome_name": "LAL -3.5",
        "price": -110
      }]
    },
    "bankroll": 10000,
    "kelly_fraction": 0.25
  }'
```

## Best Practices

1. **Always use fractional Kelly** (1/4 or 1/2) - full Kelly assumes perfect edge estimation
2. **Cap maximum stake** at 10% of bankroll (already implemented)
3. **Minimum edge threshold** of 1-2% to account for uncertainty
4. **Separate betting bankroll** from other funds
5. **Track CLV over time** - adjust if consistently negative

## Integration

This service is designed to be called from the opportunities page when the "Bet" button is clicked:

1. User clicks "Bet" on opportunity
2. Frontend sends opportunity to Kelly Calculator
3. Calculator returns optimized stakes
4. User confirms and places bets via API Gateway

See `web/fortuna_client/components/bet/QuickBetModal.tsx` for frontend integration.



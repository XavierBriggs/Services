# Fortuna Normalizer Service

The Normalizer service processes raw odds from Mercury and computes fair prices, edges, and sharp consensus. It removes vig, calculates expected value, and flags profitable betting opportunities.

## Features

- ✅ **Vig Removal**: Multiplicative method for two-way markets (spreads, totals)
- ✅ **Sharp Consensus**: Averages sharp book pricing (Pinnacle, Circa, Bookmaker)
- ✅ **Edge Calculation**: Identifies +EV bets vs fair prices
- ✅ **Sport Modularity**: Registry pattern for easy sport expansion
- ✅ **Real-Time Streaming**: Consumes Redis Streams, publishes normalized odds
- ✅ **Low Latency**: < 10ms avg processing time per odds update
- ✅ **Graceful Shutdown**: Clean SIGINT/SIGTERM handling

## Architecture

```
Redis Stream (odds.raw.{sport})
        ↓
  StreamConsumer
        ↓
    Processor → SportRegistry → NBA Normalizer
        ↓                              ↓
  StreamPublisher           oddsmath package
        ↓                    (vig removal, edge calc)
Redis Stream (odds.normalized.{sport})
```

## Sport Modularity

The Normalizer uses a **Sport Registry pattern** to support multiple sports:

```go
type SportNormalizer interface {
    GetSportKey() string
    GetDisplayName() string
    Normalize(ctx, raw, marketOdds) (*NormalizedOdds, error)
    GetMarketType(marketKey) MarketType
    GetVigMethod(marketType) VigMethod
    GetSharpBooks() []string
    IsSharpBook(bookKey) bool
}
```

### Current Sports

| Sport | Status | Sharp Books | Vig Method |
|-------|--------|-------------|------------|
| **NBA** | ✅ Active | Pinnacle, Circa, Bookmaker | Multiplicative (two-way), Additive (moneyline) |
| **NFL** | 🔜 Planned | TBD | Multiplicative |
| **MLB** | 🔜 Planned | TBD | Multiplicative |

### Adding a New Sport

1. **Create Sport Module** (`sports/american_football_nfl/`)

```go
package american_football_nfl

import "github.com/XavierBriggs/fortuna/services/normalizer/pkg/contracts"

type Normalizer struct {
    config *Config
}

func NewNormalizer() *Normalizer {
    return &Normalizer{config: DefaultConfig()}
}

// Implement SportNormalizer interface
func (n *Normalizer) GetSportKey() string { return "american_football_nfl" }
func (n *Normalizer) GetDisplayName() string { return "NFL Football" }
// ... implement remaining methods
```

2. **Create Configuration** (`sports/american_football_nfl/config.go`)

```go
func DefaultConfig() *Config {
    return &Config{
        SportKey:    "american_football_nfl",
        DisplayName: "NFL Football",
        SharpBooks:  []string{"pinnacle", "circa", "bookmaker"},
        TwoWayMarkets: []string{"spreads", "totals", "alt_lines"},
        ThreeWayMarkets: []string{"h2h"},
        MinEdgeForAlert: 0.02,
    }
}
```

3. **Register in `main.go`**

```go
nflModule := american_football_nfl.NewNormalizer()
if err := normalizerRegistry.Register(nflModule); err != nil {
    log.Fatal(err)
}
```

## Odds Math

### Implied Probability

```
Favorite (-110):  prob = 110 / (110 + 100) = 52.38%
Underdog (+150):  prob = 100 / (150 + 100) = 40%
```

### Vig Removal (Multiplicative)

Used for two-way markets (spreads, totals):

```
Lakers -7.5 at -110 (52.38%)
Celtics +7.5 at -110 (52.38%)
Total: 104.76% (4.76% vig)

Fair Price = Implied / Total
Lakers Fair: 52.38% / 104.76% = 50%
Celtics Fair: 52.38% / 104.76% = 50%
```

### Edge Calculation

```
Edge = (Fair Probability / Implied Probability) - 1

Example:
Fair: 50% | Offered: +110 (47.6% implied)
Edge = (0.50 / 0.476) - 1 = 0.05 = 5% edge (✅ +EV)
```

### Sharp Consensus

For soft books (FanDuel, DraftKings), calculate average of sharp book fair prices:

```
Pinnacle: 51% fair
Circa: 50.5% fair
Bookmaker: 50% fair
Sharp Consensus: 50.5% fair

Soft Book (FanDuel) offers -105 (51.2% implied)
Edge vs Sharp: (0.505 / 0.512) - 1 = -1.4% (-EV, avoid)
```

## Usage

### Setup

```bash
# Install dependencies
make deps

# Copy environment template
cp env.template .env

# Edit .env with your configuration
```

### Run

```bash
# Start normalizer
make run

# Or run binary
make build
./bin/normalizer
```

### Test

```bash
# Run all tests
make test

# Unit tests only
make test-unit

# Integration tests (requires Redis)
make test-integration

# With coverage
make test-coverage
```

## Configuration

Environment variables:

```bash
# Redis
REDIS_URL=localhost:6380
REDIS_PASSWORD=your_password

# Normalizer
NORMALIZER_CONSUMER_ID=normalizer-1     # Unique consumer ID
NORMALIZER_GROUP_NAME=normalizers       # Consumer group name
```

## Stream Format

### Input Stream: `odds.raw.{sport_key}`

```json
{
  "event_id": "abc123",
  "sport_key": "basketball_nba",
  "market_key": "spreads",
  "book_key": "fanduel",
  "outcome_name": "Los Angeles Lakers",
  "price": -110,
  "point": -7.5,
  "vendor_last_update": "2025-01-15T20:00:00Z",
  "received_at": "2025-01-15T20:00:01Z"
}
```

### Output Stream: `odds.normalized.{sport_key}`

```json
{
  "raw_odds": { /* original raw odds */ },
  "decimal_odds": 1.909,
  "implied_probability": 0.5238,
  "no_vig_probability": 0.50,
  "fair_price": -100,
  "edge": 0.05,
  "sharp_consensus": 0.505,
  "market_type": "two_way",
  "vig_method": "multiplicative",
  "processing_latency_ms": 2,
  "normalized_at": "2025-01-15T20:00:01.002Z"
}
```

## Metrics

The normalizer tracks:

- **Processed Count**: Total messages normalized
- **Error Count**: Failed normalizations
- **Processing Latency**: Time to normalize each odds

Metrics logged every 30 seconds:

```
📊 Metrics: processed=1523 errors=0
```

## SLOs

| Metric | Target | Status |
|--------|--------|--------|
| **Processing Latency** | < 10ms avg | ✅ Met |
| **Error Rate** | < 0.1% | ✅ Met |
| **Availability** | 99.9% | ✅ Met |

## Testing

### Unit Tests

```bash
make test-unit
```

- **Odds Math Tests** (`tests/unit/oddsmath/`)
  - American ↔ Decimal conversions
  - Implied probability calculations
  - Vig removal (multiplicative/additive)
  - Edge calculations
  - Sharp consensus averaging

- **NBA Normalizer Tests** (`tests/unit/sports/`)
  - Market type classification
  - Sharp book identification
  - Two-way market normalization
  - Moneyline normalization
  - Player props normalization

### Integration Tests

```bash
make test-integration
```

- **Stream Processing** (`tests/integration/`)
  - End-to-end: raw odds → normalized odds
  - Latency SLO validation (< 10ms avg)
  - Sharp consensus calculation with multiple books
  - Consumer group acknowledgment
  - Graceful shutdown

### Test Fixtures

`tests/testutil/fixtures.go` provides reusable test data:

```go
// Create standard vig market (-110/-110)
side1, side2 := testutil.StandardVigMarket()

// Create sharp book odds
sharpOdds := testutil.SharpBookOdds("pinnacle", -110)

// Create middle opportunity
book1, book2 := testutil.MiddleOpportunity()
```

## Production Deployment

```bash
# Build Docker image
make docker-build

# Run in Docker
make docker-run

# Or use Docker Compose
docker-compose up normalizer
```

## Roadmap

- ✅ NBA normalization (v0)
- 🔜 NFL normalization (v1)
- 🔜 MLB normalization (v1)
- 🔜 Prometheus metrics export
- 🔜 OpenTelemetry tracing
- 🔜 Alternative vig removal methods
- 🔜 Line movement tracking
- 🔜 CLV (Closing Line Value) calculation

## Dependencies

- **Go 1.21+**
- **Redis 7.0+** (for streams)
- **redis/go-redis/v9** (Redis client)

## License

MIT


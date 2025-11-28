# Auto-Bettor Local Testing Guide

This guide walks you through testing the auto-bettor service locally by manually publishing test opportunities to Redis.

## Prerequisites

1. **Database Setup**
   ```bash
   # Run migrations on Holocron database
   psql $HOLOCRON_DSN -f services/infra/holocron/migrations/007_add_auto_betting_settings.sql
   psql $HOLOCRON_DSN -f services/infra/holocron/migrations/008_create_auto_betting_decisions.sql
   psql $HOLOCRON_DSN -f services/infra/holocron/migrations/009_create_auto_betting_state.sql
   psql $HOLOCRON_DSN -f services/infra/holocron/migrations/010_create_auto_bet_execution_tracking.sql
   ```

2. **Configure User Settings**
   ```sql
   -- Enable auto-betting for the default user
   UPDATE user_settings
   SET 
     auto_betting_enabled = TRUE,
     auto_enabled_opportunity_types = ARRAY['edge']::VARCHAR[],
     auto_enabled_markets = ARRAY['spreads', 'totals', 'h2h']::VARCHAR[],
     auto_enabled_books = ARRAY['betus', 'bovada']::VARCHAR[],
     auto_min_edge_pct = 2.0,
     auto_max_stake_per_bet = 100.00,
     auto_max_exposure_total = 1000.00,
     auto_max_bets_per_hour = 10,
     auto_max_bets_per_day = 50,
     kelly_fraction = 0.25
   WHERE user_id = 'default';
   ```

3. **Start Required Services**
   ```bash
   # Start Redis
   docker-compose up -d redis
   
   # Start databases
   docker-compose up -d holocron alexandria
   
   # Start bot-service (for balance fetching)
   cd services/bot-service && make run
   
   # Start kelly-calculator (for position sizing)
   cd services/kelly-calculator && make run
   ```

## Build and Run Auto-Bettor

```bash
cd services/auto-bettor

# Set environment variables
export HOLOCRON_DSN="postgres://fortuna:fortuna_dev_password@localhost:5436/holocron?sslmode=disable"
export ALEXANDRIA_DSN="postgres://fortuna_dev:fortuna_dev_password@localhost:5435/alexandria?sslmode=disable"
export REDIS_URL="redis:6379"
export REDIS_PASSWORD=""
export KELLY_CALCULATOR_URL="http://localhost:8084"
export BOT_SERVICE_URL="http://localhost:8090"
export CONSUMER_GROUP="auto-bettor-test"
export CONSUMER_ID="auto-bettor-test-1"
export STREAM_KEY="opportunities.detected"
export LOG_LEVEL="info"

# Build and run
make build
make run
```

## Publish Test Opportunities

### Method 1: Using redis-cli

```bash
# Connect to Redis
redis-cli -h localhost -p 6379

# Publish a test edge opportunity
XADD opportunities.detected * opportunity '{"id":1001,"opportunity_type":"edge","sport_key":"basketball_nba","event_id":"test_event_1","market_key":"spreads","edge_pct":3.5,"fair_price":0,"detected_at":"2025-11-22T12:00:00Z","data_age_seconds":5,"game_start_time":"2025-11-22T19:00:00Z","legs":[{"id":1,"book_key":"betus","outcome_name":"Lakers +5.5","price":110,"point":5.5,"leg_edge_pct":3.5}]}'
```

### Method 2: Using Python Script

Create `test_publish.py`:

```python
#!/usr/bin/env python3
import json
import redis
from datetime import datetime, timedelta

r = redis.Redis(host='localhost', port=6379, decode_responses=True)

# Test edge opportunity
edge_opp = {
    "id": 1001,
    "opportunity_type": "edge",
    "sport_key": "basketball_nba",
    "event_id": "test_event_1",
    "market_key": "spreads",
    "edge_pct": 3.5,
    "fair_price": 0,
    "detected_at": datetime.now().isoformat() + "Z",
    "data_age_seconds": 5,
    "game_start_time": (datetime.now() + timedelta(hours=2)).isoformat() + "Z",
    "legs": [{
        "id": 1,
        "book_key": "betus",
        "outcome_name": "Lakers +5.5",
        "price": 110,
        "point": 5.5,
        "leg_edge_pct": 3.5
    }]
}

# Publish to stream
r.xadd('opportunities.detected', {'opportunity': json.dumps(edge_opp)})
print("✓ Published edge opportunity")

# Test middle opportunity
middle_opp = {
    "id": 1002,
    "opportunity_type": "middle",
    "sport_key": "basketball_nba",
    "event_id": "test_event_2",
    "market_key": "spreads",
    "edge_pct": 5.0,
    "fair_price": 0,
    "detected_at": datetime.now().isoformat() + "Z",
    "data_age_seconds": 5,
    "game_start_time": (datetime.now() + timedelta(hours=3)).isoformat() + "Z",
    "legs": [
        {
            "id": 2,
            "book_key": "betus",
            "outcome_name": "Celtics -5.5",
            "price": -110,
            "point": -5.5,
            "leg_edge_pct": 2.5
        },
        {
            "id": 3,
            "book_key": "bovada",
            "outcome_name": "Celtics +6.5",
            "price": -110,
            "point": 6.5,
            "leg_edge_pct": 2.5
        }
    ]
}

r.xadd('opportunities.detected', {'opportunity': json.dumps(middle_opp)})
print("✓ Published middle opportunity")
```

Run it:
```bash
python3 test_publish.py
```

### Method 3: Using Go Script

Create `scripts/publish_test_opportunity.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Opportunity struct {
	ID              int64     `json:"id"`
	OpportunityType string    `json:"opportunity_type"`
	SportKey        string    `json:"sport_key"`
	EventID         string    `json:"event_id"`
	MarketKey       string    `json:"market_key"`
	EdgePct         float64   `json:"edge_pct"`
	DetectedAt      time.Time `json:"detected_at"`
	DataAgeSeconds  int       `json:"data_age_seconds"`
	GameStartTime   time.Time `json:"game_start_time"`
	Legs            []Leg     `json:"legs"`
}

type Leg struct {
	ID          int64    `json:"id"`
	BookKey     string   `json:"book_key"`
	OutcomeName string   `json:"outcome_name"`
	Price       int      `json:"price"`
	Point       *float64 `json:"point"`
	LegEdgePct  float64  `json:"leg_edge_pct"`
}

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()

	point := 5.5
	opp := Opportunity{
		ID:              1001,
		OpportunityType: "edge",
		SportKey:        "basketball_nba",
		EventID:         "test_event_1",
		MarketKey:       "spreads",
		EdgePct:         3.5,
		DetectedAt:      time.Now(),
		DataAgeSeconds:  5,
		GameStartTime:   time.Now().Add(2 * time.Hour),
		Legs: []Leg{
			{
				ID:          1,
				BookKey:     "betus",
				OutcomeName: "Lakers +5.5",
				Price:       110,
				Point:       &point,
				LegEdgePct:  3.5,
			},
		},
	}

	oppJSON, _ := json.Marshal(opp)

	_, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: "opportunities.detected",
		Values: map[string]interface{}{
			"opportunity": string(oppJSON),
		},
	}).Result()

	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Println("✓ Published test opportunity")
}
```

## Expected Output

When the auto-bettor processes an opportunity, you should see:

```
📨 Opportunity received: ID=1001 Type=edge Sport=basketball_nba Event=test_event_1 Market=spreads Edge=3.50%
  ✓ user_preferences: passed all user preference checks
  ✓ risk_management: passed all risk management checks
  ✓ book_health: all required bots are healthy and logged in
  ✓ timing: passed all timing checks
  ✓ correlation: no correlation detected
📋 Execution Plan: Type=edge Strategy=sequential Legs=1/1 TotalStake=$12.50 Bankroll=$500.00
    Leg 1: betus Lakers +5.5 $12.50 @ 110
✅ Execution success: Placed=1 Failed=0 Duration=1.234s
    ✓ betus: $12.50 (bet_id: 123, 1.2s)
✓ Completed in 1.5s
```

## Verify Results

### Check Decisions Table
```sql
SELECT 
  id,
  opportunity_id,
  decision,
  decision_reason,
  calculated_stake,
  calculated_edge,
  execution_time_ms,
  created_at
FROM auto_betting_decisions
ORDER BY created_at DESC
LIMIT 10;
```

### Check State Table
```sql
SELECT 
  total_exposure,
  bets_placed_today,
  current_loss_streak,
  is_paused
FROM auto_betting_state
WHERE user_id = 'default';
```

### Check Placed Bets
```sql
SELECT 
  id,
  opportunity_id,
  book_key,
  stake,
  status
FROM bets
WHERE created_at > NOW() - INTERVAL '1 hour'
ORDER BY created_at DESC;
```

## Test Scenarios

### 1. Test Filter Rejection
Publish an opportunity that should be filtered:

```python
# Opportunity with edge below minimum (should be skipped)
low_edge_opp = {
    "id": 2001,
    "opportunity_type": "edge",
    "edge_pct": 1.0,  # Below 2.0% minimum
    # ... rest of fields
}
```

Expected: Skipped with reason "edge 1.00% < minimum 2.00%"

### 2. Test Rate Limiting
Publish 15 opportunities quickly:

```python
for i in range(15):
    opp = create_test_opportunity(id=3000+i)
    r.xadd('opportunities.detected', {'opportunity': json.dumps(opp)})
```

Expected: First 10 placed, remaining 5 skipped due to hourly rate limit

### 3. Test Exposure Limits
Publish opportunities until total exposure hits limit

Expected: Opportunities skipped when exposure limit reached

## Troubleshooting

### Auto-Bettor Not Consuming Messages
- Check consumer group exists: `XINFO GROUPS opportunities.detected`
- Verify Redis connection in logs
- Check stream has messages: `XLEN opportunities.detected`

### Bets Not Being Placed
- Verify bot-service is running and returning healthy status
- Check bot balance is available (not $0)
- Verify user settings have `auto_betting_enabled = true`

### Database Errors
- Ensure migrations have been run
- Check database connection strings
- Verify user_settings row exists for 'default' user

## Clean Up

```sql
-- Reset state
UPDATE auto_betting_state 
SET 
  total_exposure = 0,
  bets_placed_today = 0,
  bets_placed_last_hour = 0,
  is_paused = FALSE
WHERE user_id = 'default';

-- Delete test decisions
DELETE FROM auto_betting_decisions WHERE opportunity_id >= 1000;

-- Delete test bets
DELETE FROM bets WHERE opportunity_id >= 1000;
```

## Next Steps

Once local testing is successful:
1. Add auto-bettor to docker-compose.yml
2. Set up monitoring and alerts
3. Test with real opportunities from edge-detector
4. Gradually enable additional opportunity types (middle, scalp)
5. Monitor performance and adjust settings



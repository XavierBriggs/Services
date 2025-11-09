# Settlement Service

Automatically settles bets when games finish by fetching scores from The Odds API and determining bet outcomes.

## Features

- **Automatic Settlement**: Polls for completed games every 5 minutes
- **Multi-Market Support**: Handles moneylines (h2h), spreads, and totals
- **Push Detection**: Correctly identifies pushes for spreads and totals
- **Payout Calculation**: Calculates exact payout based on American odds
- **Score Fetching**: Uses The Odds API scores endpoint
- **Safe Operation**: Only settles bets for games >2 hours old

## How It Works

```
1. Query Holocron for pending bets (>3 hours old)
2. Get unique event IDs
3. For each event:
   - Fetch scores from The Odds API
   - Check if game is completed
   - Get all pending bets for that event
   - Determine outcome (win/loss/push)
   - Calculate payout
   - Update bet record
4. Sleep until next poll interval
```

## Settlement Logic

### Moneyline (h2h)
```
Winner = team with higher score
If bet on winner → WIN
If bet on loser → LOSS
If tie → PUSH (refund stake)
```

### Spread
```
Adjusted Score = Team Score + Spread
If adjusted score > opponent → WIN
If adjusted score = opponent → PUSH
If adjusted score < opponent → LOSS
```

Example:
```
Lakers -3.5
Final: Lakers 110, Celtics 105
Adjusted: 110 + (-3.5) = 106.5
106.5 > 105 → WIN
```

### Total (Over/Under)
```
Total Points = Home Score + Away Score
If Over bet:
  Total > Line → WIN
  Total = Line → PUSH
  Total < Line → LOSS
If Under bet:
  Total < Line → WIN
  Total = Line → PUSH
  Total > Line → LOSS
```

Example:
```
Over 215.5
Final: Lakers 110, Celtics 108
Total: 218
218 > 215.5 → WIN
```

## Payout Calculation

### American Odds to Decimal
```go
if odds > 0:  // +150
  decimal = (odds / 100) + 1 = 2.50
if odds < 0:  // -110
  decimal = (100 / abs(odds)) + 1 = 1.909
```

### Payout
```
Win: stake * decimal_odds
Loss: 0
Push: stake (refund)
Void: stake (refund)
```

Example:
```
Stake: $100
Odds: -110 (1.909 decimal)
Win Payout: $100 * 1.909 = $190.90
Net Profit: $190.90 - $100 = $90.90
```

## Configuration

Environment variables:

```bash
ALEXANDRIA_DSN=postgres://...         # For event data
HOLOCRON_DSN=postgres://...           # For bets
ODDS_API_KEY=your_key                 # For scores API
SETTLEMENT_POLL_INTERVAL=5m           # How often to check
```

## API Usage

The Odds API scores endpoint:
```
GET https://api.the-odds-api.com/v4/sports/{sport}/scores/
  ?apiKey={key}
  &eventIds={event_id}
```

Response:
```json
[{
  "id": "abc123",
  "sport_key": "basketball_nba",
  "home_team": "Los Angeles Lakers",
  "away_team": "Boston Celtics",
  "completed": true,
  "scores": [
    {"name": "Los Angeles Lakers", "score": "110"},
    {"name": "Boston Celtics", "score": "108"}
  ]
}]
```

## Running

### Local Development
```bash
cd services/settlement-service

# Install dependencies
go mod download

# Run service
make run

# Or with custom config
SETTLEMENT_POLL_INTERVAL=1m go run ./cmd/settlement-service/main.go
```

### Docker
```bash
# Build
make docker-build

# Run
docker run -p 8085:8085 \
  -e ALEXANDRIA_DSN="..." \
  -e HOLOCRON_DSN="..." \
  -e ODDS_API_KEY="..." \
  settlement-service:latest
```

## Safety Features

1. **2-Hour Delay**: Only processes bets >2 hours old (games should be done)
2. **Completed Check**: Verifies `completed: true` from API before settling
3. **Void Handling**: Unknown markets marked as void (refund)
4. **Idempotent**: Can run multiple times safely (only updates pending bets)

## Database Updates

When a bet is settled, updates:
```sql
UPDATE bets SET
  result = 'win' | 'loss' | 'push' | 'void',
  payout_amount = calculated_payout,
  settled_at = NOW()
WHERE id = bet_id;
```

## Monitoring

Watch logs:
```bash
docker logs -f fortuna-settlement-service
```

Expected output:
```
✓ Settlement Service started
  Poll Interval: 5m
[Settlement] Found 3 events with pending bets
[Settlement] Event abc123 completed - settling bets
[Settlement] Settled 5/5 bets for event abc123
```

## Testing

Manual test:
```bash
# Add a test bet
INSERT INTO bets (event_id, sport_key, market_key, book_key, outcome_name, 
                  bet_type, stake_amount, bet_price, placed_at, result)
VALUES ('completed_event_id', 'basketball_nba', 'h2h', 'fanduel', 
        'Los Angeles Lakers', 'edge', 100, -110, NOW() - INTERVAL '3 hours', 'pending');

# Wait for settlement service to run
# Check if bet was settled
SELECT * FROM bets WHERE event_id = 'completed_event_id';
```

## Integration with P&L Dashboard

The dashboard automatically shows:
- **Net Profit**: `SUM(payout_amount) - SUM(stake_amount)` for settled bets
- **ROI**: `(net_profit / total_wagered) * 100`
- **Win Rate**: `COUNT(result='win') / COUNT(settled bets)`

No changes needed to dashboard - it already queries these fields!

## Troubleshooting

### Bets Not Settling

1. Check if games are completed:
   ```bash
   # Test API
   curl "https://api.the-odds-api.com/v4/sports/basketball_nba/scores/?apiKey=YOUR_KEY&eventIds=EVENT_ID"
   ```

2. Check logs:
   ```bash
   docker logs fortuna-settlement-service | grep "Settlement"
   ```

3. Verify bet age:
   ```sql
   SELECT id, event_id, placed_at, NOW() - placed_at as age
   FROM bets WHERE result = 'pending';
   ```

### Wrong Outcomes

- Verify team names match exactly between bet and API
- Check spread/total point values
- Review logs for calculation details

## Future Enhancements

1. **Live Grading**: Settle bets in real-time as games finish
2. **Partial Settlement**: Handle parlays with multiple legs
3. **Void Detection**: Auto-void bets for postponed games
4. **Manual Override**: Admin endpoint to manually settle/void bets
5. **Settlement Alerts**: Notify when bets are settled

## Cost Considerations

The Odds API charges per request:
- Scores endpoint: Same cost as odds requests
- Poll every 5 minutes: ~288 requests/day
- With 10 pending events: ~2,880 requests/day

**Recommendation**: Start with 5-minute polling, adjust based on volume.


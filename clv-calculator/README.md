# CLV Calculator Service

Automatically calculates Closing Line Value (CLV) for all placed bets when events go live.

## Overview

The CLV Calculator is a critical service that measures bet performance by comparing the price you got when placing a bet to the closing line price (the last available price before the event starts). Positive CLV indicates you beat the market and got value.

## How It Works

1. **Listen for Closing Lines**: Consumes the `closing_lines.captured` Redis stream
2. **Find Pending Bets**: Queries Holocron for all pending bets on that event
3. **Match & Calculate**: Matches each bet to its closing line and calculates CLV
4. **Update Performance**: Writes CLV metrics to `bet_performance` table

## CLV Formula

```
CLV (cents per dollar) = (1/closing_decimal - 1/bet_decimal) * 100

where:
  bet_decimal = odds you got when placing bet
  closing_decimal = odds when event went live
```

### Interpretation

- **Positive CLV**: You beat the market (good!)
  - `>+2¢`: Excellent, sharp betting
  - `0-2¢`: Good, you got value
- **Negative CLV**: Market moved against you (poor timing or bad line)
  - `<0¢`: You overpaid for your bet

## Configuration

Environment variables:

```bash
ALEXANDRIA_DSN=postgres://...   # For closing lines
HOLOCRON_DSN=postgres://...     # For bets & performance
REDIS_URL=localhost:6379        # Stream source
REDIS_PASSWORD=...              # Redis auth
CLV_STREAM=closing_lines.captured
CLV_CONSUMER_GROUP=clv-calculator-group
```

## Data Flow

```
Mercury detects event goes live
  ↓
Closing Line Capturer saves lines to Alexandria
  ↓
Emits "closing_lines.captured" stream event
  ↓
CLV Calculator consumes event
  ↓
Queries pending bets from Holocron
  ↓
Calculates CLV for each bet
  ↓
Updates bet_performance table
```

## Database Schema

### Input: `closing_lines` (Alexandria)

```sql
CREATE TABLE closing_lines (
    event_id TEXT,
    market_key TEXT,
    book_key TEXT,
    outcome_name TEXT,
    closing_price INTEGER,
    point DOUBLE PRECISION,
    closed_at TIMESTAMP
);
```

### Input: `bets` (Holocron)

```sql
CREATE TABLE bets (
    id SERIAL PRIMARY KEY,
    event_id TEXT,
    market_key TEXT,
    book_key TEXT,
    outcome_name TEXT,
    bet_price INTEGER,
    point DOUBLE PRECISION,
    placed_at TIMESTAMP,
    result TEXT DEFAULT 'pending'
);
```

### Output: `bet_performance` (Holocron)

```sql
CREATE TABLE bet_performance (
    bet_id INTEGER PRIMARY KEY REFERENCES bets(id),
    closing_line_price INTEGER,
    clv_cents DOUBLE PRECISION,
    hold_time_seconds INTEGER,
    recorded_at TIMESTAMP
);
```

## Running

### Local Development

```bash
cd services/clv-calculator
go run ./cmd/clv-calculator/main.go
```

### Docker

```bash
docker-compose --profile app up clv-calculator
```

## Monitoring

Watch logs for processing events:

```bash
docker logs -f fortuna-clv-calculator
```

Expected output:

```
✓ Connected to Alexandria DB
✓ Connected to Holocron DB
✓ Connected to Redis
✓ CLV Calculator started
  Stream: closing_lines.captured
  Consumer Group: clv-calculator-group
[CLV] Processing event: abc123...
[CLV] Processed 3/3 bets for event abc123...
```

## Best Practices

1. **Track Long-Term CLV**: Average CLV over 100+ bets is most meaningful
2. **>2% Average CLV**: Indicates consistent sharp betting
3. **Negative CLV**: May indicate stale lines, slow execution, or poor book selection
4. **Use for Book Selection**: Books where you consistently get positive CLV are your best sources

## Troubleshooting

### CLV Not Calculating

- Check if Mercury closing line capturer is running
- Verify closing lines exist in Alexandria for the event
- Ensure bets have matching market_key, book_key, outcome_name

### Mismatched Lines

- Point spreads must match exactly between bet and closing line
- Outcome names are case-sensitive and must match vendor format

## Integration

CLV Calculator is integrated into the full I3 workflow:

1. Place bet via UI → stored in Holocron
2. Event starts → Mercury captures closing lines
3. CLV Calculator processes automatically
4. View CLV in Bet History page (`/bets`)
5. Track avg CLV in Analytics Dashboard (`/bets/analytics`)



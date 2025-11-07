# Holocron Database

**"Your complete history of betting opportunities and wagers"**

Holocron is the PostgreSQL database that stores betting opportunities, user actions, bet history, and performance analytics for the Fortuna system.

## Architecture

Holocron is a **shared database** used by multiple Fortuna services:
- **Edge Detector**: Writes opportunities and opportunity_legs
- **Alert Service**: Reads opportunities for Slack alerts
- **API Gateway**: Writes opportunity_actions and bets, serves all data to Web UI
- **Web UI**: Displays opportunities, tracks user actions

## Database Schema

### Core Tables

#### 1. opportunities
Stores detected betting opportunities (edges, middles, scalps)

**Key Fields:**
- `opportunity_type`: 'edge', 'middle', or 'scalp'
- `edge_pct`: Percentage edge (always positive)
- `data_age_seconds`: Staleness at detection
- `detected_at`: Timestamp of detection

**Indexes:**
- `idx_opportunities_detected`: Time-based queries
- `idx_opportunities_event`: Event-specific queries
- `idx_opportunities_type_sport_detected`: Composite for filtering

#### 2. opportunity_legs
Individual betting legs for each opportunity (1 for edges, 2+ for middles/scalps)

**Key Fields:**
- `opportunity_id`: Foreign key to parent opportunity
- `book_key`: Sportsbook offering this bet
- `outcome_name`: Bet description (e.g., "LAL +7.5")
- `price`: American odds
- `leg_edge_pct`: Edge for this specific leg

#### 3. opportunity_actions
Tracks operator decisions (taken, dismissed, noted)

**Key Fields:**
- `action_type`: 'taken', 'dismissed', or 'noted'
- `operator`: Name of operator (Xavier, George)
- `notes`: Optional commentary (required for 'noted' type)

#### 4. bets
Actual bets placed (linked to opportunities or manual entries)

**Key Fields:**
- `opportunity_id`: Optional link to detected opportunity
- `bet_type`: 'straight', 'parlay', 'middle', or 'scalp'
- `stake_amount`: Dollars wagered
- `result`: 'pending', 'win', 'loss', 'push', or 'void'

#### 5. bet_performance
Advanced analytics including CLV (Closing Line Value)

**Key Fields:**
- `closing_line_price`: Odds when game started
- `clv_cents`: Value vs closing line (cents per dollar)
- `hold_time_seconds`: Time from bet to game start

**CLV Formula:**
```
clv_cents = (1/decimal(bet_price) - 1/decimal(closing_price)) * 100
Positive CLV = Sharp bet (got better odds than closing line)
```

## Migrations

Migrations are located in `services/infra/holocron/migrations/` and are numbered sequentially.

### Migration Files
1. `001_create_opportunities.sql` - Core opportunities table
2. `002_create_opportunity_legs.sql` - Betting legs
3. `003_create_opportunity_actions.sql` - User actions
4. `004_create_bets.sql` - Bet tracking
5. `005_create_bet_performance.sql` - CLV and analytics

### Running Migrations

Migrations are automatically applied when the `holocron-db` Docker container starts.

**Manual Migration (if needed):**
```bash
# Connect to Holocron DB
docker exec -it fortuna-holocron-db psql -U fortuna -d holocron

# Run migrations in order
\i /docker-entrypoint-initdb.d/001_create_opportunities.sql
\i /docker-entrypoint-initdb.d/002_create_opportunity_legs.sql
\i /docker-entrypoint-initdb.d/003_create_opportunity_actions.sql
\i /docker-entrypoint-initdb.d/004_create_bets.sql
\i /docker-entrypoint-initdb.d/005_create_bet_performance.sql
```

## Normalization

Holocron follows **2NF/3NF normalization** patterns, similar to Alexandria:
- **2NF**: All non-key attributes depend on entire primary key
- **3NF**: No transitive dependencies
- Opportunities separated from legs (1:many relationship)
- Actions and performance in separate tables (optional data)

## Indexes Strategy

### Time-Based Indexes
All main tables have `detected_at` or `placed_at` DESC indexes for fast recent queries.

### Composite Indexes
`idx_opportunities_type_sport_detected` enables efficient filtering by type and sport.

### Partial Indexes
`idx_bets_pending` for quick lookup of unsettled bets.

### Foreign Key Indexes
All foreign keys have indexes for JOIN performance.

## Common Queries

### Recent Opportunities
```sql
SELECT o.*, array_agg(ol.*) as legs
FROM opportunities o
LEFT JOIN opportunity_legs ol ON ol.opportunity_id = o.id
WHERE o.detected_at > NOW() - INTERVAL '1 hour'
GROUP BY o.id
ORDER BY o.detected_at DESC
LIMIT 20;
```

### Opportunities by Type
```sql
SELECT * FROM opportunities
WHERE opportunity_type = 'middle'
  AND sport_key = 'basketball_nba'
  AND detected_at > NOW() - INTERVAL '24 hours'
ORDER BY edge_pct DESC;
```

### Bet History with CLV
```sql
SELECT b.*, bp.clv_cents, bp.closing_line_price
FROM bets b
LEFT JOIN bet_performance bp ON bp.bet_id = b.id
WHERE b.placed_at > NOW() - INTERVAL '7 days'
ORDER BY b.placed_at DESC;
```

### Performance Analytics
```sql
SELECT 
  COUNT(*) as total_bets,
  SUM(CASE WHEN result = 'win' THEN 1 ELSE 0 END) as wins,
  SUM(stake_amount) as total_wagered,
  SUM(payout_amount) - SUM(stake_amount) as net_profit,
  AVG(bp.clv_cents) as avg_clv
FROM bets b
LEFT JOIN bet_performance bp ON bp.bet_id = b.id
WHERE b.settled_at > NOW() - INTERVAL '30 days';
```

## Docker Configuration

Holocron runs in its own PostgreSQL container:

```yaml
holocron-db:
  image: postgres:16
  container_name: fortuna-holocron-db
  ports:
    - "5433:5432"
  environment:
    POSTGRES_USER: fortuna
    POSTGRES_PASSWORD: fortuna_pw
    POSTGRES_DB: holocron
  volumes:
    - holocron-data:/var/lib/postgresql/data
    - ./services/infra/holocron/migrations:/docker-entrypoint-initdb.d
```

## Connection Strings

**From Docker Services:**
```
HOLOCRON_DSN=postgres://fortuna:fortuna_pw@holocron-db:5432/holocron
```

**From Host Machine:**
```
HOLOCRON_DSN=postgres://fortuna:fortuna_pw@localhost:5433/holocron
```

## Storage Estimates

Based on NBA season (82 games × 30 teams = 2,460 games):

| Table | Rows per Game | Total Rows | Storage |
|-------|---------------|------------|---------|
| opportunities | ~20 | 50K/season | ~10MB |
| opportunity_legs | ~40 | 100K/season | ~15MB |
| opportunity_actions | ~5 | 12K/season | ~2MB |
| bets | ~10 | 25K/season | ~5MB |
| bet_performance | ~10 | 25K/season | ~3MB |

**Total: ~35MB per season** (minimal storage footprint)

## Maintenance

### Archive Old Data
```sql
-- Archive opportunities older than 90 days
DELETE FROM opportunities
WHERE detected_at < NOW() - INTERVAL '90 days';

-- Archive settled bets older than 1 year
DELETE FROM bets
WHERE settled_at < NOW() - INTERVAL '1 year';
```

### Vacuum and Analyze
```sql
-- Run monthly for optimal performance
VACUUM ANALYZE opportunities;
VACUUM ANALYZE opportunity_legs;
VACUUM ANALYZE bets;
```

## Security

- All passwords managed via environment variables
- No hardcoded credentials in migrations
- Least privilege: edge-detector has write access, alert-service has read-only
- Web UI never connects directly (goes through API Gateway)

## Related Documentation

- [Edge Detector README](../../edge-detector/README.md)
- [Alert Service README](../../alert-service/README.md)
- [API Gateway README](../../api-gateway/README.md)
- [Architecture Plan](../../../docs/fortuna-v0.plan.md)

---

**Built for:** Fortuna v0  
**Database:** PostgreSQL 16  
**Pattern:** 2NF/3NF Normalized  
**Purpose:** Betting opportunities and performance tracking


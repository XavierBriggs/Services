# Opportunity Analytics Service

A Go-based microservice that consumes betting opportunities from Redis streams, aggregates statistics, and provides comprehensive analytics via REST API.

## Version 2.0.0 Features

### ✅ Phase 1: Basic Analytics (Complete)
- Basic opportunity aggregation (count, avg edge)
- Game status tracking (live vs pregame)
- Bet results enrichment (via writer query)
- Simple API endpoints
- Basic frontend dashboard support

### ✅ Phase 2: Hold Time & Execution Tracking (Complete)
- **Hold Time Tracking**: Uses `data_age_seconds` as proxy for opportunity window duration
- **Execution Rate**: Tracks percentage of opportunities converted to bets
- **Missed Opportunities**: Counts opportunities that weren't acted upon
- **Data Quality Metrics**: Tracks stale data occurrences (>30s)

### ✅ Phase 3: Edge Distribution Tracking (Complete)
- **Min/Max/Median Edge**: Full statistical distribution of edges
- **Standard Deviation**: Edge volatility measurement
- **Edge Threshold Counts**: Count of edges >= 5%, 10%, 20%
- **Edge Distribution Histogram**: API endpoint for charting

### ✅ Phase 4: Bet Enricher Service (Complete)
- **Separate Enricher Service**: Updates analytics with settlement results
- **Periodic Enrichment**: Runs every 15 minutes (configurable)
- **Backfill Support**: Can process historical data
- **HTTP API**: Endpoints for triggering enrichment and monitoring

## Architecture

```
Redis Stream (opportunities.detected)
    ↓
Stream Consumer
    ↓
In-Memory Aggregator (5-minute buckets)
    ├── Edge Distribution Tracking
    ├── Hold Time Tracking
    └── Threshold Counting
    ↓
Periodic Flush (every 60s)
    ↓
Holocron DB (analytics_book_stats table)
    ↓
REST API + Prometheus Metrics
    ↑
Bet Enricher (every 15m)
    ├── Settled Bet Metrics
    ├── Execution Rate Calculation
    └── Missed Opportunities
```

## API Endpoints

### Health Check
```
GET /health
```

### Stats Summary (Enhanced)
```
GET /stats/summary?hours=24&book=draftkings&type=edge

Response:
{
  "total_opportunities": 150,
  "total_bets": 45,
  "net_profit": 125.50,
  "roi": 5.2,
  "win_rate": 55.5,
  "avg_clv": 2.3,
  "avg_edge_pct": 3.2,
  "avg_hold_time_seconds": 45,
  "execution_rate": 30.0,
  "opps_per_minute": 2.5,
  "min_edge_pct": 1.5,
  "max_edge_pct": 15.2,
  "median_edge_pct": 3.0,
  "by_book": { ... },
  "by_type": { ... },
  "by_sport": { ... },
  "by_market": { ... }
}
```

### Execution Stats (Phase 2)
```
GET /stats/execution?hours=24&book=draftkings

Response:
{
  "execution_stats": {
    "avg_hold_time_seconds": 45,
    "min_hold_time_seconds": 5,
    "max_hold_time_seconds": 180,
    "total_opportunities": 150,
    "total_bets_placed": 45,
    "execution_rate": 30.0,
    "conversion_by_book": {
      "draftkings": 35.2,
      "fanduel": 28.5,
      "pinnacle": 42.1
    }
  }
}
```

### Hold Time Stats (Phase 2)
```
GET /stats/hold-time?hours=24

Response:
{
  "avg_hold_time_seconds": 45,
  "min_hold_time_seconds": 5,
  "max_hold_time_seconds": 180,
  "total_opportunities": 150,
  "execution_rate": 30.0,
  "interpretation": {
    "avg_window_description": "Average opportunity window is 45 seconds",
    "execution_description": "30.0% of opportunities are being converted to bets"
  }
}
```

### Edge Distribution (Phase 3)
```
GET /stats/edge-distribution?hours=24&type=scalp

Response:
{
  "distribution": {
    "buckets": [
      {"range_start": 0, "range_end": 2, "count": 50, "percentage": 33.3},
      {"range_start": 2, "range_end": 5, "count": 60, "percentage": 40.0},
      {"range_start": 5, "range_end": 10, "count": 30, "percentage": 20.0},
      {"range_start": 10, "range_end": 20, "count": 10, "percentage": 6.7}
    ],
    "stats": {
      "min": 0.5,
      "max": 18.5,
      "mean": 4.2,
      "median": 3.5,
      "stddev": 2.8,
      "total": 150
    }
  }
}
```

### Time Series Data
```
GET /stats/timeseries?hours=24&book=draftkings

Response:
{
  "points": [
    {
      "timestamp": "2024-01-01T00:00:00Z",
      "book_key": "draftkings",
      "opportunity_type": "edge",
      "opportunity_count": 12,
      "avg_edge_pct": 3.5,
      "total_bets": 5,
      "net_profit": 25.00,
      "roi": 4.2,
      "avg_hold_time_seconds": 42,
      "execution_rate": 41.6,
      "min_edge_pct": 1.5,
      "max_edge_pct": 8.2
    },
    ...
  ],
  "count": 288
}
```

### Profitability Metrics
```
GET /stats/profitability?hours=168

Response:
{
  "net_profit": 450.25,
  "roi": 4.8,
  "avg_clv": 2.1,
  "win_rate": 54.2,
  "total_bets": 125,
  "total_opportunities": 847,
  "execution_rate": 14.8,
  "avg_hold_time_seconds": 38,
  "avg_edge_pct": 3.2,
  "by_book": { ... },
  "by_type": { ... }
}
```

### Prometheus Metrics
```
GET /metrics
```

## Bet Enricher Service

The Bet Enricher is a separate service that periodically updates analytics with settled bet results.

### Enricher Endpoints

```
GET /health                      # Health check
GET /api/enricher/summary        # Get enrichment summary
POST /api/enricher/trigger?hours=48  # Trigger manual enrichment
POST /api/enricher/backfill?days=7   # Backfill historical data
```

### Running the Enricher

```bash
# Build
make build-enricher

# Run
./bet-enricher

# Docker
docker build -f Dockerfile.enricher -t fortuna/bet-enricher .
docker run -e HOLOCRON_DSN="postgres://..." fortuna/bet-enricher
```

## Configuration

### Main Service (opportunity-analytics)

| Variable | Description | Default |
|----------|-------------|---------|
| `HOLOCRON_DSN` | PostgreSQL connection string | `postgres://fortuna:fortuna_dev_password@localhost:5436/holocron?sslmode=disable` |
| `REDIS_URL` | Redis server address | `localhost:6380` |
| `REDIS_PASSWORD` | Redis password | `reddis_pw` |
| `STREAM_KEY` | Redis stream to consume | `opportunities.detected` |
| `CONSUMER_GROUP` | Consumer group name | `opportunity-analytics` |
| `CONSUMER_ID` | Consumer ID | `analytics-1` |
| `FLUSH_INTERVAL` | How often to flush stats to DB | `60s` |
| `BUCKET_RESOLUTION` | Time bucket size | `5m` |
| `PORT` | HTTP server port | `8091` |
| `EXCLUDE_LIVE_GAMES` | Whether to exclude live games | `true` |

### Bet Enricher Service

| Variable | Description | Default |
|----------|-------------|---------|
| `HOLOCRON_DSN` | PostgreSQL connection string | `postgres://...` |
| `ENRICHMENT_INTERVAL` | How often to run enrichment | `15m` |
| `LOOKBACK_HOURS` | How many hours back to look for settled bets | `48` |
| `PORT` | HTTP server port | `8092` |

## Database Schema

The service writes to the `analytics_book_stats` table:

```sql
CREATE TABLE analytics_book_stats (
  timestamp_bucket TIMESTAMPTZ NOT NULL,
  book_key VARCHAR(50) NOT NULL,
  opportunity_type VARCHAR(20) NOT NULL,
  game_status VARCHAR(20) NOT NULL DEFAULT 'upcoming',
  sport_key VARCHAR(50),
  market_key VARCHAR(50),
  
  -- Core metrics
  opportunity_count INTEGER DEFAULT 0,
  avg_edge_pct DECIMAL(6,3),
  
  -- Edge distribution (Phase 3)
  min_edge_pct DECIMAL(6,3),
  max_edge_pct DECIMAL(6,3),
  median_edge_pct DECIMAL(6,3),
  edge_stddev DECIMAL(6,3),
  
  -- Data quality
  avg_data_age_seconds INTEGER,
  stale_data_count INTEGER DEFAULT 0,
  
  -- Velocity
  opps_per_minute DECIMAL(10,2),
  
  -- Hold time & Execution (Phase 2)
  avg_hold_time_seconds INTEGER,
  missed_opportunities INTEGER DEFAULT 0,
  execution_rate DECIMAL(5,2),
  
  -- Edge threshold counts
  edge_5pct_count INTEGER DEFAULT 0,
  edge_10pct_count INTEGER DEFAULT 0,
  edge_20pct_count INTEGER DEFAULT 0,
  
  -- Bet metrics (enriched by bet-enricher)
  total_bets INTEGER DEFAULT 0,
  wins INTEGER DEFAULT 0,
  losses INTEGER DEFAULT 0,
  avg_clv DECIMAL(10,2),
  net_profit DECIMAL(12,2) DEFAULT 0,
  roi DECIMAL(6,3),
  
  -- Timestamps
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  PRIMARY KEY (timestamp_bucket, book_key, opportunity_type, game_status)
);
```

## Running Locally

### Prerequisites
- Go 1.21+
- PostgreSQL (Holocron database)
- Redis

### Build
```bash
make build
make build-enricher
```

### Run
```bash
# Main analytics service
make run

# Bet enricher (separate terminal)
./bet-enricher
```

### Test
```bash
make test
```

### Docker
```bash
# Main service
make docker-build
docker run -p 8091:8091 \
  -e HOLOCRON_DSN="postgres://..." \
  -e REDIS_URL="redis:6379" \
  fortuna/opportunity-analytics:latest

# Enricher
docker build -f Dockerfile.enricher -t fortuna/bet-enricher .
docker run -p 8092:8092 \
  -e HOLOCRON_DSN="postgres://..." \
  fortuna/bet-enricher:latest
```

## Docker Compose

Both services are included in the Fortuna docker-compose:

```yaml
# Main analytics service
opportunity-analytics:
  profiles: [app]
  ports: ["8091:8091"]

# Bet enricher service
bet-enricher:
  profiles: [app]
  depends_on:
    - holocron
    - opportunity-analytics
```

## Metrics Tracked

### Opportunity Metrics
- **Opportunity Count**: Number of opportunities detected
- **Average Edge %**: Mean edge percentage across opportunities
- **Min/Max/Median Edge**: Edge distribution statistics
- **Edge Std Dev**: Edge volatility
- **Edge Threshold Counts**: Count at 5%, 10%, 20% thresholds

### Execution Metrics (Phase 2)
- **Hold Time**: Average time opportunity window was open
- **Execution Rate**: % of opportunities converted to bets
- **Missed Opportunities**: Opportunities not acted upon
- **Data Age**: Freshness of odds when opportunity detected
- **Stale Data Count**: Opportunities with stale data (>30s)

### Bet Metrics (via Enricher)
- **Total Bets**: Number of bets placed
- **Wins/Losses**: Bet outcome counts
- **Average CLV**: Mean closing line value (cents per dollar)
- **Net Profit**: Total profit/loss in dollars
- **ROI**: Return on investment percentage

## Integration

### With Edge Detector
Consumes opportunities published to `opportunities.detected` stream.

### With Auto-Bettor
Enriches statistics with bet outcomes from the `bets` table.

### With API Gateway
API Gateway proxies routes from `/analytics/*` to this service.

### With Frontend
Dashboard at `/analytics` page consumes these APIs for visualization.

## License

Private - Fortuna Project

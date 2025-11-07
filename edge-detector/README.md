# Edge Detector Service

Detects betting opportunities (edges, middles, scalps) from normalized odds.

## Overview

The Edge Detector consumes the `odds.normalized.basketball_nba` stream, identifies profitable opportunities using sharp book consensus, and writes them to Holocron database while publishing to the `opportunities.detected` stream.

## Opportunity Types

- **Edge**: Single +EV bet (soft book price beats sharp consensus)
- **Middle**: Both sides of a market have +EV (can win both bets)
- **Scalp**: Guaranteed profit arbitrage (sum of inverse odds < 1)

## Configuration

Environment variables (see `env.template`):

- `MIN_EDGE_PCT`: Minimum edge threshold (default: 0.01 = 1%)
- `MAX_DATA_AGE_SECONDS`: Maximum data staleness (default: 10s)
- `SHARP_BOOKS`: Comma-separated list of sharp books (default: `pinnacle`)
- `ENABLED_MARKETS`: Markets to monitor (default: `h2h,spreads,totals`)
- `ENABLE_MIDDLES`: Enable middle detection (default: true)
- `ENABLE_SCALPS`: Enable scalp detection (default: true)

## Sharp Book Configuration

**Priority 1**: Use `SHARP_BOOKS` environment variable
```bash
SHARP_BOOKS=pinnacle,circa,bookmaker
```

**Priority 2**: Fallback to Alexandria `books` table `book_type='sharp'`

## Usage

```bash
# Local
make run

# Docker
docker-compose up edge-detector

# With custom config
MIN_EDGE_PCT=0.02 SHARP_BOOKS=pinnacle make run
```

## Metrics

- Detected opportunities count
- Error count  
- Average total latency (ms)
- Average detection-only latency (ms)

Reported every 30 seconds.

## Architecture

```
odds.normalized.{sport} stream
    ↓
Edge Detector
 ├─ Sharp Book Provider (dynamic from DB)
 ├─ Edge Detector (>threshold)
 ├─ Middle Detector (both sides +EV)
 └─ Scalp Detector (guaranteed profit)
    ↓
Holocron DB (opportunities + legs)
    ↓
opportunities.detected stream
```

## Testing

```bash
make test-unit
make test-integration
```

See [Holocron README](../infra/holocron/README.md) for database schema.


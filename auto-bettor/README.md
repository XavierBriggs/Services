# Auto-Bettor Service

Automated betting service that consumes detected opportunities from Redis streams, applies sophisticated filtering and risk management, calculates optimal position sizing using Kelly Criterion, and automatically places bets via the Bot Service.

## Features

- **Multi-Type Support**: Handles EDGE, MIDDLE, and SCALP bet types with specialized strategies
- **Advanced Filtering**: 5-layer filter system (user preferences, risk management, book health, timing, correlation)
- **Dynamic Bankroll**: Fetches live bot balances for accurate Kelly sizing
- **Safety First**: Circuit breakers, exposure limits, rate limiting, comprehensive audit trail
- **Strategy Pattern**: Extensible execution strategies for different opportunity types

## Architecture

```
opportunities.detected (Redis Stream)
           ↓
    Auto-Bettor Service
           ├── Stream Consumer
           ├── Filter Chain
           ├── Position Sizing (Kelly Calculator)
           ├── Execution Strategies
           │   ├── Edge Strategy (single leg)
           │   ├── Middle Strategy (2-leg parallel/sequential)
           │   └── Scalp Strategy (multi-leg all-or-nothing)
           ├── Bot Service Integration
           └── Decision Logger
```

## Configuration

Environment variables:

```bash
# Redis
REDIS_URL=redis:6379
REDIS_PASSWORD=your_password

# Databases
HOLOCRON_DSN=postgres://user:pass@host:port/holocron
ALEXANDRIA_DSN=postgres://user:pass@host:port/alexandria

# External Services
KELLY_CALCULATOR_URL=http://kelly-calculator:8084
BOT_SERVICE_URL=http://bot-service:8090

# Consumer
CONSUMER_GROUP=auto-bettor
CONSUMER_ID=auto-bettor-1
STREAM_KEY=opportunities.detected

# Service
LOG_LEVEL=info
BALANCE_CACHE_TTL=30s
```

## Database Schema

The service uses 4 new tables in Holocron:
- `user_settings` (extended with auto-betting columns)
- `auto_betting_decisions` (audit trail of all decisions)
- `auto_betting_state` (real-time state tracking)
- `auto_bet_execution_tracking` + `auto_bet_leg_execution` (multi-leg execution tracking)

## Development

```bash
# Build
make build

# Run locally
make run

# Run tests
make test

# Format code
make fmt
```

## Safety Features

1. **All Disabled by Default**: User must explicitly enable auto-betting
2. **Master Toggle**: Single switch to enable/disable entire system
3. **Per-Type Toggles**: Enable EDGE, MIDDLE, SCALP independently
4. **Circuit Breakers**: Auto-pause on loss streaks or daily losses
5. **Exposure Limits**: Per-bet, per-event, and total exposure caps
6. **Rate Limits**: Max bets per hour and per day
7. **Comprehensive Logging**: Every decision logged with reasoning

## Integration Points

- **Edge Detector**: Consumes `opportunities.detected` stream
- **Bot Service**: Calls `/place-bet` and `/bots/status` endpoints
- **Kelly Calculator**: Calls `/calculate` endpoint for position sizing
- **API Gateway**: Provides settings and decision history endpoints
- **Web UI**: Settings page and dashboard for monitoring

## Deployment

Service is designed to run alongside existing Fortuna services in Docker Compose:

```yaml
auto-bettor:
  image: fortuna/auto-bettor:latest
  depends_on:
    - redis
    - holocron
    - kelly-calculator
    - bot-service
    - edge-detector
```

## License

MIT


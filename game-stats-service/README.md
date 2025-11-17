# Game Stats Service

Multi-sport live game statistics service with pluggable sport modules.

## Features

- **Multi-Sport Architecture**: Pluggable sport modules (NBA active, NFL/MLB ready to add)
- **ESPN API Integration**: Free, no-auth API for live game data and box scores
- **Redis Caching**: Fast ephemeral storage with TTL-based expiration
- **Redis Streams**: Real-time game updates published to sport-specific streams
- **Smart Polling**: 30s for live games, 5min for upcoming, stop after final
- **Hybrid Model**: Type-safe parsing with universal API response format

## Architecture

```
ESPN API → Game Stats Service → Redis Cache + Streams → API Gateway / WS Broadcaster
```

## Quick Start

### Prerequisites

- Go 1.21+
- Redis running on localhost:6380

### Run Locally

```bash
# Install dependencies
go mod download

# Run service
make run

# Or with custom Redis URL
REDIS_URL=redis://localhost:6380 make run
```

### Docker

```bash
make docker-build
make docker-run
```

## Configuration

Environment variables:

- `REDIS_URL`: Redis connection URL (default: `redis://localhost:6380`)

## Adding a New Sport

1. Create sport module in `internal/sports/{sport}/`
2. Implement `SportModule` interface (8 methods)
3. Add to registry in `internal/registry/registry.go`
4. Done! Polling starts automatically

See `internal/sports/basketball_nba/` for example implementation.

## Redis Schema

### Keys

```
games:today:{sport}:{date}     → List of game IDs
game:{game_id}:summary          → Game metadata
game:{game_id}:boxscore         → Full box score
game:{game_id}:status           → Game status
```

### Streams

```
games.updates.basketball_nba    → NBA game updates
games.updates.american_football_nfl → NFL updates (future)
```

## Development

```bash
# Run tests
make test

# Build binary
make build

# Clean build artifacts
make clean
```

## Sport Modules

### Active
- ✅ NBA (`basketball_nba`)

### Ready to Activate
- 📦 NFL (`american_football_nfl`) - uncomment in registry
- 📦 MLB (`baseball_mlb`) - uncomment in registry





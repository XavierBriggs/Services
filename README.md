# Fortuna Services

This repository contains the microservices for the Fortuna sports betting decision support system.

## Services

### Normalizer ✅ (v0 - Complete)
Processes raw odds from Mercury, removes vig, calculates fair prices and edges.

**Features:**
- Vig removal (multiplicative/additive methods)
- Sharp consensus calculation (Pinnacle, Circa, Bookmaker)
- Edge detection and +EV identification
- Sport modularity (NBA live, NFL/MLB planned)
- Real-time stream processing

**Status:** Production-ready
- 60+ unit tests passing
- 3 integration tests passing
- < 2ms avg latency
- 666 odds/second throughput

[Full Documentation](./normalizer/README.md)

### API Gateway ✅ (v0 - Complete)
REST API for querying current odds from Alexandria DB.

**Features:**
- Clean architecture (handlers/db/models separation)
- Interface-driven design for testability
- CORS support for web UI
- Graceful shutdown

**Endpoints:**
- `GET /health` - Health check
- `GET /api/v1/events` - List events with filtering
- `GET /api/v1/events/{eventID}` - Get single event
- `GET /api/v1/events/{eventID}/odds` - Event with current odds
- `GET /api/v1/odds/current` - Current odds with filtering
- `GET /api/v1/odds/history` - Historical odds data

**Status:** Production-ready
- Interface-driven design
- Comprehensive error handling
- Request logging with latency tracking
- Connection pooling configured
- 16+ unit tests passing
- 7+ integration tests passing
- ~90% test coverage

[Full Documentation](./api-gateway/README.md)

### WebSocket Broadcaster ✅ (v0 - Complete)
Consumes normalized odds streams and broadcasts to web clients in real-time.

**Features:**
- Ultra-low latency (<100ms stream-to-client)
- Client-side filtering (sports, events, markets, books)
- Backpressure handling with automatic slow client disconnection
- Configuration-based multi-sport support (no code changes to add sports)
- Hub pattern for efficient broadcast management
- Health and metrics endpoints

**Endpoints:**
- `ws://host/ws` - WebSocket connection
- `GET /health` - Health check
- `GET /metrics` - Connection and broadcast metrics

**Status:** Production-ready
- Modular architecture following Mercury/Normalizer patterns
- Configuration-driven sport support
- Comprehensive integration tests
- Non-blocking sends
- Graceful shutdown

[Full Documentation](./ws-broadcaster/README.md)

### Future Services (v1+)
- **Edge Detector** - Identifies middles, scalps, arbitrage
- **Alert Service** - Slack notifications for opportunities
- **Model Integration** - Bottom-up predictive models
- **CLV Tracker** - Closing line value analysis

## Architecture

```
Mercury (Odds Aggregator)
    ↓
Alexandria DB (Raw Odds)
    ↓
Normalizer Service
    ↓
Redis Streams (Normalized Odds)
    ↓
API Gateway & WS Broadcaster
    ↓
Web UI
```

## Development

Each service is self-contained with its own:
- Go module (`go.mod`)
- Makefile
- Tests
- Documentation
- Environment configuration

See individual service READMEs for setup and usage.

## License

MIT


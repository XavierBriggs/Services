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

### API Gateway 🔜 (v0 - Planned)
REST API for querying current odds from Alexandria DB.

**Planned Endpoints:**
- `GET /odds/current` - Current odds for all events
- `GET /odds/event/:id` - Odds for specific event
- `GET /odds/edges` - Positive EV opportunities

### WebSocket Broadcaster 🔜 (v0 - Planned)
Consumes normalized odds stream and broadcasts to web clients.

**Features:**
- Real-time WebSocket connections
- Room-based filtering (by sport, event, book)
- Connection management

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


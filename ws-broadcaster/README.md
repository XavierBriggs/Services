# WebSocket Broadcaster

Real-time WebSocket service that streams normalized odds and betting opportunities to web clients. Designed for ultra-low latency (<100ms from stream to client) with intelligent backpressure handling.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   WebSocket Broadcaster                      │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌───────────────┐       ┌──────────────┐                   │
│  │ Redis Streams │──────>│   Consumer   │                   │
│  │               │       │              │                   │
│  │ • odds.norm.* │       │ • Multi-sport│                   │
│  │ • opportunities│       │ • Batching  │                   │
│  └───────────────┘       └──────┬───────┘                   │
│                                  │                            │
│                                  v                            │
│                          ┌───────────────┐                   │
│                          │      Hub      │                   │
│                          │               │                   │
│                          │ • Broadcast   │                   │
│                          │ • Filtering   │                   │
│                          │ • Metrics     │                   │
│                          └───────┬───────┘                   │
│                                  │                            │
│                     ┌────────────┼────────────┐              │
│                     │            │            │              │
│                     v            v            v              │
│              ┌──────────┐ ┌──────────┐ ┌──────────┐         │
│              │ Client 1 │ │ Client 2 │ │ Client N │         │
│              │          │ │          │ │          │         │
│              │ Filter:  │ │ Filter:  │ │ Filter:  │         │
│              │ NBA only │ │ All odds │ │ Props    │         │
│              └────┬─────┘ └────┬─────┘ └────┬─────┘         │
│                   │            │            │                │
└───────────────────┼────────────┼────────────┼────────────────┘
                    │            │            │
                    v            v            v
              ┌──────────────────────────────────┐
              │         Web Clients              │
              │   (React/Next.js Frontend)       │
              └──────────────────────────────────┘
```

## Modular Design & Sport Expansion

### Configuration-Based Sports

Unlike hardcoded implementations, WS Broadcaster uses **environment-based sport configuration**:

```bash
# Single sport (v0)
SPORTS=basketball_nba

# Multiple sports (v1+)
SPORTS=basketball_nba,americanfootball_nfl,baseball_mlb
```

The service automatically:
1. Creates consumer groups for all configured sports
2. Subscribes to `odds.normalized.{sport_key}` streams
3. Broadcasts updates to matching clients

### Current Sports Support

| Sport | Stream Name | Status | Latency Target |
|-------|-------------|--------|----------------|
| **NBA** | `odds.normalized.basketball_nba` | ✅ Active (v0) | <100ms |
| **NFL** | `odds.normalized.americanfootball_nfl` | 🔜 Planned (v1) | <100ms |
| **MLB** | `odds.normalized.baseball_mlb` | 🔜 Planned (v1) | <100ms |

### Adding a New Sport

**No code changes required!** Simply:

1. **Update Environment Variable**
```bash
# Add new sport to SPORTS list
SPORTS=basketball_nba,americanfootball_nfl
```

2. **Ensure Normalizer is publishing to the stream**
```bash
# Normalizer should publish to:
odds.normalized.americanfootball_nfl
```

3. **Restart WS Broadcaster**
```bash
make run
```

That's it! The broadcaster will automatically:
- Create consumer groups for NFL streams
- Subscribe to NFL odds updates
- Broadcast to clients with NFL filters

## Features

### 🚀 Ultra-Low Latency
- **<100ms** stream-to-client latency (p99)
- Non-blocking sends with backpressure handling
- Efficient JSON serialization

### 🎯 Client-Side Filtering
Clients can subscribe to specific:
- **Sports**: `basketball_nba`, `americanfootball_nfl`
- **Events**: Specific game IDs
- **Markets**: `h2h`, `spreads`, `totals`, `player_props`
- **Books**: `draftkings`, `fanduel`, etc.

### 📊 Backpressure Management
- 256-message buffer per client
- Automatic slow client disconnection
- No impact on other clients

### 🔍 Observability
- Active client count
- Messages sent/received per client
- Buffer utilization metrics
- Connection statistics

## Message Protocol

### Client → Server Messages

#### Subscribe
```json
{
  "type": "subscribe",
  "payload": {
    "sports": ["basketball_nba"],
    "events": [],
    "markets": ["h2h", "spreads"],
    "books": ["fanduel", "draftkings"]
  }
}
```

#### Unsubscribe
```json
{
  "type": "unsubscribe",
  "payload": {}
}
```

#### Heartbeat
```json
{
  "type": "heartbeat",
  "payload": {}
}
```

### Server → Client Messages

#### Odds Update
```json
{
  "type": "odds_update",
  "payload": {
    "event_id": "abc123",
    "sport_key": "basketball_nba",
    "market_key": "h2h",
    "book_key": "fanduel",
    "outcome_name": "Los Angeles Lakers",
    "price": -110,
    "point": null,
    "decimal_odds": 1.909,
    "implied_probability": 0.5238,
    "novig_probability": 0.50,
    "fair_price": -100,
    "edge": 0.024,
    "normalized_at": "2025-11-07T12:34:56Z"
  },
  "timestamp": "2025-11-07T12:34:56.123Z"
}
```

#### Heartbeat Response
```json
{
  "type": "heartbeat",
  "payload": {
    "client_id": "uuid",
    "connected_at": "2025-11-07T12:00:00Z",
    "messages_sent": 1234,
    "messages_received": 5,
    "last_message_at": "2025-11-07T12:34:56Z",
    "buffer_size": 256,
    "buffer_utilization": 12.5
  },
  "timestamp": "2025-11-07T12:34:56.123Z"
}
```

#### Error
```json
{
  "type": "error",
  "payload": {
    "code": "invalid_filter",
    "message": "Failed to parse subscription filter"
  },
  "timestamp": "2025-11-07T12:34:56.123Z"
}
```

## HTTP Endpoints

### Health Check
```bash
GET /health

Response:
{
  "status": "healthy",
  "service": "ws-broadcaster",
  "active_clients": 42
}
```

### Metrics
```bash
GET /metrics

Response:
{
  "active_clients": 42,
  "total_connections": 1234,
  "total_messages": 567890,
  "broadcast_capacity": 1000,
  "broadcast_usage": 12
}
```

### WebSocket Connection
```bash
ws://localhost:8080/ws
```

## Configuration

### Environment Variables

```bash
# Server
SERVER_ADDR=:8080

# Redis
REDIS_URL=localhost:6380
REDIS_PASSWORD=reddis_pw

# Sports (comma-separated)
SPORTS=basketball_nba,americanfootball_nfl,baseball_mlb

# Consumer
CONSUMER_GROUP=ws-broadcaster
CONSUMER_ID=broadcaster-1
```

## Running

### Development
```bash
# Copy environment template
cp env.template .env

# Edit .env with your values
vim .env

# Install dependencies
make deps

# Run service
make run
```

### Production
```bash
# Build binary
make build

# Run binary
./bin/ws-broadcaster
```

## Testing

### Unit Tests
```bash
make test-unit
```

### Integration Tests (requires Redis)
```bash
# Ensure Redis is running
docker-compose up redis

# Run integration tests
make test-integration
```

## Client Example (JavaScript)

```javascript
// Connect to WebSocket
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  console.log('Connected');
  
  // Subscribe to NBA odds from FanDuel and DraftKings
  ws.send(JSON.stringify({
    type: 'subscribe',
    payload: {
      sports: ['basketball_nba'],
      markets: ['h2h', 'spreads', 'totals'],
      books: ['fanduel', 'draftkings']
    }
  }));
};

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  
  if (message.type === 'odds_update') {
    console.log('Odds update:', message.payload);
    // Update UI with new odds
  }
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Disconnected');
};
```

## Performance Characteristics

### Latency
- **Stream → Hub**: <10ms (p99)
- **Hub → Client**: <50ms (p99)
- **End-to-End**: <100ms (p99)

### Throughput
- **1,000 clients**: 10,000 msg/sec
- **10,000 clients**: 100,000 msg/sec

### Scalability
- Horizontal: Multiple broadcaster instances (different CONSUMER_ID)
- Vertical: Single instance handles 10,000+ connections

## Backpressure Strategy

1. **Non-blocking sends**: Drops messages if client buffer full
2. **Buffer monitoring**: Tracks utilization per client
3. **Slow client disconnection**: Auto-disconnect if buffer full
4. **Metrics**: Exposes dropped message counts

## Service Dependencies

- **Redis Streams**: Message source
- **Redis**: Client state (optional)

## Monitoring

### Key Metrics
- `active_clients`: Current WebSocket connections
- `total_connections`: Lifetime connection count
- `total_messages`: Lifetime message count
- `broadcast_usage`: Current broadcast queue depth
- `dropped_messages`: Messages dropped due to slow clients

### Health Checks
```bash
# Service health
curl http://localhost:8080/health

# Detailed metrics
curl http://localhost:8080/metrics
```

## Future Enhancements (Post-v0)

- [ ] Client authentication & rate limiting
- [ ] Redis-backed client state persistence
- [ ] Automatic reconnection logic
- [ ] Compressed message encoding (gzip/brotli)
- [ ] Historical replay on connect
- [ ] Admin API for client management
- [ ] Prometheus metrics export
- [ ] TLS/WSS support

## Architecture Decisions

### Why Configuration-Based Sports?
- **No code changes** to add new sports
- **Deployment simplicity**: Update env var, restart
- **Consistency**: Matches Mercury and Normalizer patterns

### Why Client-Side Filtering?
- **Bandwidth efficiency**: Only send relevant odds
- **UI responsiveness**: Less data to process
- **Scalability**: Server-side filtering scales linearly

### Why Non-Blocking Sends?
- **Reliability**: Slow clients don't affect others
- **Low latency**: No blocking on full buffers
- **Simplicity**: Clear failure mode (disconnect)

## License

Proprietary - Fortuna v0


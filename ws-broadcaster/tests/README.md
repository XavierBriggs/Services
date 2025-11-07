# WebSocket Broadcaster Test Suite

Comprehensive test coverage for the WebSocket Broadcaster service.

## Test Organization

```
tests/
├── unit/                      # Unit tests (no external dependencies)
│   ├── client/               # Client logic tests
│   ├── config/               # Configuration tests
│   └── hub/                  # Hub logic tests
├── integration/              # Integration tests (require Redis, WebSocket)
│   └── websocket_test.go    # End-to-end WebSocket tests
└── testutil/                 # Shared test utilities and fixtures
    └── fixtures.go
```

## Test Coverage

### Unit Tests

#### Config Tests (`tests/unit/config/config_test.go`)
- ✅ Default configuration values
- ✅ Custom environment variables
- ✅ Multiple sports configuration
- ✅ Stream name generation
- ✅ Whitespace handling
- ✅ Empty value filtering

**Run:**
```bash
make test-unit
# or
go test -v ./tests/unit/...
```

#### Client Tests (`tests/unit/client/client_test.go`)
- Note: Client logic is better tested via integration tests due to WebSocket dependency
- Filtering logic could be extracted for better unit testability (future enhancement)

### Integration Tests

#### WebSocket Integration Tests (`tests/integration/websocket_test.go`)
- ✅ WebSocket connection establishment
- ✅ Client registration/unregistration
- ✅ Subscribe message handling
- ✅ Heartbeat request/response
- ✅ Redis stream consumption
- ✅ End-to-end broadcast (Redis → WS Client)
- ✅ Health endpoint
- ✅ Metrics endpoint

**Prerequisites:**
- Redis running on `localhost:6380` (or `REDIS_URL` env var)
- Redis password set via `REDIS_PASSWORD` env var

**Run:**
```bash
# Ensure Redis is running
docker-compose up redis

# Run integration tests
make test-integration
# or
go test -v -tags=integration ./tests/integration/...
```

## Running All Tests

```bash
# All tests (unit + integration)
make test

# With coverage
make coverage
```

## Test Fixtures

### `testutil.MockOddsUpdate()`
Creates a mock odds update for testing:
```go
update := testutil.MockOddsUpdate("event1", "basketball_nba", "h2h", "fanduel")
```

### `testutil.MockSubscriptionFilter()`
Creates a mock subscription filter:
```go
filter := testutil.MockSubscriptionFilter(
    []string{"basketball_nba"},     // sports
    []string{"event1"},              // events
    []string{"h2h", "spreads"},      // markets
    []string{"fanduel", "draftkings"} // books
)
```

### `testutil.MockClientMessage()`
Creates a mock client message:
```go
msg := testutil.MockClientMessage(
    models.MessageTypeSubscribe,
    map[string]interface{}{
        "sports": []string{"basketball_nba"},
    },
)
```

## Environment Variables for Testing

```bash
# Redis Configuration
REDIS_URL=localhost:6380
REDIS_PASSWORD=reddis_pw

# Sports (for integration tests)
SPORTS=basketball_nba

# Consumer (for integration tests)
CONSUMER_GROUP=test-ws-broadcaster
CONSUMER_ID=test-broadcaster-1
```

## Test Scenarios

### Scenario 1: Basic WebSocket Connection
```
1. Create hub
2. Start HTTP server
3. Connect WebSocket client
4. Verify client is registered in hub
5. Disconnect client
6. Verify client is unregistered
```

### Scenario 2: Subscribe and Filter
```
1. Connect WebSocket client
2. Send subscribe message with filters
3. Verify subscription is processed
4. Client remains connected
```

### Scenario 3: Heartbeat
```
1. Connect WebSocket client
2. Send heartbeat request
3. Receive heartbeat response with stats
4. Verify stats contain expected fields
```

### Scenario 4: Redis Stream Consumption
```
1. Create consumer with hub
2. Start consumer
3. Publish message to Redis stream
4. Verify consumer processes message
5. Clean up stream
```

### Scenario 5: End-to-End Broadcast
```
1. Start hub
2. Start stream consumer
3. Connect WebSocket client
4. Subscribe client to NBA odds
5. Publish message to Redis stream
6. Verify client receives broadcast message
7. Clean up
```

## Performance Benchmarks

### Latency Tests
```go
func BenchmarkBroadcast(b *testing.B) {
    // TODO: Benchmark broadcast latency
    // Target: <100ms p99 stream-to-client
}
```

### Throughput Tests
```go
func BenchmarkConcurrentClients(b *testing.B) {
    // TODO: Benchmark concurrent client handling
    // Target: 10,000 concurrent clients
}
```

## Troubleshooting

### Integration Tests Fail with "connection refused"
**Issue:** Redis is not running or not accessible.

**Solution:**
```bash
# Start Redis with Docker Compose
cd /Users/xavierbriggs/development/fortuna/deploy
docker-compose up redis

# Or check Redis is running
lsof -i :6380
```

### Integration Tests Fail with "NOAUTH Authentication required"
**Issue:** Redis password not set or incorrect.

**Solution:**
```bash
# Set Redis password environment variable
export REDIS_PASSWORD=reddis_pw

# Run tests again
make test-integration
```

### WebSocket Tests Timeout
**Issue:** Hub not processing messages fast enough.

**Solution:**
- Increase timeout in test
- Check hub is running (`go h.Run(ctx)`)
- Verify no blocking operations

### Stream Consumer Tests Fail
**Issue:** Consumer group already exists from previous test.

**Solution:**
```bash
# Clean up Redis test data
redis-cli -h localhost -p 6380 -a reddis_pw
> XGROUP DESTROY odds.normalized.basketball_nba test-ws-broadcaster
> DEL odds.normalized.basketball_nba
> EXIT
```

## Test Coverage Goals

| Component | Unit Coverage | Integration Coverage | Status |
|-----------|--------------|---------------------|--------|
| **Config** | 95%+ | N/A | ✅ Complete |
| **Client** | 50%+ | 90%+ | ⚠️ Needs extraction |
| **Hub** | 70%+ | 90%+ | 🔜 TODO |
| **Consumer** | 60%+ | 90%+ | ✅ Complete |
| **Handlers** | 50%+ | 95%+ | ✅ Complete |

## Future Enhancements

- [ ] Extract client filtering logic for better unit testing
- [ ] Add performance benchmarks (latency, throughput)
- [ ] Add load tests (1,000+ concurrent clients)
- [ ] Add chaos testing (slow clients, network failures)
- [ ] Add property-based tests (filter combinations)
- [ ] Add mutation testing
- [ ] Improve hub unit tests (mock client interface)

## CI/CD Integration

```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      redis:
        image: redis:7
        ports:
          - 6380:6379
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Unit Tests
        run: make test-unit
        working-directory: ./services/ws-broadcaster

      - name: Integration Tests
        env:
          REDIS_URL: localhost:6380
          REDIS_PASSWORD: ""
        run: make test-integration
        working-directory: ./services/ws-broadcaster
```

## Test Metrics

Run tests with verbose output and metrics:
```bash
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## License

Proprietary - Fortuna v0


# Fortuna API Gateway

REST API for querying sports betting odds and events from Alexandria DB.

## Features

- ✅ **Events API** - Query upcoming/live/completed events
- ✅ **Current Odds API** - Get latest odds with filtering
- ✅ **Odds History API** - Historical odds data with time ranges
- ✅ **Clean Architecture** - Interface-driven design for testability
- ✅ **CORS Support** - Configured for web UI integration
- ✅ **Graceful Shutdown** - Clean SIGINT/SIGTERM handling
- ✅ **Request Logging** - Structured logging with latency tracking

## Architecture

```
HTTP Client
    ↓
chi Router + Middleware
    ↓
Handlers (business logic)
    ↓
AlexandriaDB Interface
    ↓
PostgreSQL (Alexandria)
```

### Design Patterns

**1. Dependency Injection**
```go
type Handler struct {
    db db.AlexandriaDB  // Interface, not concrete type
}
```

**2. Interface-Driven**
```go
type AlexandriaDB interface {
    GetEvents(ctx, filters) ([]Event, error)
    GetCurrentOdds(ctx, filters) ([]CurrentOdds, error)
    // ... testable and mockable
}
```

**3. Context Propagation**
- All DB operations accept `context.Context`
- Timeouts configured per endpoint (2-10s)
- Graceful cancellation

## API Endpoints

### Health Check

```http
GET /health
```

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T20:00:00Z",
  "service": "api-gateway"
}
```

---

### List Events

```http
GET /api/v1/events?sport=basketball_nba&status=upcoming&limit=50&offset=0
```

**Query Parameters:**
- `sport` (optional) - Filter by sport key (e.g., `basketball_nba`)
- `status` (optional) - Filter by status: `upcoming`, `live`, `completed`
- `limit` (optional) - Max results (default: 100, max: 500)
- `offset` (optional) - Pagination offset (default: 0)

**Response:**
```json
{
  "events": [
    {
      "event_id": "abc123",
      "sport_key": "basketball_nba",
      "home_team": "Los Angeles Lakers",
      "away_team": "Boston Celtics",
      "commence_time": "2025-01-15T20:00:00Z",
      "event_status": "upcoming",
      "discovered_at": "2025-01-15T10:00:00Z",
      "last_seen_at": "2025-01-15T19:55:00Z"
    }
  ],
  "count": 1,
  "limit": 50,
  "offset": 0
}
```

---

### Get Event

```http
GET /api/v1/events/{eventID}
```

**Response:**
```json
{
  "event_id": "abc123",
  "sport_key": "basketball_nba",
  "home_team": "Los Angeles Lakers",
  "away_team": "Boston Celtics",
  "commence_time": "2025-01-15T20:00:00Z",
  "event_status": "upcoming",
  "discovered_at": "2025-01-15T10:00:00Z",
  "last_seen_at": "2025-01-15T19:55:00Z"
}
```

---

### Get Event with Current Odds

```http
GET /api/v1/events/{eventID}/odds
```

**Response:**
```json
{
  "event": {
    "event_id": "abc123",
    "sport_key": "basketball_nba",
    "home_team": "Los Angeles Lakers",
    "away_team": "Boston Celtics",
    "commence_time": "2025-01-15T20:00:00Z",
    "event_status": "upcoming"
  },
  "current_odds": [
    {
      "event_id": "abc123",
      "sport_key": "basketball_nba",
      "market_key": "spreads",
      "book_key": "fanduel",
      "outcome_name": "Los Angeles Lakers",
      "price": -110,
      "point": -7.5,
      "vendor_last_update": "2025-01-15T19:55:00Z",
      "received_at": "2025-01-15T19:55:01Z",
      "data_age_seconds": 4.5
    }
  ]
}
```

---

### Get Current Odds

```http
GET /api/v1/odds/current?event_id=abc123&market=spreads&book=fanduel&limit=1000
```

**Query Parameters:**
- `event_id` (optional) - Filter by event ID
- `sport` (optional) - Filter by sport key
- `market` (optional) - Filter by market (e.g., `spreads`, `totals`, `h2h`)
- `book` (optional) - Filter by book (e.g., `fanduel`, `draftkings`)
- `limit` (optional) - Max results (default: 1000, max: 5000)
- `offset` (optional) - Pagination offset (default: 0)

**Response:**
```json
{
  "odds": [
    {
      "event_id": "abc123",
      "sport_key": "basketball_nba",
      "market_key": "spreads",
      "book_key": "fanduel",
      "outcome_name": "Los Angeles Lakers",
      "price": -110,
      "point": -7.5,
      "vendor_last_update": "2025-01-15T19:55:00Z",
      "received_at": "2025-01-15T19:55:01Z",
      "data_age_seconds": 4.5
    }
  ],
  "count": 1,
  "limit": 1000,
  "offset": 0
}
```

---

### Get Odds History

```http
GET /api/v1/odds/history?event_id=abc123&market=spreads&book=fanduel&since=2025-01-15T10:00:00Z&limit=1000
```

**Query Parameters:**
- `event_id` (optional) - Filter by event ID
- `market` (optional) - Filter by market
- `book` (optional) - Filter by book
- `since` (optional) - Start time (RFC3339 format)
- `until` (optional) - End time (RFC3339 format)
- `limit` (optional) - Max results (default: 1000, max: 10000)
- `offset` (optional) - Pagination offset (default: 0)

**Response:**
```json
{
  "history": [
    {
      "id": 12345,
      "event_id": "abc123",
      "sport_key": "basketball_nba",
      "market_key": "spreads",
      "book_key": "fanduel",
      "outcome_name": "Los Angeles Lakers",
      "price": -110,
      "point": -7.5,
      "vendor_last_update": "2025-01-15T19:55:00Z",
      "received_at": "2025-01-15T19:55:01Z",
      "is_latest": false
    }
  ],
  "count": 1,
  "limit": 1000,
  "offset": 0
}
```

---

## Error Responses

All errors follow a consistent format:

```json
{
  "error": "Not Found",
  "message": "event not found",
  "code": 404
}
```

**HTTP Status Codes:**
- `200` - Success
- `400` - Bad Request (invalid parameters)
- `404` - Not Found (resource doesn't exist)
- `500` - Internal Server Error (database/system error)
- `503` - Service Unavailable (database unhealthy)

## Usage

### Setup

```bash
# Install dependencies
make deps

# Copy environment template
cp env.template .env

# Edit .env with your configuration
```

### Run

```bash
# Start API gateway
make run

# Or build and run binary
make build
./bin/api-gateway
```

### Test

```bash
# Run all tests
make test

# Unit tests only
make test-unit

# Integration tests (requires Alexandria DB)
make test-integration

# With coverage
make test-coverage
```

## Configuration

Environment variables:

```bash
# Server
API_GATEWAY_PORT=:8080                    # HTTP listen address

# Database
ALEXANDRIA_DSN=postgres://user:pass@localhost:5435/alexandria?sslmode=disable

# CORS
CORS_ORIGIN=http://localhost:3000        # Frontend origin
```

## Middleware Stack

1. **RequestID** - Unique ID per request
2. **RealIP** - Extract real client IP
3. **Logger** - Request/response logging with latency
4. **Recoverer** - Panic recovery
5. **Timeout** - 30s request timeout
6. **CORS** - Cross-origin resource sharing

## Performance

### SLO Targets
| Metric | Target | Status |
|--------|--------|--------|
| **Latency (p95)** | < 100ms | ✅ Met |
| **Latency (p99)** | < 300ms | ✅ Met |
| **Availability** | 99.5% | ✅ Met |

### Database Connection Pooling
- Max Open Connections: 25
- Max Idle Connections: 5
- Connection Max Lifetime: 5 minutes

### Query Timeouts
- Health check: 2s
- Event queries: 5s
- Odds queries: 5s
- History queries: 10s

## Development

### Project Structure

```
api-gateway/
├── cmd/
│   └── api-gateway/
│       └── main.go              # Entry point
├── internal/
│   ├── db/
│   │   └── alexandria.go        # Database client + interface
│   ├── handlers/
│   │   └── handlers.go          # HTTP handlers
│   └── middleware/
│       └── middleware.go        # Custom middleware
├── pkg/
│   └── models/
│       └── odds.go              # Data models
├── tests/
│   ├── unit/                    # Unit tests
│   └── integration/             # Integration tests
├── Makefile
├── go.mod
├── env.template
└── README.md
```

### Adding New Endpoints

1. **Add Model** (`pkg/models/`)
```go
type NewResource struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
```

2. **Add DB Method** (`internal/db/alexandria.go`)
```go
func (c *Client) GetNewResource(ctx context.Context, id string) (*models.NewResource, error) {
    // Implementation
}
```

3. **Add Handler** (`internal/handlers/handlers.go`)
```go
func (h *Handler) GetNewResource(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

4. **Add Route** (`cmd/api-gateway/main.go`)
```go
r.Get("/api/v1/resources/{id}", handler.GetNewResource)
```

## Testing

### Test Coverage

**Unit Tests** (`tests/unit/handlers/`)
- ✅ 16+ test cases
- ✅ Mock database implementation
- ✅ All handlers tested in isolation
- ✅ Error handling verification
- ✅ Coverage: ~90%

**Integration Tests** (`tests/integration/`)
- ✅ 7+ end-to-end scenarios
- ✅ Real Alexandria database
- ✅ Latency SLO validation
- ✅ Pagination testing
- ✅ Response structure verification

### Running Tests

```bash
# Run all tests
make test

# Unit tests only (fast, no dependencies)
make test-unit

# Integration tests (requires Alexandria DB)
make test-integration

# With coverage report
make test-coverage
```

### Unit Test Example

Test handlers with mocked database:

```go
type MockDB struct {
    events []models.Event
}

func (m *MockDB) GetEvents(ctx, filters) ([]models.Event, error) {
    return m.events, nil
}

func TestGetEvents(t *testing.T) {
    mockDB := &MockDB{events: testData}
    handler := handlers.NewHandler(mockDB)
    
    req := httptest.NewRequest("GET", "/api/v1/events", nil)
    w := httptest.NewRecorder()
    
    handler.GetEvents(w, req)
    
    if w.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", w.Code)
    }
}
```

### Integration Test Example

Test against real Alexandria DB:

```bash
# Start Alexandria DB
docker-compose up alexandria

# Run integration tests
make test-integration
```

See [tests/README.md](./tests/README.md) for comprehensive testing documentation.

## Production Deployment

```bash
# Build Docker image
make docker-build

# Run in Docker
make docker-run

# Or use Docker Compose
docker-compose up api-gateway
```

## Roadmap

- ✅ Events API (v0)
- ✅ Current odds API (v0)
- ✅ Odds history API (v0)
- 🔜 Authentication & authorization (v1)
- 🔜 Rate limiting (v1)
- 🔜 Caching layer (Redis) (v1)
- 🔜 Prometheus metrics export (v1)
- 🔜 OpenTelemetry tracing (v1)
- 🔜 GraphQL API (v2)

## Dependencies

- **go-chi/chi/v5** - Lightweight HTTP router
- **go-chi/cors** - CORS middleware
- **lib/pq** - PostgreSQL driver
- **Go 1.21+**
- **PostgreSQL 14+** (Alexandria DB)

## License

MIT


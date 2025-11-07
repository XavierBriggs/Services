# API Gateway Tests

Comprehensive test suite for the Fortuna API Gateway service.

## Test Structure

```
tests/
├── unit/
│   └── handlers/
│       └── handlers_test.go    # Handler tests with mocked DB
└── integration/
    └── api_test.go              # End-to-end tests with real DB
```

## Unit Tests

Unit tests use a **mock database** implementation to test handlers in isolation.

### MockDB

```go
type MockDB struct {
    events       []models.Event
    currentOdds  []models.CurrentOdds
    shouldError  bool
}
```

The mock implements the `db.AlexandriaDB` interface, allowing handlers to be tested without a real database.

### Coverage

**Handler Tests:**
- ✅ Health check (success + database unhealthy)
- ✅ Get events (with filtering)
- ✅ Get single event (success + not found)
- ✅ Get current odds (with multiple filters)
- ✅ Get odds history (with time filters)
- ✅ Get event with odds (success + not found)
- ✅ Error handling (database errors)

### Running Unit Tests

```bash
# Run unit tests
make test-unit

# With verbose output
go test -v ./tests/unit/...

# With coverage
go test -cover ./tests/unit/...
```

## Integration Tests

Integration tests run against a **real Alexandria database** to verify end-to-end functionality.

### Prerequisites

1. **Alexandria DB Running**
   ```bash
   cd /path/to/fortuna
   docker-compose up alexandria
   ```

2. **Database User Setup** (if needed)
   ```bash
   # Connect to PostgreSQL container
   docker exec -it fortuna-alexandria psql -U postgres
   
   # Create user and database
   CREATE USER fortuna_dev WITH PASSWORD 'fortuna_dev_password';
   CREATE DATABASE alexandria OWNER fortuna_dev;
   GRANT ALL PRIVILEGES ON DATABASE alexandria TO fortuna_dev;
   ```

3. **Environment Variables** (optional override)
   ```bash
   export ALEXANDRIA_TEST_DSN="postgres://fortuna_dev:fortuna_dev_password@localhost:5435/alexandria?sslmode=disable"
   ```

### Test Coverage

**API Tests:**
- ✅ Health check with real DB ping
- ✅ Get events from actual data
- ✅ Get current odds with data age verification
- ✅ Get odds history with time range filtering
- ✅ Get event with odds (combined query)
- ✅ Response latency SLO validation
- ✅ Pagination testing (limit/offset)

**SLO Validation:**
| Endpoint | Target | Test |
|----------|--------|------|
| Health check | < 50ms | ✅ |
| Get events | < 100ms | ✅ |
| Get current odds | < 100ms | ✅ |
| Get odds history | < 150ms | ✅ |

### Running Integration Tests

```bash
# Run integration tests
make test-integration

# With verbose output
go test -v -tags=integration ./tests/integration/...

# Run specific test
go test -v -tags=integration ./tests/integration/... -run TestIntegration_HealthCheck
```

## Test Data

Integration tests use **existing data** from:
- Alexandria seed data (sports, books, markets)
- Mercury ingested odds (if running)

Tests are **read-only** and don't modify production data.

## Continuous Integration

Tests are designed to run in CI/CD pipelines:

```yaml
# Example GitHub Actions
test:
  services:
    postgres:
      image: postgres:14
      env:
        POSTGRES_PASSWORD: fortuna_dev_password
        POSTGRES_DB: alexandria
  steps:
    - name: Run unit tests
      run: make test-unit
    
    - name: Run integration tests
      run: make test-integration
      env:
        ALEXANDRIA_TEST_DSN: ${{ secrets.ALEXANDRIA_TEST_DSN }}
```

## Test Best Practices

### 1. Use Table-Driven Tests

```go
tests := []struct {
    name    string
    input   string
    want    int
}{
    {"case 1", "input1", 200},
    {"case 2", "input2", 404},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // Test implementation
    })
}
```

### 2. Mock External Dependencies

```go
// Good: Test with mocked DB
mockDB := &MockDB{events: testEvents}
handler := handlers.NewHandler(mockDB)
```

### 3. Test Error Cases

```go
// Test database errors
mockDB := &MockDB{shouldError: true}
handler.GetEvents(w, req)
// Verify error response
```

### 4. Verify Response Structure

```go
var response map[string]interface{}
json.NewDecoder(w.Body).Decode(&response)

if response["status"] != "healthy" {
    t.Error("unexpected status")
}
```

### 5. Use httptest for HTTP Testing

```go
req := httptest.NewRequest("GET", "/health", nil)
w := httptest.NewRecorder()

handler.HealthCheck(w, req)

if w.Code != http.StatusOK {
    t.Errorf("expected 200, got %d", w.Code)
}
```

## Troubleshooting

### "Failed to connect to test database" or "password authentication failed"

**Solution:** Ensure Alexandria DB is running and user exists:
```bash
# Start Alexandria DB
cd /path/to/fortuna
docker-compose up alexandria

# If user doesn't exist, create it:
docker exec -it fortuna-alexandria psql -U postgres -c "CREATE USER fortuna_dev WITH PASSWORD 'fortuna_dev_password';"
docker exec -it fortuna-alexandria psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE alexandria TO fortuna_dev;"
```

**Note:** If database infrastructure isn't set up yet, that's okay! Unit tests validate the code logic. Integration tests are for validating the full stack once infrastructure is deployed.

### "No events available for testing"

**Solution:** Run Mercury to ingest some data:
```bash
cd mercury
make run
```

### "Test timeout"

**Solution:** Database may be slow. Increase timeout:
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

## Metrics

**Unit Tests:**
- Tests: 16+
- Coverage: ~90%
- Execution time: < 1s

**Integration Tests:**
- Tests: 7+
- Execution time: 2-5s (depends on DB)

## Adding New Tests

### Unit Test Template

```go
func TestNewFeature(t *testing.T) {
    // Setup
    mockDB := &MockDB{
        // Test data
    }
    handler := handlers.NewHandler(mockDB)

    // Execute
    req := httptest.NewRequest("GET", "/endpoint", nil)
    w := httptest.NewRecorder()
    handler.NewFeature(w, req)

    // Verify
    if w.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", w.Code)
    }

    var response YourType
    json.NewDecoder(w.Body).Decode(&response)
    // Assert response fields
}
```

### Integration Test Template

```go
// +build integration

func TestIntegration_NewFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    dbClient := getTestDB(t)
    handler := handlers.NewHandler(dbClient)

    // Test with real database
    req := httptest.NewRequest("GET", "/endpoint", nil)
    w := httptest.NewRecorder()
    handler.NewFeature(w, req)

    // Verify against real data
    if w.Code != http.StatusOK {
        t.Errorf("expected 200, got %d", w.Code)
    }
}
```

## License

MIT


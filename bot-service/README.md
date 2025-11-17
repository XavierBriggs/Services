# Bot Service

Automated bet placement service that integrates Fortuna opportunities with Talos betting bots.

## Features

- **Manual Bet Placement**: Place bets via API when user clicks "Place with Bot"
- **Team Mapping**: Automatically maps team names using Atlas database short_name lookups
- **Format Transformation**: Converts Fortuna opportunity format to Talos bot format
- **Retry Logic**: Exponential backoff retry for failed bet placements
- **Execution Logging**: Comprehensive logging to `bet_execution_logs` table
- **Health Monitoring**: Checks bot availability before placing bets

## Architecture

```
Fortuna UI → API Gateway → Bot Service → Talos Bot Manager → Book Bots
                ↓
           Holocron DB (bet records)
```

## Team Mapping

The service uses the Atlas `teams` table to map full team names to short names:

1. Fetches event info from Alexandria (home_team, away_team)
2. Looks up `short_name` in Atlas `teams` table by `full_name`
3. Converts to lowercase (e.g., "Lakers" → "lakers")
4. Sends short names to Talos bots

If Atlas DB is not available, falls back to extracting short name from full name.

## API

### POST /api/v1/place-bet

Place bets using automated bots.

**Request:**
```json
{
  "opportunity_id": 42,
  "legs": [
    {
      "book_key": "betus",
      "outcome_name": "LAL -3.5",
      "stake": 96.25,
      "expected_odds": -110
    }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "results": [
    {
      "book_key": "betus",
      "success": true,
      "bet_id": 123,
      "ticket_number": "TKT-12345",
      "latency_ms": 15234,
      "error": ""
    }
  ]
}
```

### GET /health

Service health check.

### GET /api/v1/bot-status

Get status of all registered Talos bots.

## Configuration

Environment variables:

- `BOT_SERVICE_PORT`: Service port (default: 8090)
- `TALOS_BOT_MANAGER_URL`: Talos Bot Manager URL (default: http://localhost:5000)
- `HOLOCRON_DSN`: Holocron database connection string
- `ALEXANDRIA_DSN`: Alexandria database connection string
- `ATLAS_DSN`: Atlas database connection string (optional)
- `RETRY_MAX_ATTEMPTS`: Maximum retry attempts (default: 3)
- `RETRY_INITIAL_DELAY`: Initial retry delay (default: 1s)

## Running

### Local Development
```bash
# Install dependencies
go mod download

# Run service
make run

# Or with custom config
TALOS_BOT_MANAGER_URL=http://localhost:5000 \
HOLOCRON_DSN=postgres://... \
ALEXANDRIA_DSN=postgres://... \
go run ./cmd/bot-service/main.go
```

### Docker
```bash
# Build image
make docker-build

# Run container
docker run -p 8090:8090 \
  -e TALOS_BOT_MANAGER_URL=http://talos-bot-manager:5000 \
  -e HOLOCRON_DSN=postgres://... \
  -e ALEXANDRIA_DSN=postgres://... \
  bot-service:latest
```

## Data Flow

1. **Receive Request**: Bot Service receives bet placement request from API Gateway
2. **Fetch Opportunity**: Loads opportunity and legs from Holocron
3. **Fetch Event Info**: Gets team names from Alexandria events table
4. **Map Teams**: Looks up short names from Atlas teams table
5. **Transform Format**: Converts to Talos format (short names, lowercase)
6. **Check Health**: Verifies bot is available and logged in
7. **Place Bet**: Sends request to Talos Bot Manager with retry logic
8. **Create Record**: Creates bet record in Holocron
9. **Log Execution**: Logs to bet_execution_logs table

## Latency

- **Fortuna Services**: ~100-250ms (database queries, transformation)
- **Talos Routing**: ~20-40ms (health checks, routing)
- **Browser Automation**: 10-60 seconds (Selenium execution - bottleneck)
- **Total**: ~10-60 seconds (typically 15-30 seconds)

## Error Handling

- **Bot Unavailable**: Returns error if bot is not healthy
- **Transformation Errors**: Returns error if team mapping fails
- **Retry Logic**: Automatically retries failed requests with exponential backoff
- **Partial Success**: For multi-leg bets, returns results for each leg individually

## Future Enhancements

- **Automated Betting**: Redis Stream consumer for fully automated placement
- **Parallel Execution**: Execute multiple legs simultaneously
- **Rate Limiting**: Per-book rate limiting to avoid detection
- **Health Caching**: Cache bot health status to reduce checks


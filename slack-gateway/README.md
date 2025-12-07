# Slack Gateway Service

Interactive bet placement service for Slack. Enables users to place bets directly from Slack alerts via buttons and modals.

## Features

- **Interactive Alerts**: Block Kit formatted alerts with "Place Bet" buttons
- **Bet Confirmation Modal**: Modal dialog for stake entry and confirmation
- **User Preferences**: Per-user filtering and default stake settings
- **Deduplication**: Prevents accidental duplicate bets
- **Async Execution**: Non-blocking bet placement with thread replies
- **Structured Logging**: JSON logs for all bet events

## Architecture

```
Alert Service → Slack (Block Kit message with buttons)
                    ↓
User clicks "Place Bet"
                    ↓
Slack Gateway ← /slack/interactions
    ├── Opens confirmation modal
    ├── Validates stake & filters
    ├── Checks dedup (Redis SETNX)
    └── Calls Bet API async
                    ↓
Thread Reply ← Success/failure result
```

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/slack/interactions` | POST | Slack button clicks and modal submissions |
| `/slack/commands` | POST | Slash commands (`/fortuna`) |

## Slack App Setup

### 1. Create Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Click "Create New App" → "From scratch"
3. Name it "Fortuna" and select your workspace

### 2. Configure Bot Token Scopes

Go to **OAuth & Permissions** and add these Bot Token Scopes:

- `chat:write` - Post messages and ephemeral messages
- `commands` - Add slash commands
- `users:read` - Read user info

### 3. Enable Interactivity

Go to **Interactivity & Shortcuts**:

1. Toggle **Interactivity** to On
2. Set **Request URL** to: `https://your-host/slack/interactions`

### 4. Add Slash Command

Go to **Slash Commands** and create a new command:

- **Command**: `/fortuna`
- **Request URL**: `https://your-host/slack/commands`
- **Description**: Fortuna betting controls
- **Usage Hint**: `[filters|status|help]`

### 5. Install App to Workspace

Go to **Install App** and click "Install to Workspace"

### 6. Get Credentials

- **Bot Token**: Copy from OAuth & Permissions → Bot User OAuth Token (`xoxb-...`)
- **Signing Secret**: Copy from Basic Information → Signing Secret

## Configuration

Environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SLACK_SIGNING_SECRET` | Yes | - | Slack app signing secret |
| `SLACK_BOT_TOKEN` | Yes | - | Slack bot OAuth token |
| `PORT` | No | `:8095` | HTTP listen address |
| `REDIS_URL` | No | `localhost:6380` | Redis address |
| `REDIS_PASSWORD` | No | - | Redis password |
| `HOLOCRON_DSN` | No | local | Holocron database connection |
| `ALEXANDRIA_DSN` | No | local | Alexandria database connection |
| `BET_API_URL` | No | `http://localhost:8081` | Bet placement API URL |

## Local Development

### Using ngrok

For local development, use ngrok to expose your local server:

```bash
# Start the service
make run

# In another terminal, start ngrok
ngrok http 8095

# Copy the https URL and update Slack app settings:
# - Interactivity URL: https://xxxx.ngrok.io/slack/interactions
# - Command URL: https://xxxx.ngrok.io/slack/commands
```

### Running with Docker

```bash
# Build image
make docker-build

# Run (set environment variables first)
export SLACK_SIGNING_SECRET=your_secret
export SLACK_BOT_TOKEN=xoxb-your-token
make docker-run
```

### Running with Docker Compose

```bash
cd deploy
docker-compose --profile app up slack-gateway
```

## User Commands

### `/fortuna filters`

Opens a modal to configure:
- **Allowed Books**: Only show alerts and allow bets for selected books
- **Minimum Edge**: Minimum edge % threshold for alerts
- **Default Stake**: Pre-filled stake in bet confirmation
- **Alerts Enabled**: Toggle alerts on/off

### `/fortuna status`

Shows current filter settings.

### `/fortuna help`

Shows help information.

## Bet Flow

1. **Alert arrives** with "Place Bet" button
2. User clicks **Place Bet**
3. **Modal opens** with opportunity details and stake input
4. User confirms **stake** and clicks "Confirm Bet"
5. Gateway **validates** filters and dedup
6. **Acknowledges** Slack immediately
7. **Calls bet API** in background (10-60s)
8. **Posts thread reply** with result

## Deduplication

- Each button click generates a unique `bet_intent_id`
- Modal stores it in `private_metadata`
- On submit, Redis SETNX with 10-minute TTL
- Same modal resubmit → blocked (duplicate)
- New modal → new `bet_intent_id` → allowed

## Logging

Structured JSON logs with events:

| Event | Description |
|-------|-------------|
| `bet_button_clicked` | User clicked Place Bet button |
| `bet_modal_submit_received` | Modal submission received |
| `bet_blocked_by_filter` | Bet blocked by user filters |
| `bet_dedup_blocked` | Duplicate submission blocked |
| `bet_request_accepted` | Bet sent to API |
| `bet_completed` | Bet successfully placed |
| `bet_failed` | Bet execution failed |
| `filters_updated` | User updated filter settings |

## Database

### slack_filter_preferences

```sql
CREATE TABLE slack_filter_preferences (
  slack_user_id VARCHAR(20) PRIMARY KEY,
  books_whitelist TEXT[] DEFAULT '{}',
  min_edge_percent DECIMAL(5,2) DEFAULT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  default_stake_cents INTEGER DEFAULT 10000,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Dependencies

- `github.com/slack-go/slack` - Slack SDK
- `github.com/redis/go-redis/v9` - Redis client
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/go-chi/chi/v5` - HTTP router
- `github.com/google/uuid` - UUID generation








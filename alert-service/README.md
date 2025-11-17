# Alert Service

Sends Slack notifications for detected betting opportunities.

## Overview

Consumes the `opportunities.detected` stream, filters/deduplicates/rate-limits opportunities, and sends formatted alerts to Slack.

## Features

- **Filtering**: Min edge %, max data age
- **Deduplication**: Redis-based (5min TTL default)
- **Rate Limiting**: Token bucket (10/min default)
- **Age Badges**: 🟢 <5s, 🟡 5-10s, 🔴 >10s
- **Latency Tracking**: Full pipeline visibility

## Configuration

Environment variables (see `env.template`):

- `SLACK_WEBHOOK_URL`: Slack incoming webhook URL (required)
- `ALERT_MIN_EDGE_PCT`: Minimum edge for alerts (default: 1.0%)
- `ALERT_MAX_DATA_AGE_SECONDS`: Max staleness (default: 10s)
- `ALERT_RATE_LIMIT`: Max alerts/minute (default: 10)
- `ALERT_DEDUP_TTL_MINUTES`: Dedup cache TTL (default: 5)

## Slack Webhook Setup

1. Go to https://api.slack.com/apps
2. Create new app → Incoming Webhooks
3. Add webhook to your workspace
4. Copy webhook URL to `.env`:

```bash
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/HERE
```

## Alert Format

```
💰 EDGE DETECTED | Edge: 2.5%

Event: lakers_clippers_123
Market: spreads
Age: 🟢 3s

Leg 1: FanDuel | LAL +7.5 @ -105 (7.5) | Edge: 2.5%

Fair Price: -110

View: http://localhost:3000/opportunities

Detected: 15:04:05 | ID: 42
```

## Usage

```bash
# Local  
make run

# Docker
docker-compose up alert-service
```

## Metrics

- Alerts sent
- Alerts filtered  
- Alerts rate-limited
- Average latency (ms)

Reported every 30 seconds.

## Architecture

```
opportunities.detected stream
    ↓
Alert Service
 ├─ Filter (edge%, age)
 ├─ Deduplicator (Redis)
 ├─ Rate Limiter (token bucket)
 └─ Slack Notifier (webhook)
```

## Testing

```bash
make test-unit
make test-integration
```





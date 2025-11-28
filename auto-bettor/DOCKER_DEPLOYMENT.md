# Auto-Bettor Docker Deployment Guide

## Quick Start

The auto-bettor service is now integrated into the main Fortuna docker-compose setup.

### 1. Prerequisites

Ensure you have the `.env` file in the `deploy/` directory with:

```bash
# Required API Keys
ODDS_API_KEY=your_theoddsapi_key

# Redis Password
REDIS_PASSWORD=reddis_pw

# Database Passwords
ALEXANDRIA_PASSWORD=fortuna_dev_password
HOLOCRON_PASSWORD=fortuna_dev_password
ATLAS_PASSWORD=fortuna_dev_password

# Optional: Bot Credentials (if using Talos bots)
BETUS_USERNAME=your_username
BETUS_PASSWORD=your_password
BETONLINE_USERNAME=your_username
BETONLINE_PASSWORD=your_password
BOVADA_USERNAME=your_username
BOVADA_PASSWORD=your_password
CAPTCHA_API_KEY=your_2captcha_key

# Optional: Service Configuration
LOG_LEVEL=info
```

### 2. Start the Stack

```bash
cd deploy/

# Start all services including auto-bettor
docker-compose --profile app up -d

# Or with development tools (pgAdmin, Redis Commander)
docker-compose --profile app --profile dev up -d
```

### 3. Verify Auto-Bettor is Running

```bash
# Check service status
docker-compose ps auto-bettor

# View logs
docker-compose logs -f auto-bettor

# Expected output:
# 🤖 Auto-Bettor Service Starting...
# ✓ Database connections established
# ✓ Components initialized
# ✓ Started consuming from stream: opportunities.detected
# 🚀 Auto-Bettor Service Running
```

### 4. Enable Auto-Betting

The auto-bettor starts in **disabled mode** for safety. Enable it via:

**Option A: Database (Recommended for first setup)**
```bash
docker exec -it fortuna-holocron psql -U fortuna -d holocron
```

```sql
-- Enable auto-betting for default user
UPDATE user_settings
SET 
  auto_betting_enabled = TRUE,
  auto_enabled_opportunity_types = ARRAY['edge']::VARCHAR[],
  auto_enabled_markets = ARRAY['spreads', 'totals', 'h2h']::VARCHAR[],
  auto_enabled_books = ARRAY['betus', 'bovada']::VARCHAR[],
  auto_min_edge_pct = 2.0,
  auto_max_stake_per_bet = 100.00,
  auto_max_exposure_total = 1000.00,
  kelly_fraction = 0.25
WHERE user_id = 'default';

\q
```

**Option B: Frontend Dashboard**
1. Navigate to `http://localhost:3000/auto-betting`
2. Toggle "Auto-Betting Enabled"
3. Configure settings as needed

**Option C: API**
```bash
curl -X PUT http://localhost:8080/api/v1/auto-betting/settings \
  -H "Content-Type: application/json" \
  -d '{"user_id":"default","auto_betting_enabled":true,"auto_min_edge_pct":2.0}'
```

### 5. Monitor Activity

**View Live Logs:**
```bash
docker-compose logs -f auto-bettor

# You should see:
# 📨 Opportunity received: ID=1234 Type=edge Edge=3.50%
# ✓ All filters passed
# 📋 Execution Plan: Stake=$25.00 Bankroll=$500.00
# ✅ Execution success: Placed=1 Failed=0
```

**Check Decisions:**
```bash
docker exec -it fortuna-holocron psql -U fortuna -d holocron -c \
  "SELECT id, opportunity_id, decision, decision_reason, calculated_stake 
   FROM auto_betting_decisions 
   ORDER BY created_at DESC LIMIT 10;"
```

**Check State:**
```bash
docker exec -it fortuna-holocron psql -U fortuna -d holocron -c \
  "SELECT total_exposure, bets_placed_today, todays_pnl, is_paused 
   FROM auto_betting_state 
   WHERE user_id = 'default';"
```

**Frontend Dashboard:**
- Dashboard: `http://localhost:3000/auto-betting`
- Decision History: `http://localhost:3000/auto-betting/decisions`

## Service Dependencies

The auto-bettor requires these services to be healthy:
- ✅ **Redis** - Stream consumption
- ✅ **Holocron DB** - Settings & state storage
- ✅ **Alexandria DB** - Event data lookups
- ✅ **Kelly Calculator** - Position sizing
- ✅ **Bot Service** - Bet execution & balance fetching
- ✅ **Edge Detector** - Opportunity publishing

## Configuration

All configuration is via environment variables (set in docker-compose.yml):

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | `redis:6379` | Redis host |
| `REDIS_PASSWORD` | `reddis_pw` | Redis password |
| `HOLOCRON_DSN` | postgres connection | Holocron database |
| `ALEXANDRIA_DSN` | postgres connection | Alexandria database |
| `KELLY_CALCULATOR_URL` | `http://kelly-calculator:8084` | Kelly service |
| `BOT_SERVICE_URL` | `http://bot-service:8090` | Bot service |
| `CONSUMER_GROUP` | `auto-bettor` | Redis consumer group |
| `STREAM_KEY` | `opportunities.detected` | Redis stream to consume |
| `LOG_LEVEL` | `info` | Logging level |
| `BALANCE_CACHE_TTL` | `30s` | Balance cache duration |

## Testing

### Publish Test Opportunity

```bash
# Enter Redis container
docker exec -it fortuna-redis redis-cli -a reddis_pw

# Publish test opportunity
XADD opportunities.detected * opportunity '{"id":9999,"opportunity_type":"edge","sport_key":"basketball_nba","event_id":"test_event_1","market_key":"spreads","edge_pct":3.5,"detected_at":"2025-11-22T12:00:00Z","data_age_seconds":5,"game_start_time":"2025-11-22T19:00:00Z","legs":[{"id":1,"book_key":"betus","outcome_name":"Lakers +5.5","price":110,"point":5.5,"leg_edge_pct":3.5}]}'

# Exit
exit
```

Watch the auto-bettor logs to see it process the opportunity.

## Troubleshooting

### Auto-Bettor Not Starting
```bash
# Check build logs
docker-compose logs auto-bettor | grep -i error

# Check dependencies
docker-compose ps | grep -E "(redis|holocron|kelly|bot-service)"

# Rebuild if needed
docker-compose build auto-bettor
docker-compose up -d auto-bettor
```

### No Opportunities Being Processed
```bash
# Check if edge-detector is running
docker-compose ps edge-detector

# Check stream has messages
docker exec -it fortuna-redis redis-cli -a reddis_pw XLEN opportunities.detected

# Check consumer group
docker exec -it fortuna-redis redis-cli -a reddis_pw XINFO GROUPS opportunities.detected
```

### Bets Not Being Placed
```bash
# Check bot-service is healthy
curl http://localhost:8090/health

# Check bot status
curl http://localhost:8090/bots/status | jq

# Check auto-betting is enabled
docker exec -it fortuna-holocron psql -U fortuna -d holocron -c \
  "SELECT auto_betting_enabled FROM user_settings WHERE user_id='default';"

# Check for pause state
docker exec -it fortuna-holocron psql -U fortuna -d holocron -c \
  "SELECT is_paused, pause_reason FROM auto_betting_state WHERE user_id='default';"
```

### Reset Auto-Betting State
```bash
docker exec -it fortuna-holocron psql -U fortuna -d holocron -c \
  "UPDATE auto_betting_state 
   SET total_exposure=0, bets_placed_today=0, bets_placed_last_hour=0, 
       is_paused=false, pause_reason=null 
   WHERE user_id='default';"
```

## Scaling

To run multiple auto-bettor instances (for redundancy):

```bash
# Scale to 3 instances
docker-compose --profile app up -d --scale auto-bettor=3

# Each instance gets a unique consumer ID automatically
# Redis consumer groups ensure each opportunity is processed once
```

## Monitoring

### Prometheus Metrics (Future Enhancement)
The service is ready for metrics integration. Add Prometheus endpoint in future version.

### Health Checks
```bash
# Service health via logs
docker-compose logs auto-bettor | grep "Auto-Bettor Service Running"

# Database health
docker-compose ps | grep healthy
```

## Stopping and Cleanup

```bash
# Stop auto-bettor only
docker-compose stop auto-bettor

# Stop all services
docker-compose --profile app down

# Stop and remove volumes (⚠️ DELETES ALL DATA)
docker-compose --profile app down -v
```

## Production Deployment

For production, consider:

1. **Resource Limits** - Add memory/CPU limits in docker-compose
2. **Health Checks** - Enable health check endpoint in auto-bettor
3. **Monitoring** - Add Prometheus/Grafana integration
4. **Alerts** - Configure alert-service for critical failures
5. **Backups** - Regular database backups of Holocron
6. **Secrets** - Use Docker secrets instead of environment variables
7. **Multiple Instances** - Run 2-3 instances for redundancy

## Next Steps

1. ✅ Service is running in Docker
2. ⚙️ Configure settings via frontend or database
3. 📊 Monitor dashboard at `/auto-betting`
4. 🎯 Watch for opportunities to be processed automatically
5. 📈 Track performance in decision history

**Important Safety Reminder:**
- Auto-betting starts **DISABLED** by default
- Test with small stakes initially
- Monitor closely for first few hours
- Set conservative exposure limits
- Use circuit breakers (loss streak, daily loss limits)


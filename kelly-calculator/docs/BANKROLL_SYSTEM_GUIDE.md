# Fortuna Per-Book Bankroll System - Complete Guide

## 🎯 Overview

Your system now supports **per-book bankroll tracking** with intelligent Kelly sizing based on each sportsbook's actual balance. This matches real-world betting where you have separate accounts at each book.

## ✨ What Was Implemented

### 1. Database Layer
- **New Table**: `user_settings` in Holocron database
- **Fields**:
  - `bankrolls` (JSONB): Per-book bankrolls (e.g., `{"fanduel": 5000, "draftkings": 3000}`)
  - `kelly_fraction`: Your Kelly fraction (default: 0.25 = 1/4 Kelly)
  - `min_edge_threshold`: Minimum edge to show opportunities  
  - `max_stake_pct`: Maximum stake cap (safety limit)

### 2. API Endpoints (API Gateway)
- `GET /api/v1/settings` - Retrieve your settings
- `PUT /api/v1/settings` - Update your settings

### 3. Web UI Components
- **Settings Page** (`/settings`):
  - Set bankroll for each sportsbook
  - Configure Kelly fraction (1/10 to Full Kelly)
  - Set edge thresholds and safety caps
  - Real-time total bankroll calculation
  - Best practices guidance

- **Opportunities Page** - Now shows:
  - **Sharp Line**: The fair price (sharp consensus) that edge is compared against
  - Edge percentage
  - All bet details

- **QuickBetModal** - Enhanced:
  - Automatically loads your settings
  - Uses the **specific book's bankroll** for Kelly calculations
  - Shows which book and bankroll amount being used
  - Falls back to defaults if settings unavailable

## 📊 How It Works

### Edge Calculation (Already Optimal!)
Your system follows best practices:

1. **Sharp Consensus**: Averages no-vig probabilities from configured sharp books
   - Sharp books: Pinnacle, Circa, Bookmaker (configured in env)
   - Uses no-vig probability when available, falls back to implied probability

2. **Edge Formula**: 
   ```
   edge = (fairProb / impliedProb) - 1.0
   ```
   This is the **Kelly-optimal** formula for single bets.

3. **Fair Price**: Calculated as American odds from sharp consensus

### Kelly Sizing Per Book
When you click "Bet" on an opportunity at FanDuel:

1. System loads your settings
2. Looks up your **FanDuel bankroll** (e.g., $5,000)
3. Uses your configured Kelly fraction (e.g., 0.25)
4. Calculates: `stake = bankroll × kelly% × kelly_fraction`
5. Applies max stake cap (default: 10% of bankroll)

## 🚀 Getting Started

### Step 1: Clear Old Bets (Optional)
```bash
cd /Users/xavierbriggs/development/fortuna
psql -h localhost -p 5436 -U fortuna -d holocron -f scripts/clear-bets.sql
```

### Step 2: Restart Holocron DB (to apply new migration)
```bash
cd deploy
docker-compose restart holocron
```

Wait ~30 seconds for the database to fully restart and apply the `006_create_user_settings.sql` migration.

### Step 3: Set Your Bankrolls
1. Navigate to http://localhost:3000/settings
2. Enter your actual bankroll for each book:
   - FanDuel: $X,XXX
   - DraftKings: $X,XXX  
   - BetMGM: $X,XXX
   - etc.
3. Configure Kelly settings:
   - **Recommended**: 1/4 Kelly (0.25) - conservative, optimal balance
   - **Moderate**: 1/2 Kelly (0.50) - more aggressive
   - **Very Conservative**: 1/10 Kelly (0.10) - minimal variance
4. Click "Save Settings"

### Step 4: Place a Bet
1. Go to Opportunities page
2. Find an edge opportunity
3. Click "Bet"
4. Modal will show:
   - Book name
   - Your bankroll for that specific book
   - Recommended stake (Kelly-sized)
   - Fair price (sharp consensus line)
5. Confirm and place

### Step 5: Verify in Bets Tab
Your bet will appear in the `/bets` page with:
- Book, stake amount, odds
- CLV (will calculate when event starts)
- P&L tracking

## 📝 Configuration Files

### Database Migration
`services/infra/holocron/migrations/006_create_user_settings.sql`

### API Models  
`services/api-gateway/pkg/models/models.go` - `UserSettings` type

### API Handlers
`services/api-gateway/internal/handlers/settings.go`
`services/api-gateway/internal/db/holocron.go` - DB methods

### Frontend
- Settings page: `web/fortuna_client/app/settings/page.tsx`
- Types: `web/fortuna_client/types/settings.ts`
- API functions: `web/fortuna_client/lib/api-settings.ts`
- Quick Bet Modal: `web/fortuna_client/components/bet/QuickBetModal.tsx`

## 🎓 Best Practices

### Kelly Fraction Recommendations
- **1/4 Kelly (0.25)**: RECOMMENDED - balances growth and variance
- **1/2 Kelly (0.50)**: Moderate - 75% of growth, 50% of variance
- **Full Kelly (1.0)**: NOT RECOMMENDED - maximum growth but extreme variance

### Bankroll Management
- Keep separate bankrolls per book (matches real accounts)
- Rebalance quarterly by withdrawing profits or adding funds
- Never bet more than 10% of a single book's bankroll on one bet
- Track CLV - if consistently negative, you're overestimating edges

### Edge Thresholds
- **Minimum 1-2%**: Accounts for estimation uncertainty
- **Higher for player props**: More uncertainty in these markets
- **Lower for game lines**: Main markets are more efficient

## 🔧 Troubleshooting

### Settings not saving?
- Check API Gateway logs: `docker logs fortuna-api-gateway`
- Verify Holocron DB is running: `docker ps | grep holocron`
- Check migration applied: `psql -h localhost -p 5436 -U fortuna -d holocron -c "\dt user_settings"`

### Zero bankroll warning?
- Go to Settings and set your bankroll for that book
- System will show warning but still calculate with $0

### Kelly sizes seem wrong?
- Verify your edge estimates are accurate (check sharp consensus)
- Check your Kelly fraction setting (0.25 is recommended)
- Ensure max stake cap is reasonable (10% default)

## 📈 Future Enhancements

Possible additions:
- Multi-user support (currently single "default" user)
- Bankroll history tracking
- Auto-rebalancing suggestions
- CLV-based edge calibration
- Risk of ruin calculator

## 🔥 Summary

You now have a **professional-grade bankroll management system** that:
- ✅ Tracks per-book bankrolls (matches real accounts)
- ✅ Uses Kelly Criterion for optimal bet sizing
- ✅ Shows sharp consensus lines (what edge is compared against)
- ✅ Follows betting best practices
- ✅ Integrates seamlessly with your existing system
- ✅ Provides safety caps and warnings

Your edge detection already follows best practices - this completes the betting workflow!










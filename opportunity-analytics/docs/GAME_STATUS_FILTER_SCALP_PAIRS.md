# Game Status Filter for Book Scalp Pairs

## Overview
Extended the game_status filtering to include the Best Scalp Pairs and Best Middle Pairs endpoints, allowing users to see which book combinations are generating live vs pregame opportunities.

## Changes Made

### Backend Updates

#### 1. Updated Handler Functions
**File**: `services/opportunity-analytics/internal/handlers/handlers.go`

Added `game_status` parameter to scalp and middle pairs endpoints:

**GetScalpPairs:**
```go
startTime, endTime := parseTimeRange(r)
gameStatus := r.URL.Query().Get("game_status")  // NEW

pairs, err := h.writer.GetBestBookPairs(ctx, startTime, endTime, "scalp", gameStatus, limit)
```

**GetMiddlePairs:**
```go
startTime, endTime := parseTimeRange(r)
gameStatus := r.URL.Query().Get("game_status")  // NEW

pairs, err := h.writer.GetBestBookPairs(ctx, startTime, endTime, "middle", gameStatus, limit)
```

**GetBestBookPairs:** (General endpoint for both types)
```go
oppType := r.URL.Query().Get("type") 
gameStatus := r.URL.Query().Get("game_status")  // NEW

pairs, err := h.writer.GetBestBookPairs(ctx, startTime, endTime, oppType, gameStatus, limit)
```

#### 2. Updated Writer Function
**File**: `services/opportunity-analytics/internal/writer/holocron.go`

Modified `GetBestBookPairs` to accept and filter by `gameStatus`:

**Function Signature:**
```go
// Before:
func (w *HolocronWriter) GetBestBookPairs(ctx context.Context, startTime, endTime time.Time, oppType string, limit int)

// After:
func (w *HolocronWriter) GetBestBookPairs(ctx context.Context, startTime, endTime time.Time, oppType, gameStatus string, limit int)
```

**SQL Filtering:**
```go
if gameStatus != "" {
    query += fmt.Sprintf(" AND COALESCE(o.game_status, 'upcoming') = $%d", argCount)
    args = append(args, gameStatus)
    argCount++
}
```

### Frontend Updates

#### 3. Updated Data Fetching
**File**: `web/fortuna_client/app/analytics/page.tsx`

Modified the data loading to pass `game_status` to scalp/middle pairs:

```typescript
fetchScalpPairs({ 
  ...hoursOrDays, 
  limit: 10, 
  game_status: gameStatusFilter || undefined  // NEW
}).catch(() => []),

fetchMiddlePairs({ 
  ...hoursOrDays, 
  limit: 10, 
  game_status: gameStatusFilter || undefined  // NEW
}).catch(() => []),
```

**Note**: The filter dropdown was already added to the Pairs tab in the previous update, so no additional UI changes were needed.

## Test Results

### Live Scalp Pairs (last 24h)
```bash
curl "http://localhost:8091/api/v1/analytics/stats/scalp-pairs?hours=24&game_status=live&limit=3"
```

**Results:**
```json
{
  "pair_name": "betmgm + fanduel",
  "total_opportunities": 42,
  "live_opportunities": 42,     ← All live
  "pregame_opportunities": 0
}
{
  "pair_name": "betmgm + gtbets",
  "total_opportunities": 42,
  "live_opportunities": 42,     ← All live
  "pregame_opportunities": 0
}
{
  "pair_name": "fanduel + mybookieag",
  "total_opportunities": 41,
  "live_opportunities": 41,     ← All live
  "pregame_opportunities": 0
}
```

### Pregame Scalp Pairs (last 24h)
```bash
curl "http://localhost:8091/api/v1/analytics/stats/scalp-pairs?hours=24&game_status=upcoming&limit=3"
```

**Results:**
```json
{
  "pair_name": "ballybet + williamhill",
  "total_opportunities": 2,
  "live_opportunities": 0,
  "pregame_opportunities": 2    ← All pregame
}
{
  "pair_name": "draftkings + fanduel",
  "total_opportunities": 2,
  "live_opportunities": 0,
  "pregame_opportunities": 2    ← All pregame
}
{
  "pair_name": "ballybet + fliff",
  "total_opportunities": 1,
  "live_opportunities": 0,
  "pregame_opportunities": 1    ← All pregame
}
```

## Key Findings

### Live vs Pregame Book Pairs

**Live Game Pairs:**
- **Higher volume pairs** (40+ opportunities)
- Books: BetMGM, FanDuel, GTBets, MyBookieAG
- These are major books with fast-moving live lines

**Pregame Pairs:**
- **Lower volume pairs** (1-2 opportunities)
- Books: BallyBet, WilliamHill, Fliff, DraftKings + FanDuel
- Sharp pregame lines with fewer arbitrage opportunities

### Strategic Insights

1. **Live Scalping**:
   - Focus on: BetMGM, FanDuel, GTBets, MyBookieAG combinations
   - High opportunity volume
   - Requires fast execution

2. **Pregame Scalping**:
   - Different book combinations emerge
   - Lower frequency but potentially sharper lines
   - Better execution windows

## User Experience

### Using the Filter

1. **Navigate to Analytics → Pairs Tab**
2. **Use the Game Status filter** at the top of the page
3. **Select filter option**:
   - **All Games** - Shows all book pairs
   - **🔴 Live Only** - Shows only pairs with live opportunities
   - **🔵 Pregame Only** - Shows only pairs with pregame opportunities

4. **View filtered results**:
   - Scalp pairs list updates automatically
   - Middle pairs list updates automatically
   - Counts reflect filtered data

### What Gets Filtered

When you select **🔴 Live Only**:
- Only shows book pairs that have opportunities detected during live games
- The pair's `total_opportunities`, `live_opportunities`, and `pregame_opportunities` counts reflect ONLY live opportunities
- Example: "betmgm × fanduel" with 42 live opportunities appears, but pairs with only pregame won't show

When you select **🔵 Pregame Only**:
- Only shows book pairs with pregame opportunities
- Different pairs may appear than in live view
- Example: "ballybet × williamhill" with 2 pregame opportunities

## API Endpoints Updated

### 1. Best Book Pairs (General)
```
GET /api/v1/analytics/stats/book-pairs?game_status=live
GET /api/v1/analytics/stats/book-pairs?game_status=upcoming
```

### 2. Scalp Pairs (Specific)
```
GET /api/v1/analytics/stats/scalp-pairs?game_status=live
GET /api/v1/analytics/stats/scalp-pairs?game_status=upcoming
```

### 3. Middle Pairs (Specific)
```
GET /api/v1/analytics/stats/middle-pairs?game_status=live
GET /api/v1/analytics/stats/middle-pairs?game_status=upcoming
```

**Parameters:**
- `game_status` (optional): `"live"`, `"upcoming"`, or omit for all
- `hours` or `days`: Time range
- `limit`: Max number of pairs to return

## Use Cases

### 1. Identify Live Scalp Books
**Question**: "Which book combinations create the most live scalp opportunities?"

**Action**: 
- Go to Pairs tab
- Filter: **🔴 Live Only**
- Review top scalp pairs

**Result**: Focus execution speed and automation on these specific book pairs during games.

### 2. Pregame Arbitrage Strategy
**Question**: "Are there consistent pregame scalp opportunities?"

**Action**:
- Go to Pairs tab  
- Filter: **🔵 Pregame Only**
- Review scalp pairs

**Result**: Different books appear (BallyBet, WilliamHill, Fliff) with lower volume but potentially better execution windows.

### 3. Book Speed Analysis
**Question**: "Which books are slow to update live odds?"

**Action**:
- Filter: **🔴 Live Only**
- Check which books appear most frequently in pairs
- High frequency = slower line updates

**Result**: Books like MyBookieAG, GTBets appearing frequently in live pairs suggests they may be slower to update during games.

### 4. Strategy Optimization
**Question**: "Should we focus on live or pregame scalping?"

**Action**:
- Compare pair counts between live and pregame
- Review average edges for each
- Check ROI differences

**Result from data**: Live pairs have much higher volume (42 vs 2 opportunities), suggesting more live scalp opportunities exist.

## Files Modified

### Backend
1. `services/opportunity-analytics/internal/handlers/handlers.go`
   - Added `gameStatus` parameter to 3 handler functions
   
2. `services/opportunity-analytics/internal/writer/holocron.go`
   - Updated `GetBestBookPairs` signature
   - Added SQL filtering by game_status

### Frontend
3. `web/fortuna_client/app/analytics/page.tsx`
   - Updated scalp/middle pairs fetch calls
   - Already had filter UI from previous update

## Testing

### Backend Tests
✅ Live filter: Returns only live pairs (42 opps for top pair)  
✅ Pregame filter: Returns only pregame pairs (2 opps for top pair)  
✅ No filter: Returns all pairs with both live and pregame counts  
✅ No linter errors

### Frontend Tests
✅ Filter dropdown exists in Pairs tab  
✅ Changing filter triggers data reload  
✅ Scalp pairs update based on filter  
✅ Middle pairs update based on filter  
✅ No linter errors

## Performance Notes

- SQL query uses existing `game_status` column index
- Efficient filtering at database level (not client-side)
- No additional joins required
- Same fast performance as other game_status filters

## Summary

✅ **Complete Implementation**: Scalp and middle pairs now respect game_status filter  
✅ **Backend Support**: All three pair endpoints support filtering  
✅ **Frontend Integration**: Filter automatically applied to pair fetching  
✅ **Tested & Working**: Live and pregame pairs return correctly filtered data  
✅ **Insights Enabled**: Can now see which book pairs are active live vs pregame

**Key Discovery**: Live scalp pairs dominate with 40+ opportunities (BetMGM, FanDuel, GTBets), while pregame pairs are rarer with 1-2 opportunities (BallyBet, WilliamHill, Fliff).

**Status**: Complete and deployed  
**Date**: November 29, 2025





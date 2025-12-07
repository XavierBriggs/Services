# Live/Pregame Opportunity Breakdown Fix

## Overview
Fixed the Best Scalp Pairs and Best Middle Pairs displays to correctly show the breakdown between **live** and **pregame** opportunities, providing clear visibility into when opportunities are being detected.

## Problem
The analytics dashboard was showing total opportunity counts for book pairs, but wasn't distinguishing between:
- **Pregame opportunities**: Detected before game start (when lines are typically sharper)
- **Live opportunities**: Detected during active games (often with stale data or different risk profiles)

This made it difficult to assess the quality and timing of detected opportunities.

## Solution

### Backend Changes

#### 1. Updated `BookPairSummary` Model
**File**: `services/opportunity-analytics/pkg/models/stats.go`

Added two new fields to track live vs pregame breakdown:
```go
type BookPairSummary struct {
    // ... existing fields ...
    TotalOpportunities   int     `json:"total_opportunities"`
    LiveOpportunities    int     `json:"live_opportunities"`    // NEW
    PregameOpportunities int     `json:"pregame_opportunities"` // NEW
    // ... rest of fields ...
}
```

#### 2. Enhanced SQL Query in `GetBestBookPairs`
**File**: `services/opportunity-analytics/internal/writer/holocron.go`

Modified the query to:
- Include `game_status` in the CTE
- Use conditional aggregation to count live vs pregame opportunities separately

```sql
SELECT 
    book_key_1,
    book_key_2,
    book_key_1 || ' + ' || book_key_2 as pair_name,
    opportunity_type,
    COUNT(DISTINCT id) as total_opportunities,
    COUNT(DISTINCT CASE WHEN game_status = 'live' THEN id END) as live_opportunities,
    COUNT(DISTINCT CASE WHEN game_status = 'upcoming' THEN id END) as pregame_opportunities,
    -- ... rest of aggregations ...
FROM book_pairs
GROUP BY book_key_1, book_key_2, opportunity_type
```

**Key Change**: Removed the blanket `excludeLive` filter that was hiding all live opportunities. Now all opportunities are counted and labeled correctly.

### Frontend Changes

#### 3. Updated TypeScript Interface
**File**: `web/fortuna_client/lib/analytics-api.ts`

```typescript
export interface BookPairSummary {
  // ... existing fields ...
  total_opportunities: number;
  live_opportunities: number;      // NEW
  pregame_opportunities: number;   // NEW
  // ... rest of fields ...
}
```

#### 4. Enhanced UI Display
**File**: `web/fortuna_client/app/analytics/page.tsx`

Updated both Scalp Pairs and Middle Pairs displays to show:
- **Red badge**: Live opportunities (e.g., "5 live")
- **Blue badge**: Pregame opportunities (e.g., "16 pregame")
- **Green/Purple badge**: Total count (e.g., "21 total")

Visual hierarchy now shows:
```
#1  fanduel × mybookieag    [5 live] [16 pregame] [21 total]
```

## Color Coding

- 🔴 **Red badge** (`bg-red-500/10 text-red-500`): Live opportunities
- 🔵 **Blue badge** (`bg-blue-500/10 text-blue-500`): Pregame opportunities  
- 🟢 **Green badge** (scalps) or 🟣 **Purple badge** (middles): Total count

## Example Output

### Before (Unclear timing):
```
#1 fanduel × mybookieag          21 opps
```

### After (Clear breakdown):
```
#1 fanduel × mybookieag    [5 live] [16 pregame] [21 total]
   Avg Edge: 2.77%    Best Edge: 8.69%    ROI: +0.0%    Profit: $0
```

## Database Schema

The fix leverages existing infrastructure:
- `opportunities.game_status` column (added in migration `013_add_game_status_to_opportunities.sql`)
- `analytics_book_pairs.game_status` column (added in migration `015_create_analytics_book_pairs.sql`)

Values:
- `'upcoming'` or `'pregame'`: Opportunity detected before game start
- `'live'`: Opportunity detected during game

## Verification Steps

1. **Restart the service**:
   ```bash
   cd deploy
   docker compose up -d opportunity-analytics
   ```

2. **Check the API response**:
   ```bash
   curl "http://localhost:8091/api/v1/analytics/stats/scalp-pairs?hours=24&limit=10" | jq '.scalp_pairs[0]'
   ```
   
   Should now include:
   ```json
   {
     "pair_name": "fanduel + mybookieag",
     "total_opportunities": 21,
     "live_opportunities": 5,
     "pregame_opportunities": 16,
     ...
   }
   ```

3. **View in UI**:
   - Navigate to Analytics → Pairs tab
   - Verify badges show live/pregame breakdown
   - Hover over badges to see tooltips

## Benefits

1. **Better Decision Making**: Know which pairs are generating opportunities live vs pregame
2. **Quality Assessment**: Pregame opportunities often have sharper lines and better hold times
3. **Risk Management**: Live opportunities may require different stake sizing or filters
4. **Trend Analysis**: Track if certain book pairs are better live or pregame

## Related Files

- Backend Model: `services/opportunity-analytics/pkg/models/stats.go`
- Backend Query: `services/opportunity-analytics/internal/writer/holocron.go`
- Frontend Types: `web/fortuna_client/lib/analytics-api.ts`
- Frontend UI: `web/fortuna_client/app/analytics/page.tsx`

## Next Steps (Optional Enhancements)

1. Add filter dropdown to show "Live Only", "Pregame Only", or "All"
2. Sort options by live count or pregame count
3. Time series chart showing live vs pregame ratio over time
4. Alerts when live opportunity percentage exceeds threshold (may indicate stale data issues)

## Testing

✅ No linter errors  
✅ Docker build successful  
✅ TypeScript types aligned with Go models  
✅ UI properly displays conditional badges (only show if count > 0)

---

**Status**: Ready for deployment and testing  
**Date**: 2025-11-29  
**Impact**: Analytics display clarity improvement - no breaking changes


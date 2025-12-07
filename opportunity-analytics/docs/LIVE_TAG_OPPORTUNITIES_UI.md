# Live/Pregame Tags Added to Opportunities UI

## Overview
Added live/pregame status badges to the main Opportunities page, allowing users to see at a glance which opportunities are detected during live games vs before game start.

## Changes Made

### 1. Updated TypeScript Types
**File**: `web/fortuna_client/types/opportunity.ts`

Added `event_status` field to the Opportunity interface:
```typescript
export interface Opportunity {
  id: number;
  opportunity_type: OpportunityType;
  sport_key: string;
  event_id: string;
  event_status?: string; // "upcoming" (pregame) or "live" ← NEW
  market_key: string;
  edge_pct: number;
  // ... rest of fields
}
```

### 2. Enhanced Opportunities Page
**File**: `web/fortuna_client/app/opportunities/page.tsx`

#### A. Added Game Status Badge Function
```typescript
const getGameStatusBadge = (eventStatus?: string) => {
  if (eventStatus === 'live') {
    return {
      label: 'LIVE',
      color: 'bg-red-500/10 text-red-500 border-red-500/20',
      emoji: '🔴'
    };
  }
  return {
    label: 'PREGAME',
    color: 'bg-blue-500/10 text-blue-500 border-blue-500/20',
    emoji: '🔵'
  };
};
```

#### B. Updated Table Display
Modified the Type column to show both opportunity type and game status:

**Before:**
```
Type Column:
┌──────────┐
│ 🎯 SCALP │
└──────────┘
```

**After:**
```
Type Column:
┌──────────┐
│ 🎯 SCALP │
├──────────┤
│ 🔴 LIVE  │
└──────────┘
```

#### C. Added Game Status Filter
Added a new filter dropdown to filter opportunities by game status:
- **All Games** - Show both live and pregame
- **🔴 Live Only** - Only show opportunities detected during live games
- **🔵 Pregame Only** - Only show pregame opportunities

Filter implementation:
```typescript
const filtered = data.filter(opp => {
  if (opp.edge_pct < minEdge) return false;
  if (gameStatusFilter !== 'all') {
    if (gameStatusFilter === 'live' && opp.event_status !== 'live') return false;
    if (gameStatusFilter === 'pregame' && opp.event_status === 'live') return false;
  }
  return true;
});
```

## Visual Design

### Badge Colors
- 🔴 **Live Badge**: Red with `bg-red-500/10 text-red-500 border-red-500/20`
- 🔵 **Pregame Badge**: Blue with `bg-blue-500/10 text-blue-500 border-blue-500/20`

### Layout
The badges are stacked vertically in the Type column:
1. Top: Opportunity Type badge (Edge/Middle/Scalp)
2. Bottom: Game Status badge (Live/Pregame)

### Responsive
- Filters grid updated from 4 columns to 5 columns
- Mobile: Stacks to single column automatically
- Badges are compact and fit well in table cells

## Example UI

### Opportunities Table Row
```
┌──────────────┬─────────────────────┬──────────┬────────┐
│ Type         │ Event               │ Market   │ Edge % │
├──────────────┼─────────────────────┼──────────┼────────┤
│ ⚡ SCALP     │ Knicks vs Bucks     │ spreads  │ 5.07%  │
│ 🔴 LIVE      │ basketball_nba      │          │        │
├──────────────┼─────────────────────┼──────────┼────────┤
│ 🎯 MIDDLE    │ Heat vs Celtics     │ totals   │ 2.34%  │
│ 🔵 PREGAME   │ basketball_nba      │          │        │
└──────────────┴─────────────────────┴──────────┴────────┘
```

### Filters Section
```
┌──────────┬──────────┬──────────────┬──────────┬─────────┐
│ Type     │ Sport    │ Game Status  │ Min Edge │ Results │
├──────────┼──────────┼──────────────┼──────────┼─────────┤
│ All Types│ NBA      │ 🔴 Live Only │ 1.0      │ 21 found│
└──────────┴──────────┴──────────────┴──────────┴─────────┘
```

## User Benefits

### 1. **Instant Visual Feedback**
- Red badge immediately shows which opportunities are from live games
- Blue badge shows pregame opportunities (typically sharper lines)

### 2. **Risk Assessment**
- Live opportunities may have:
  - Stale data (slower updates during games)
  - Wider spreads
  - Faster line movement
  - Shorter execution windows
- Pregame opportunities typically have:
  - More stable lines
  - Better data freshness
  - Longer execution windows

### 3. **Filtering Capability**
- Quickly focus on live-only opportunities during game time
- Filter to pregame-only to avoid live game risks
- Toggle between both as needed

### 4. **Strategy Optimization**
Users can now:
- Apply different stake sizing for live vs pregame
- Use different edge thresholds for each type
- Track which game status produces better results
- Avoid live opportunities if concerned about execution speed

## Technical Details

### Data Source
The `event_status` field comes from the backend API:
- Set by the edge detector based on `commence_time` vs `detected_at`
- Values: `"live"` or `"upcoming"` (pregame)
- Stored in `opportunities.game_status` column in Holocron database

### Backend API
The opportunities API (`/api/v1/opportunities`) already returns `event_status` in the response:
```json
{
  "id": 12345,
  "opportunity_type": "scalp",
  "event_status": "live",
  "edge_pct": 5.07,
  ...
}
```

### Fallback Behavior
If `event_status` is missing or undefined:
- Badge defaults to "PREGAME" (safer assumption)
- No errors thrown
- UI gracefully handles missing field

## Testing Steps

1. **Navigate to Opportunities page** (`/opportunities`)
2. **Verify badges display**:
   - Each opportunity should show a game status badge
   - Live opportunities: 🔴 LIVE (red)
   - Pregame opportunities: 🔵 PREGAME (blue)

3. **Test filters**:
   - Select "🔴 Live Only" → Only live opportunities shown
   - Select "🔵 Pregame Only" → Only pregame opportunities shown
   - Select "All Games" → Both types shown

4. **Verify styling**:
   - Badges should be compact and aligned
   - Colors should match design (red for live, blue for pregame)
   - Text should be readable

## Related Changes

This builds on the earlier analytics page update:
- **Analytics Page** (`/analytics`): Shows live/pregame breakdown in Best Scalp Pairs
- **Opportunities Page** (`/opportunities`): Shows live/pregame badge on each opportunity

Both pages now consistently display game status information.

## Files Modified

1. `web/fortuna_client/types/opportunity.ts` - Added `event_status` field
2. `web/fortuna_client/app/opportunities/page.tsx` - Added badges and filter

## Next Steps (Optional Enhancements)

1. **Stats Breakdown**: Show count of live vs pregame in footer stats
2. **Color-coded Rows**: Subtle background tint for live opportunities
3. **Auto-filter**: Remember user's last selected game status filter
4. **Notifications**: Alert when new live opportunities detected
5. **Edge Comparison**: Show avg edge for live vs pregame in stats

---

**Status**: ✅ Complete - Ready for testing  
**Date**: 2025-11-29  
**Impact**: User experience improvement - clearer opportunity classification  
**Breaking Changes**: None (backward compatible)


# 🚀 Analytics Implementation Roadmap

## Current Status: Phase 1 (Basic) ✅

### What's Working Now:
- ✅ Basic opportunity aggregation (count, avg edge)
- ✅ Game status tracking (live vs pregame)  
- ✅ Bet results enrichment (via writer query)
- ✅ Simple API endpoints
- ✅ Basic frontend dashboard

### What's Missing:
- ❌ Advanced metrics (hold time, execution rate, edge distribution)
- ❌ Additional analytics tables (event stats, book comparison, etc.)
- ❌ Bet enricher service (post-settlement updates)
- ❌ Advanced API endpoints
- ❌ Rich visualizations

---

## Phase 2: Hold Time & Execution Tracking (NEXT - 2-3 days)

**Goal:** Track how long opportunities last and conversion rates

### Tasks:

#### 2.1. Update Opportunity Model
```go
// edge-detector/pkg/models/opportunity.go
type Opportunity struct {
    // ... existing fields ...
    DetectedAt time.Time
    ExpiresAt  *time.Time  // NEW: When opportunity closed
}
```

#### 2.2. Track Opportunity Lifecycle
```go
// edge-detector: When opportunity disappears, publish update
{
  "opportunity_id": 12345,
  "status": "expired",
  "expired_at": "2025-11-27T19:02:45Z",
  "hold_time_seconds": 45
}
```

#### 2.3. Update Aggregator
```go
// opportunity-analytics/internal/aggregator/aggregator.go
type BucketStats struct {
    // ... existing ...
    TotalHoldTime int       // Sum for averaging
    HoldTimeCount int       // Count of opps with hold time
    ConversionCount int     // How many converted to bets
}
```

#### 2.4. Add Columns
```sql
ALTER TABLE analytics_book_stats 
ADD COLUMN avg_hold_time_seconds INTEGER,
ADD COLUMN execution_rate DECIMAL(5,2);
```

#### 2.5. Test & Deploy
- Test locally
- Deploy to staging
- Monitor for 24 hours
- Deploy to production

**Estimated Time:** 2-3 days
**Priority:** HIGH ⭐
**Impact:** Enables execution speed optimization

---

## Phase 3: Edge Distribution Tracking (NEXT - 1-2 days)

**Goal:** Track min/max/median/stddev of edges

### Tasks:

#### 3.1. Update Aggregator
```go
type BucketStats struct {
    // ... existing ...
    EdgeList []float64  // Store all edges for calculation
}

func (a *Aggregator) ConvertToBookStats(buckets) {
    // Calculate distribution
    sort.Float64s(stats.EdgeList)
    median := stats.EdgeList[len(stats.EdgeList)/2]
    stddev := calculateStdDev(stats.EdgeList)
}
```

#### 3.2. Add Columns
```sql
ALTER TABLE analytics_book_stats 
ADD COLUMN min_edge_pct DECIMAL(6,3),
ADD COLUMN max_edge_pct DECIMAL(6,3),
ADD COLUMN median_edge_pct DECIMAL(6,3),
ADD COLUMN edge_stddev DECIMAL(6,3);
```

#### 3.3. API Endpoint
```go
GET /api/v1/analytics/edge-distribution
// Returns histogram data for charting
```

**Estimated Time:** 1-2 days
**Priority:** MEDIUM
**Impact:** Enables edge quality analysis

---

## Phase 4: Bet Enricher Service (NEXT - 3-4 days)

**Goal:** Update analytics with settlement results

### Tasks:

#### 4.1. Build Service
- ✅ Code already written (`cmd/enricher/main.go`)
- Need to: Build, test, dockerize

#### 4.2. Add to Docker Compose
```yaml
bet-enricher:
  build: ./services/opportunity-analytics
  dockerfile: Dockerfile.enricher
  environment:
    HOLOCRON_DSN: "..."
    ENRICHMENT_INTERVAL: "15m"
```

#### 4.3. Database Migration
```bash
# Add game_status to opportunities table
docker exec -i fortuna-holocron psql ... < 013_add_game_status_to_opportunities.sql
```

#### 4.4. Test Backfill
```bash
# Test with 7 days of historical data
./bet-enricher backfill --days 7
```

#### 4.5. Deploy & Monitor
- Deploy enricher service
- Verify analytics updates every 15min
- Check win/loss/profit data accuracy

**Estimated Time:** 3-4 days
**Priority:** HIGH ⭐⭐
**Impact:** Completes the bet settlement loop!

---

## Phase 5: Additional Analytics Tables (FUTURE - 5-7 days)

**Goal:** Create specialized analytics tables

### Tasks:

#### 5.1. Event-Level Analytics
```sql
CREATE TABLE analytics_event_stats ...
```
- Track per-game opportunity metrics
- Line movement tracking
- Opportunity windows

#### 5.2. Book Comparison
```sql
CREATE TABLE analytics_book_comparison ...
```
- Reliability scoring
- Limit tracking
- Market coverage

#### 5.3. Hourly Patterns
```sql
CREATE TABLE analytics_hourly_patterns ...
```
- Time-of-day analysis
- Day-of-week patterns
- Seasonal trends

**Estimated Time:** 5-7 days
**Priority:** MEDIUM
**Impact:** Enables deep market intelligence

---

## Phase 6: Advanced API & Frontend (FUTURE - 7-10 days)

**Goal:** Rich visualizations and insights

### Tasks:

#### 6.1. New API Endpoints
- `/analytics/execution-speed` - Hold times
- `/analytics/book-comparison` - Reliability
- `/analytics/optimal-windows` - Best times
- `/analytics/market-efficiency` - Market analysis

#### 6.2. Frontend Components
- Edge distribution histogram
- Hold time comparison chart
- Book reliability scorecard
- Time-of-day heatmap
- Execution funnel visualization

#### 6.3. Real-Time Alerts
- High-value window detected
- Conversion rate dropping
- New book showing opportunities
- Unusual pattern detected

**Estimated Time:** 7-10 days
**Priority:** LOW
**Impact:** Best user experience & insights

---

## Timeline Summary

```
WEEK 1-2: Core Advanced Metrics
├─ Day 1-3:  Hold time & execution tracking  ⭐⭐
├─ Day 4-5:  Edge distribution               ⭐
└─ Day 6-10: Bet enricher service            ⭐⭐

WEEK 3-4: Specialized Analytics
├─ Day 11-15: Additional tables              
└─ Day 16-20: Advanced APIs

WEEK 5+: Polish & Optimization
├─ Day 21-25: Rich visualizations
├─ Day 26-30: Real-time alerts
└─ Ongoing:   ML models & predictions
```

---

## Quick Start: Just Get Hold Time Working

If you want to start small, here's the **minimal viable implementation**:

### Step 1: Add Columns (5 min)
```sql
ALTER TABLE analytics_book_stats 
ADD COLUMN avg_hold_time_seconds INTEGER;
```

### Step 2: Update Aggregator (30 min)
```go
// Just track average hold time from data_age_seconds
// (Not perfect but gives you something quickly)
func (a *Aggregator) ProcessOpportunity(opp) {
    // Use data_age as proxy for hold time initially
    stats.TotalHoldTime += opp.DataAgeSeconds
    stats.HoldTimeCount++
}

func ConvertToBookStats() {
    avgHoldTime := stats.TotalHoldTime / stats.HoldTimeCount
    // Write to DB
}
```

### Step 3: Query It (immediate)
```sql
SELECT 
  opportunity_type,
  AVG(avg_hold_time_seconds) as avg_window
FROM analytics_book_stats
GROUP BY opportunity_type;
```

**Result:** Basic hold time tracking in 35 minutes! 🚀

---

## Recommendations

### For Maximum Impact:
1. **Do Phase 2 first** (Hold time) - Biggest operational value
2. **Then Phase 4** (Bet enricher) - Completes the data flow
3. **Then Phase 3** (Edge distribution) - Nice insights
4. **Then Phase 5-6** - Polish and advanced features

### For Quick Win:
Just implement the "Quick Start" above to get hold time tracking today!

---

## Questions to Answer:

1. **How fast do I need this?** 
   - Immediately → Quick Start (35 min)
   - This week → Phase 2 only (2-3 days)
   - Complete system → Full roadmap (4-6 weeks)

2. **What's most valuable?**
   - Execution optimization → Phase 2 (hold time)
   - Bet results tracking → Phase 4 (enricher)
   - Market analysis → Phase 5 (tables)
   - User experience → Phase 6 (frontend)

3. **Can I help build this?**
   - Yes! Let me know which phase to implement first.


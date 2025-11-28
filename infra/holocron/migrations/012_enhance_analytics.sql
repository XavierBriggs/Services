-- Migration: Enhance analytics with rich insights
-- Description: Add comprehensive tracking for deep market analysis
-- Author: Fortuna System
-- Date: 2025-11-27

-- ============================================================================
-- 1. ADD RICH DIMENSIONS TO EXISTING TABLE
-- ============================================================================

ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS sport_key VARCHAR(50);
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS market_key VARCHAR(50);

-- Edge distribution metrics
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS min_edge_pct DECIMAL(6,3);
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS max_edge_pct DECIMAL(6,3);
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS median_edge_pct DECIMAL(6,3);
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS edge_stddev DECIMAL(6,3);

-- Data quality metrics
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS avg_data_age_seconds INTEGER;
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS stale_data_count INTEGER DEFAULT 0;

-- Velocity metrics (opportunities per time unit)
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS opps_per_minute DECIMAL(10,2);

-- Value metrics
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS total_expected_value DECIMAL(12,2);
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS avg_kelly_fraction DECIMAL(6,4);

-- Execution metrics
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS avg_hold_time_seconds INTEGER;
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS missed_opportunities INTEGER DEFAULT 0;
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS execution_rate DECIMAL(5,2); -- % of opps converted to bets

-- Edge threshold hit rates
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS edge_5pct_count INTEGER DEFAULT 0;
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS edge_10pct_count INTEGER DEFAULT 0;
ALTER TABLE analytics_book_stats ADD COLUMN IF NOT EXISTS edge_20pct_count INTEGER DEFAULT 0;

-- Add indexes for new dimensions
CREATE INDEX IF NOT EXISTS idx_analytics_sport ON analytics_book_stats(sport_key, timestamp_bucket DESC);
CREATE INDEX IF NOT EXISTS idx_analytics_market ON analytics_book_stats(market_key, timestamp_bucket DESC);
CREATE INDEX IF NOT EXISTS idx_analytics_sport_market ON analytics_book_stats(sport_key, market_key, timestamp_bucket DESC);

COMMENT ON COLUMN analytics_book_stats.sport_key IS 'Sport identifier (basketball_nba, americanfootball_nfl, etc)';
COMMENT ON COLUMN analytics_book_stats.market_key IS 'Market type (h2h, spreads, totals, props)';
COMMENT ON COLUMN analytics_book_stats.min_edge_pct IS 'Minimum edge seen in bucket';
COMMENT ON COLUMN analytics_book_stats.max_edge_pct IS 'Maximum edge seen in bucket';
COMMENT ON COLUMN analytics_book_stats.median_edge_pct IS 'Median edge (50th percentile)';
COMMENT ON COLUMN analytics_book_stats.edge_stddev IS 'Standard deviation of edges (volatility)';
COMMENT ON COLUMN analytics_book_stats.avg_data_age_seconds IS 'Average age of odds data when opportunity detected';
COMMENT ON COLUMN analytics_book_stats.stale_data_count IS 'Count of opportunities with stale data (>30s)';
COMMENT ON COLUMN analytics_book_stats.opps_per_minute IS 'Opportunity velocity (opportunities per minute)';
COMMENT ON COLUMN analytics_book_stats.total_expected_value IS 'Sum of all expected values (edge * stake)';
COMMENT ON COLUMN analytics_book_stats.avg_kelly_fraction IS 'Average Kelly Criterion fraction for optimal sizing';
COMMENT ON COLUMN analytics_book_stats.avg_hold_time_seconds IS 'Average time opportunity was valid before odds moved';
COMMENT ON COLUMN analytics_book_stats.missed_opportunities IS 'Opportunities detected but not acted on';
COMMENT ON COLUMN analytics_book_stats.execution_rate IS 'Percentage of opportunities converted to actual bets';
COMMENT ON COLUMN analytics_book_stats.edge_5pct_count IS 'Count of opportunities with edge >= 5%';
COMMENT ON COLUMN analytics_book_stats.edge_10pct_count IS 'Count of opportunities with edge >= 10%';
COMMENT ON COLUMN analytics_book_stats.edge_20pct_count IS 'Count of opportunities with edge >= 20%';

-- ============================================================================
-- 2. CREATE EVENT-LEVEL ANALYTICS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS analytics_event_stats (
  event_id VARCHAR(100) NOT NULL,
  sport_key VARCHAR(50) NOT NULL,
  commence_time TIMESTAMPTZ NOT NULL,
  game_status VARCHAR(20) NOT NULL CHECK (game_status IN ('upcoming', 'live', 'completed')),
  
  -- Opportunity metrics
  total_opportunities INTEGER DEFAULT 0,
  scalp_count INTEGER DEFAULT 0,
  middle_count INTEGER DEFAULT 0,
  edge_count INTEGER DEFAULT 0,
  
  -- Edge metrics
  max_edge_pct DECIMAL(6,3),
  avg_edge_pct DECIMAL(6,3),
  
  -- Market coverage
  markets_with_opportunities TEXT[], -- ['h2h', 'spreads', 'totals']
  books_with_opportunities TEXT[],   -- ['draftkings', 'fanduel', ...]
  
  -- Line movement
  max_line_move_pct DECIMAL(6,3),
  line_moves_count INTEGER DEFAULT 0,
  
  -- Timestamps
  first_opportunity_at TIMESTAMPTZ,
  last_opportunity_at TIMESTAMPTZ,
  opportunity_window_minutes INTEGER,
  
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  PRIMARY KEY (event_id)
);

CREATE INDEX idx_event_stats_sport_time ON analytics_event_stats(sport_key, commence_time DESC);
CREATE INDEX idx_event_stats_commence ON analytics_event_stats(commence_time DESC);
CREATE INDEX idx_event_stats_max_edge ON analytics_event_stats(max_edge_pct DESC);

COMMENT ON TABLE analytics_event_stats IS 'Aggregated opportunity statistics per sporting event';
COMMENT ON COLUMN analytics_event_stats.opportunity_window_minutes IS 'Duration between first and last opportunity detected';

-- ============================================================================
-- 3. CREATE BOOK COMPARISON TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS analytics_book_comparison (
  timestamp_bucket TIMESTAMPTZ NOT NULL,
  book_key VARCHAR(50) NOT NULL,
  sport_key VARCHAR(50) NOT NULL,
  
  -- Sharp vs Soft classification
  book_type VARCHAR(20) CHECK (book_type IN ('sharp', 'soft', 'unknown')),
  
  -- Market efficiency metrics
  avg_margin_pct DECIMAL(6,3),              -- How much vig/juice they charge
  market_coverage_score DECIMAL(5,2),       -- % of markets they offer
  odds_competitiveness_score DECIMAL(5,2),  -- How often they have best price
  
  -- Opportunity metrics
  times_in_scalp INTEGER DEFAULT 0,         -- How often they're part of scalps
  times_in_middle INTEGER DEFAULT 0,
  times_in_edge INTEGER DEFAULT 0,
  times_best_price INTEGER DEFAULT 0,       -- How often they offer best odds
  
  -- Reliability metrics
  avg_line_stability_minutes DECIMAL(8,2),  -- How long their lines stay stable
  odds_changes_per_hour DECIMAL(6,2),
  stale_odds_rate DECIMAL(5,2),             -- % of time odds are stale
  
  -- Limit tracking
  suspected_limit_hits INTEGER DEFAULT 0,    -- Bet rejections or small max bets
  avg_accepted_stake DECIMAL(12,2),
  
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  PRIMARY KEY (timestamp_bucket, book_key, sport_key)
);

CREATE INDEX idx_book_comparison_time ON analytics_book_comparison(timestamp_bucket DESC);
CREATE INDEX idx_book_comparison_book ON analytics_book_comparison(book_key, timestamp_bucket DESC);
CREATE INDEX idx_book_comparison_competitiveness ON analytics_book_comparison(odds_competitiveness_score DESC);

COMMENT ON TABLE analytics_book_comparison IS 'Comparative analysis of sportsbook characteristics and reliability';

-- ============================================================================
-- 4. CREATE TIME-OF-DAY PATTERNS TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS analytics_hourly_patterns (
  hour_of_day INTEGER NOT NULL CHECK (hour_of_day >= 0 AND hour_of_day < 24),
  day_of_week INTEGER NOT NULL CHECK (day_of_week >= 0 AND day_of_week < 7), -- 0=Sunday
  sport_key VARCHAR(50) NOT NULL,
  opportunity_type VARCHAR(20) NOT NULL,
  
  -- Aggregated metrics
  total_samples INTEGER DEFAULT 0,
  avg_opportunities_per_hour DECIMAL(10,2),
  avg_edge_pct DECIMAL(6,3),
  avg_execution_rate DECIMAL(5,2),
  
  -- Best performing hours
  best_roi DECIMAL(6,3),
  best_win_rate DECIMAL(5,2),
  
  last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  PRIMARY KEY (hour_of_day, day_of_week, sport_key, opportunity_type)
);

CREATE INDEX idx_hourly_patterns_sport ON analytics_hourly_patterns(sport_key, hour_of_day);

COMMENT ON TABLE analytics_hourly_patterns IS 'Historical patterns of opportunity quality by time of day and day of week';
COMMENT ON COLUMN analytics_hourly_patterns.day_of_week IS '0=Sunday, 1=Monday, ..., 6=Saturday';

-- ============================================================================
-- 5. CREATE MARKET EFFICIENCY TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS analytics_market_efficiency (
  timestamp_bucket TIMESTAMPTZ NOT NULL,
  sport_key VARCHAR(50) NOT NULL,
  market_key VARCHAR(50) NOT NULL,
  
  -- Efficiency metrics
  total_markets_scraped INTEGER DEFAULT 0,
  markets_with_opportunities INTEGER DEFAULT 0,
  opportunity_hit_rate DECIMAL(5,2),        -- % of markets that yield opportunities
  
  -- Edge characteristics
  avg_edge_when_found DECIMAL(6,3),
  max_edge_found DECIMAL(6,3),
  
  -- Market depth
  avg_books_per_market DECIMAL(5,2),
  avg_price_variance DECIMAL(6,3),          -- How much prices vary across books
  
  -- Arbitrage potential
  scalp_rate DECIMAL(5,2),                  -- % of markets with scalp opportunities
  avg_scalp_margin DECIMAL(6,3),
  
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  PRIMARY KEY (timestamp_bucket, sport_key, market_key)
);

CREATE INDEX idx_market_efficiency_time ON analytics_market_efficiency(timestamp_bucket DESC);
CREATE INDEX idx_market_efficiency_sport_market ON analytics_market_efficiency(sport_key, market_key, timestamp_bucket DESC);

COMMENT ON TABLE analytics_market_efficiency IS 'Market-level efficiency analysis and arbitrage potential';

-- ============================================================================
-- 6. CREATE PERFORMANCE ATTRIBUTION TABLE
-- ============================================================================

CREATE TABLE IF NOT EXISTS analytics_performance_attribution (
  timestamp_bucket TIMESTAMPTZ NOT NULL,
  attribution_type VARCHAR(50) NOT NULL, -- 'sport', 'market', 'book', 'hour', 'opportunity_type'
  attribution_key VARCHAR(100) NOT NULL,  -- Actual value (e.g., 'basketball_nba', 'draftkings', '14' for 2pm)
  
  -- P&L attribution
  opportunities_count INTEGER DEFAULT 0,
  bets_placed INTEGER DEFAULT 0,
  net_profit DECIMAL(12,2) DEFAULT 0,
  roi DECIMAL(6,3),
  
  -- Risk metrics
  max_drawdown DECIMAL(12,2),
  sharpe_ratio DECIMAL(6,3),
  win_rate DECIMAL(5,2),
  
  -- Contribution analysis
  pct_of_total_profit DECIMAL(5,2),
  pct_of_total_opportunities DECIMAL(5,2),
  
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  PRIMARY KEY (timestamp_bucket, attribution_type, attribution_key)
);

CREATE INDEX idx_performance_attr_time ON analytics_performance_attribution(timestamp_bucket DESC);
CREATE INDEX idx_performance_attr_type ON analytics_performance_attribution(attribution_type, timestamp_bucket DESC);
CREATE INDEX idx_performance_attr_roi ON analytics_performance_attribution(roi DESC);

COMMENT ON TABLE analytics_performance_attribution IS 'Performance attribution to understand what drives profits';
COMMENT ON COLUMN analytics_performance_attribution.sharpe_ratio IS 'Risk-adjusted return metric';


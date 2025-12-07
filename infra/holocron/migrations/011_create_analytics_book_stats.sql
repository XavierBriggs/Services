-- Migration: Create analytics_book_stats table
-- Description: Aggregated statistics for opportunity analysis by book, type, and time bucket
-- Author: Fortuna System
-- Date: 2025-11-27

CREATE TABLE IF NOT EXISTS analytics_book_stats (
  timestamp_bucket TIMESTAMPTZ NOT NULL,
  book_key VARCHAR(50) NOT NULL,
  opportunity_type VARCHAR(20) NOT NULL CHECK (opportunity_type IN ('edge', 'middle', 'scalp')),
  opportunity_count INTEGER DEFAULT 0,
  avg_edge_pct DECIMAL(6,3),
  total_bets INTEGER DEFAULT 0,
  wins INTEGER DEFAULT 0,
  losses INTEGER DEFAULT 0,
  avg_clv DECIMAL(10,2),
  net_profit DECIMAL(12,2) DEFAULT 0,
  roi DECIMAL(6,3),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (timestamp_bucket, book_key, opportunity_type),
  CONSTRAINT valid_counts CHECK (
    opportunity_count >= 0 AND 
    total_bets >= 0 AND 
    wins >= 0 AND 
    losses >= 0
  ),
  CONSTRAINT valid_bet_totals CHECK (wins + losses <= total_bets)
);

-- Index for time-based queries (most recent first)
CREATE INDEX idx_analytics_book_stats_time ON analytics_book_stats(timestamp_bucket DESC);

-- Index for book analysis
CREATE INDEX idx_analytics_book_stats_book ON analytics_book_stats(book_key, timestamp_bucket DESC);

-- Index for opportunity type filtering
CREATE INDEX idx_analytics_book_stats_type ON analytics_book_stats(opportunity_type, timestamp_bucket DESC);

-- Composite index for common queries
CREATE INDEX idx_analytics_book_stats_book_type ON analytics_book_stats(book_key, opportunity_type, timestamp_bucket DESC);

-- Comments for documentation
COMMENT ON TABLE analytics_book_stats IS 'Aggregated opportunity and betting statistics bucketed by time, book, and opportunity type';
COMMENT ON COLUMN analytics_book_stats.timestamp_bucket IS 'Time bucket start (e.g., 5-minute or hourly intervals)';
COMMENT ON COLUMN analytics_book_stats.book_key IS 'Sportsbook identifier';
COMMENT ON COLUMN analytics_book_stats.opportunity_type IS 'Type of opportunity: edge, middle, or scalp';
COMMENT ON COLUMN analytics_book_stats.opportunity_count IS 'Number of opportunities detected in this bucket';
COMMENT ON COLUMN analytics_book_stats.avg_edge_pct IS 'Average edge percentage across opportunities';
COMMENT ON COLUMN analytics_book_stats.total_bets IS 'Total number of bets placed';
COMMENT ON COLUMN analytics_book_stats.wins IS 'Number of winning bets';
COMMENT ON COLUMN analytics_book_stats.losses IS 'Number of losing bets';
COMMENT ON COLUMN analytics_book_stats.avg_clv IS 'Average closing line value in cents per dollar';
COMMENT ON COLUMN analytics_book_stats.net_profit IS 'Net profit/loss in dollars';
COMMENT ON COLUMN analytics_book_stats.roi IS 'Return on investment as percentage';



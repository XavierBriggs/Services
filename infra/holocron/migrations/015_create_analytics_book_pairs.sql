-- Migration: Create analytics_book_pairs table for scalp/middle pair tracking
-- Description: Track which book combinations produce the most arbitrage opportunities
-- Author: Fortuna System
-- Date: 2025-11-27

-- ============================================================================
-- Book Pair Analytics Table - Tracks scalp/middle opportunities between book pairs
-- ============================================================================

CREATE TABLE IF NOT EXISTS analytics_book_pairs (
  timestamp_bucket TIMESTAMPTZ NOT NULL,
  book_key_1 VARCHAR(50) NOT NULL,      -- First book (alphabetically sorted)
  book_key_2 VARCHAR(50) NOT NULL,      -- Second book (alphabetically sorted)
  opportunity_type VARCHAR(20) NOT NULL CHECK (opportunity_type IN ('scalp', 'middle')),
  game_status VARCHAR(20) NOT NULL DEFAULT 'upcoming',
  sport_key VARCHAR(50),
  market_key VARCHAR(50),
  
  -- Opportunity counts
  opportunity_count INTEGER DEFAULT 0,
  
  -- Edge metrics (the combined edge of the pair)
  avg_edge_pct DECIMAL(6,3),
  min_edge_pct DECIMAL(6,3),
  max_edge_pct DECIMAL(6,3),
  
  -- Which book had which side (for analysis)
  book1_favorite_count INTEGER DEFAULT 0,  -- How often book1 had the favorite side
  book1_underdog_count INTEGER DEFAULT 0,  -- How often book1 had the underdog side
  
  -- Execution metrics
  total_bets INTEGER DEFAULT 0,            -- How many times we bet both sides
  partial_bets INTEGER DEFAULT 0,          -- How many times we only got one side
  wins INTEGER DEFAULT 0,
  losses INTEGER DEFAULT 0,
  net_profit DECIMAL(12,2) DEFAULT 0,
  
  -- Hold time
  avg_hold_time_seconds INTEGER,
  
  -- Timestamps
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  -- Primary key ensures books are always in consistent order
  PRIMARY KEY (timestamp_bucket, book_key_1, book_key_2, opportunity_type, game_status),
  
  -- Constraint to ensure book_key_1 < book_key_2 alphabetically
  CONSTRAINT book_order CHECK (book_key_1 < book_key_2)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_book_pairs_time ON analytics_book_pairs(timestamp_bucket DESC);
CREATE INDEX IF NOT EXISTS idx_book_pairs_books ON analytics_book_pairs(book_key_1, book_key_2, timestamp_bucket DESC);
CREATE INDEX IF NOT EXISTS idx_book_pairs_type ON analytics_book_pairs(opportunity_type, timestamp_bucket DESC);
CREATE INDEX IF NOT EXISTS idx_book_pairs_profit ON analytics_book_pairs(net_profit DESC);

-- Comments
COMMENT ON TABLE analytics_book_pairs IS 'Tracks scalp/middle opportunities between book pairs';
COMMENT ON COLUMN analytics_book_pairs.book_key_1 IS 'First book (alphabetically sorted for consistency)';
COMMENT ON COLUMN analytics_book_pairs.book_key_2 IS 'Second book (alphabetically sorted for consistency)';
COMMENT ON COLUMN analytics_book_pairs.partial_bets IS 'Bets where only one leg was successfully placed';

-- ============================================================================
-- View for easy querying of best book pairs
-- ============================================================================

CREATE OR REPLACE VIEW v_best_scalp_pairs AS
SELECT 
  book_key_1,
  book_key_2,
  book_key_1 || ' + ' || book_key_2 as pair_name,
  SUM(opportunity_count) as total_opportunities,
  AVG(avg_edge_pct) as avg_edge,
  MAX(max_edge_pct) as best_edge,
  SUM(total_bets) as total_bets,
  SUM(net_profit) as total_profit,
  CASE 
    WHEN SUM(total_bets) > 0 
    THEN ROUND((SUM(net_profit) / SUM(total_bets) * 100)::numeric, 2)
    ELSE 0
  END as roi_pct,
  AVG(avg_hold_time_seconds) as avg_hold_time
FROM analytics_book_pairs
WHERE opportunity_type = 'scalp'
  AND timestamp_bucket > NOW() - INTERVAL '7 days'
GROUP BY book_key_1, book_key_2
ORDER BY SUM(net_profit) DESC;

COMMENT ON VIEW v_best_scalp_pairs IS 'Best performing book pairs for scalps over last 7 days';


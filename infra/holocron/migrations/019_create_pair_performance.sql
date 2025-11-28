-- Migration 019: Create pair performance tracking table
-- Tracks handle, profit, and ROI per book pair for scalps/middles

CREATE TABLE IF NOT EXISTS analytics_pair_performance (
    book_key_1 VARCHAR(50) NOT NULL,
    book_key_2 VARCHAR(50) NOT NULL,
    opportunity_type VARCHAR(20) NOT NULL CHECK (opportunity_type IN ('scalp', 'middle')),
    
    -- Opportunity metrics
    total_opportunities INT DEFAULT 0,
    
    -- Bet metrics
    total_bets_placed INT DEFAULT 0,       -- Number of pair bets (both legs placed)
    partial_bets INT DEFAULT 0,            -- Number where only 1 leg placed
    
    -- Financial metrics
    total_handle DECIMAL(12,2) DEFAULT 0,  -- Sum of all stakes
    realized_profit DECIMAL(12,2) DEFAULT 0,
    roi_pct DECIMAL(8,4) DEFAULT 0,        -- (profit / handle) * 100
    
    -- Outcome tracking
    win_count INT DEFAULT 0,               -- Both legs profitable
    loss_count INT DEFAULT 0,              -- Net loss
    push_count INT DEFAULT 0,              -- Break even
    pending_count INT DEFAULT 0,           -- Not yet settled
    
    -- Performance indicators
    avg_edge_at_detection DECIMAL(6,3),    -- Average edge when detected
    avg_hold_time_seconds INT,             -- Average opportunity duration
    execution_rate DECIMAL(5,2),           -- % of opportunities bet on
    
    -- Timestamps
    first_bet_at TIMESTAMPTZ,
    last_bet_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    PRIMARY KEY (book_key_1, book_key_2, opportunity_type),
    CONSTRAINT unique_pair_order CHECK (book_key_1 < book_key_2)
);

-- Index for quick lookups
CREATE INDEX IF NOT EXISTS idx_pair_perf_roi ON analytics_pair_performance (roi_pct DESC);
CREATE INDEX IF NOT EXISTS idx_pair_perf_profit ON analytics_pair_performance (realized_profit DESC);
CREATE INDEX IF NOT EXISTS idx_pair_perf_handle ON analytics_pair_performance (total_handle DESC);
CREATE INDEX IF NOT EXISTS idx_pair_perf_type ON analytics_pair_performance (opportunity_type);

-- Comments
COMMENT ON TABLE analytics_pair_performance IS 'Aggregated performance metrics for book pairs (scalps/middles)';
COMMENT ON COLUMN analytics_pair_performance.total_handle IS 'Total amount wagered on this pair combination';
COMMENT ON COLUMN analytics_pair_performance.realized_profit IS 'Net profit from settled bets on this pair';
COMMENT ON COLUMN analytics_pair_performance.roi_pct IS 'Return on investment: (profit/handle)*100';
COMMENT ON COLUMN analytics_pair_performance.execution_rate IS 'Percentage of detected opportunities that were bet on';


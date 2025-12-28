-- Migration: Create opportunity_performance table
-- Description: Stores CLV analysis for detected opportunities (validates edge detector accuracy)
-- Author: Fortuna System
-- Date: 2025-11-27

CREATE TABLE IF NOT EXISTS opportunity_performance (
  opportunity_leg_id BIGINT PRIMARY KEY REFERENCES opportunity_legs(id) ON DELETE CASCADE,
  detected_price INT NOT NULL,           -- Price when opportunity was detected
  closing_price INT NOT NULL,            -- Price at game start (from closing_lines)
  clv_cents DECIMAL(10,2),               -- CLV calculation: edge gained/lost vs closing
  edge_at_detection DECIMAL(6,3),        -- Original detected edge percentage
  edge_at_close DECIMAL(6,3),            -- Edge calculated using closing price
  recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT valid_detected_price CHECK (detected_price != 0),
  CONSTRAINT valid_closing_price CHECK (closing_price != 0)
);

-- Index for time-based queries
CREATE INDEX idx_opportunity_performance_recorded ON opportunity_performance(recorded_at DESC);

-- Index for CLV analysis
CREATE INDEX idx_opportunity_performance_clv ON opportunity_performance(clv_cents DESC NULLS LAST);

-- Index for edge accuracy analysis
CREATE INDEX idx_opportunity_performance_edge_detection ON opportunity_performance(edge_at_detection);

-- Comments for documentation
COMMENT ON TABLE opportunity_performance IS 'Tracks CLV for detected opportunities to validate edge detector accuracy. Unlike bet_performance which tracks placed bets, this tracks ALL opportunities.';
COMMENT ON COLUMN opportunity_performance.opportunity_leg_id IS 'Foreign key to the opportunity leg being analyzed';
COMMENT ON COLUMN opportunity_performance.detected_price IS 'American odds when the opportunity was detected';
COMMENT ON COLUMN opportunity_performance.closing_price IS 'American odds at game start (from closing_lines table)';
COMMENT ON COLUMN opportunity_performance.clv_cents IS 'Closing Line Value in cents per dollar. Positive = opportunity price was better than closing';
COMMENT ON COLUMN opportunity_performance.edge_at_detection IS 'Edge percentage at time of detection (from opportunity)';
COMMENT ON COLUMN opportunity_performance.edge_at_close IS 'Edge percentage calculated using closing line price';

-- CLV Calculation Formula (same as bet CLV):
-- clv_cents = (1/decimal(detected_price) - 1/decimal(closing_price)) * 100
--
-- Example: Detected at -110 (1.909), closed at -105 (1.952)
-- clv_cents = (1/1.909 - 1/1.952) * 100 = (0.524 - 0.512) * 100 = 1.2 cents
-- Positive CLV indicates the detected price was better than closing (edge was real)





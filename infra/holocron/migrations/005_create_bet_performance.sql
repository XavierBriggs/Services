-- Migration: Create bet_performance table
-- Description: Advanced bet analytics including CLV (Closing Line Value)
-- Author: Fortuna System
-- Date: 2025-11-07

CREATE TABLE IF NOT EXISTS bet_performance (
  bet_id BIGINT PRIMARY KEY REFERENCES bets(id) ON DELETE CASCADE,
  closing_line_price INT,  -- American odds at game start
  clv_cents DECIMAL(10,2),  -- closing line value in cents per dollar
  hold_time_seconds INT,  -- time from bet placement to game start
  recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT valid_closing_price CHECK (closing_line_price IS NULL OR closing_line_price != 0),
  CONSTRAINT valid_hold_time CHECK (hold_time_seconds IS NULL OR hold_time_seconds >= 0)
);

-- Index for CLV analysis
CREATE INDEX idx_bet_performance_clv ON bet_performance(clv_cents DESC NULLS LAST);

-- Index for hold time analysis
CREATE INDEX idx_bet_performance_hold_time ON bet_performance(hold_time_seconds);

-- Comments for documentation
COMMENT ON TABLE bet_performance IS 'Advanced bet analytics. CLV (Closing Line Value) is a key indicator of betting skill.';
COMMENT ON COLUMN bet_performance.closing_line_price IS 'Odds when the game started (from closing_lines table)';
COMMENT ON COLUMN bet_performance.clv_cents IS 'Value gained/lost vs closing line in cents per dollar wagered. Positive = sharp bet.';
COMMENT ON COLUMN bet_performance.hold_time_seconds IS 'Time between bet placement and game start. Shorter = less line movement risk.';

-- CLV Calculation Formula (for documentation):
-- clv_cents = (1/decimal(bet_price) - 1/decimal(closing_price)) * 100
-- 
-- Example: Bet at -110 (1.909), closes at -115 (1.870)
-- clv_cents = (1/1.909 - 1/1.870) * 100 = (0.524 - 0.535) * 100 = -1.1 cents
-- Positive CLV indicates the bet was placed at better odds than closing (sharp bet)


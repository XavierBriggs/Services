-- Migration: Create bets table
-- Description: Tracks actual bets placed (can be linked to opportunities or manual entries)
-- Author: Fortuna System
-- Date: 2025-11-07

CREATE TABLE IF NOT EXISTS bets (
  id BIGSERIAL PRIMARY KEY,
  opportunity_id BIGINT REFERENCES opportunities(id),  -- NULL if manual entry
  sport_key VARCHAR(50) NOT NULL,
  event_id VARCHAR(100) NOT NULL,
  market_key VARCHAR(50) NOT NULL,
  book_key VARCHAR(50) NOT NULL,
  outcome_name VARCHAR(200) NOT NULL,
  bet_type VARCHAR(20) NOT NULL CHECK (bet_type IN ('straight', 'parlay', 'middle', 'scalp')),
  stake_amount DECIMAL(10,2) NOT NULL,
  bet_price INT NOT NULL,  -- American odds at bet time
  point DECIMAL(10,2),  -- spread/total line
  placed_at TIMESTAMPTZ NOT NULL,
  settled_at TIMESTAMPTZ,
  result VARCHAR(20) CHECK (result IN ('pending', 'win', 'loss', 'push', 'void')),
  payout_amount DECIMAL(10,2),
  CONSTRAINT positive_stake CHECK (stake_amount > 0),
  CONSTRAINT valid_bet_price CHECK (bet_price != 0),
  CONSTRAINT settled_requires_result CHECK (
    (settled_at IS NULL AND result = 'pending') OR 
    (settled_at IS NOT NULL AND result IN ('win', 'loss', 'push', 'void'))
  ),
  CONSTRAINT payout_for_settled CHECK (
    (result = 'pending' AND payout_amount IS NULL) OR
    (result IN ('win', 'loss', 'push', 'void'))
  )
);

-- Index for time-based queries (most recent bets first)
CREATE INDEX idx_bets_placed ON bets(placed_at DESC);

-- Index for event-based queries
CREATE INDEX idx_bets_event ON bets(event_id);

-- Index for book performance analysis
CREATE INDEX idx_bets_book ON bets(book_key, placed_at DESC);

-- Index for pending bets
CREATE INDEX idx_bets_pending ON bets(result, placed_at DESC) WHERE result = 'pending';

-- Index for opportunity tracking
CREATE INDEX idx_bets_opportunity ON bets(opportunity_id) WHERE opportunity_id IS NOT NULL;

-- Index for sport analysis
CREATE INDEX idx_bets_sport ON bets(sport_key, placed_at DESC);

-- Comments for documentation
COMMENT ON TABLE bets IS 'Actual bets placed by operators. Can be linked to opportunities or entered manually.';
COMMENT ON COLUMN bets.opportunity_id IS 'Optional link to detected opportunity. NULL for manual bet entry.';
COMMENT ON COLUMN bets.bet_type IS 'straight: single bet, parlay: multiple legs, middle: hedged position, scalp: arbitrage';
COMMENT ON COLUMN bets.stake_amount IS 'Amount wagered in dollars';
COMMENT ON COLUMN bets.bet_price IS 'Odds at time bet was placed (American format)';
COMMENT ON COLUMN bets.result IS 'Bet outcome: pending, win, loss, push (tie), void (cancelled)';
COMMENT ON COLUMN bets.payout_amount IS 'Total payout received (includes original stake for wins)';


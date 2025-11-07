-- Migration: Create opportunity_legs table
-- Description: Stores individual betting legs for each opportunity (1 for edges, 2+ for middles/scalps)
-- Author: Fortuna System
-- Date: 2025-11-07

CREATE TABLE IF NOT EXISTS opportunity_legs (
  id BIGSERIAL PRIMARY KEY,
  opportunity_id BIGINT NOT NULL REFERENCES opportunities(id) ON DELETE CASCADE,
  book_key VARCHAR(50) NOT NULL,
  outcome_name VARCHAR(200) NOT NULL,
  price INT NOT NULL,  -- American odds
  point DECIMAL(10,2),  -- spread/total line (NULL for h2h)
  leg_edge_pct DECIMAL(6,3),  -- edge for this specific leg
  CONSTRAINT valid_american_odds CHECK (price != 0)
);

-- Index for finding all legs of an opportunity
CREATE INDEX idx_opportunity_legs_opportunity ON opportunity_legs(opportunity_id);

-- Index for book-specific queries
CREATE INDEX idx_opportunity_legs_book ON opportunity_legs(book_key);

-- Comments for documentation
COMMENT ON TABLE opportunity_legs IS 'Individual betting legs that comprise an opportunity. Edges have 1 leg, middles/scalps have 2+';
COMMENT ON COLUMN opportunity_legs.opportunity_id IS 'Foreign key to parent opportunity';
COMMENT ON COLUMN opportunity_legs.book_key IS 'Sportsbook where this bet is available';
COMMENT ON COLUMN opportunity_legs.outcome_name IS 'Bet outcome (e.g., "LAL +7.5", "Over 223.5", "LeBron James Over 24.5 Pts")';
COMMENT ON COLUMN opportunity_legs.price IS 'Offered odds in American format';
COMMENT ON COLUMN opportunity_legs.point IS 'Point value for spreads/totals (NULL for moneylines)';
COMMENT ON COLUMN opportunity_legs.leg_edge_pct IS 'Edge percentage for this individual leg';


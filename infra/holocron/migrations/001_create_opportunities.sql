-- Migration: Create opportunities table
-- Description: Core table for storing detected betting opportunities (edges, middles, scalps)
-- Author: Fortuna System
-- Date: 2025-11-07

CREATE TABLE IF NOT EXISTS opportunities (
  id BIGSERIAL PRIMARY KEY,
  opportunity_type VARCHAR(20) NOT NULL CHECK (opportunity_type IN ('edge', 'middle', 'scalp')),
  sport_key VARCHAR(50) NOT NULL,
  event_id VARCHAR(100) NOT NULL,
  market_key VARCHAR(50) NOT NULL,
  edge_pct DECIMAL(6,3) NOT NULL,
  fair_price INT,  -- American odds
  detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  data_age_seconds INT NOT NULL,  -- staleness at detection
  CONSTRAINT positive_edge CHECK (edge_pct > 0),
  CONSTRAINT valid_data_age CHECK (data_age_seconds >= 0)
);

-- Index for time-based queries (most recent opportunities first)
CREATE INDEX idx_opportunities_detected ON opportunities(detected_at DESC);

-- Index for event-based queries (all opportunities for a specific event)
CREATE INDEX idx_opportunities_event ON opportunities(event_id, detected_at DESC);

-- Index for filtering by opportunity type
CREATE INDEX idx_opportunities_type ON opportunities(opportunity_type, detected_at DESC);

-- Index for filtering by sport
CREATE INDEX idx_opportunities_sport ON opportunities(sport_key, detected_at DESC);

-- Composite index for common queries (type + sport + recent)
CREATE INDEX idx_opportunities_type_sport_detected ON opportunities(opportunity_type, sport_key, detected_at DESC);

-- Comments for documentation
COMMENT ON TABLE opportunities IS 'Stores detected betting opportunities including edges, middles, and scalps';
COMMENT ON COLUMN opportunities.opportunity_type IS 'Type of opportunity: edge (single +EV bet), middle (both sides +EV), scalp (guaranteed profit)';
COMMENT ON COLUMN opportunities.edge_pct IS 'Percentage edge of the opportunity (always positive)';
COMMENT ON COLUMN opportunities.fair_price IS 'Fair price in American odds format (calculated from sharp consensus)';
COMMENT ON COLUMN opportunities.data_age_seconds IS 'Time in seconds between odds update and opportunity detection';


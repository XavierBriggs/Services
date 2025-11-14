-- Migration: Add bot automation fields to bets table
-- Description: Adds columns to track automated bot execution details for each bet
-- Author: Fortuna System
-- Date: 2025-11-12

-- Add bot execution fields
ALTER TABLE bets
  ADD COLUMN IF NOT EXISTS execution_method VARCHAR(20) CHECK (execution_method IN ('manual', 'automated')),
  ADD COLUMN IF NOT EXISTS trigger_source VARCHAR(20) CHECK (trigger_source IN ('manual', 'automated')),
  ADD COLUMN IF NOT EXISTS bot_latency_ms INT CHECK (bot_latency_ms >= 0 OR bot_latency_ms IS NULL),
  ADD COLUMN IF NOT EXISTS ticket_number VARCHAR(100),
  ADD COLUMN IF NOT EXISTS odds_at_placement INT;

-- Index for bot performance analysis
CREATE INDEX IF NOT EXISTS idx_bets_execution_method ON bets(execution_method, placed_at DESC) 
  WHERE execution_method IS NOT NULL;

-- Index for latency analysis
CREATE INDEX IF NOT EXISTS idx_bets_bot_latency ON bets(bot_latency_ms) 
  WHERE bot_latency_ms IS NOT NULL;

-- Comments for new columns
COMMENT ON COLUMN bets.execution_method IS 'How bet was placed: manual (human) or automated (bot)';
COMMENT ON COLUMN bets.trigger_source IS 'What triggered the bet: manual (UI button click) or automated (stream consumer)';
COMMENT ON COLUMN bets.bot_latency_ms IS 'Time taken by bot to place bet in milliseconds (NULL for manual bets)';
COMMENT ON COLUMN bets.ticket_number IS 'Confirmation/ticket number from sportsbook (if available)';
COMMENT ON COLUMN bets.odds_at_placement IS 'Actual odds at moment of placement (may differ from bet_price if odds moved)';



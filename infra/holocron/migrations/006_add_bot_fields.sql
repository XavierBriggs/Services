-- Migration: Add bot execution fields to bets table
-- Description: Adds columns to track automated bet execution
-- Author: Fortuna Bot System
-- Date: 2025-01-15

-- Add execution tracking columns to existing bets table
ALTER TABLE bets 
  ADD COLUMN IF NOT EXISTS execution_method VARCHAR(20) DEFAULT 'manual' 
    CHECK (execution_method IN ('manual', 'automated')),
  ADD COLUMN IF NOT EXISTS trigger_source VARCHAR(20) DEFAULT 'manual'
    CHECK (trigger_source IN ('manual', 'automated')),
  ADD COLUMN IF NOT EXISTS bot_latency_ms INT,
  ADD COLUMN IF NOT EXISTS ticket_number VARCHAR(100),
  ADD COLUMN IF NOT EXISTS odds_at_placement INT;

-- Create indexes for bot-related queries
CREATE INDEX IF NOT EXISTS idx_bets_execution_method ON bets(execution_method, placed_at DESC);
CREATE INDEX IF NOT EXISTS idx_bets_trigger_source ON bets(trigger_source, placed_at DESC);

-- Add comments for documentation
COMMENT ON COLUMN bets.execution_method IS 'How bet was placed: manual (logged in DB only) or automated (bot executed)';
COMMENT ON COLUMN bets.trigger_source IS 'What triggered execution: manual (button) or automated (stream consumer)';
COMMENT ON COLUMN bets.bot_latency_ms IS 'Bot execution latency in milliseconds (automated bets only)';
COMMENT ON COLUMN bets.ticket_number IS 'Sportsbook confirmation ticket number (automated bets only)';
COMMENT ON COLUMN bets.odds_at_placement IS 'Actual odds at the moment of bet placement';




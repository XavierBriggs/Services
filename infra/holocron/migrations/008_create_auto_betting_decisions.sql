-- Migration: Create auto_betting_decisions table
-- Description: Tracks every automated betting decision (placed or skipped) with full context
-- Author: Fortuna System
-- Date: 2025-11-22

CREATE TABLE IF NOT EXISTS auto_betting_decisions (
  id BIGSERIAL PRIMARY KEY,
  user_id VARCHAR(50) NOT NULL DEFAULT 'default',
  opportunity_id BIGINT REFERENCES opportunities(id),

  -- Decision outcome
  decision VARCHAR(50) NOT NULL CHECK (decision IN ('placed', 'skipped_filter', 'skipped_risk', 'skipped_disabled', 'error', 'pending_confirmation')),
  decision_reason TEXT,

  -- Filter results (JSON for detailed tracking)
  filter_results JSONB NOT NULL DEFAULT '{}'::JSONB,

  -- Bet details (if placed)
  calculated_stake DECIMAL(10,2),
  calculated_edge DECIMAL(8,4),
  kelly_fraction_used DECIMAL(4,3),
  full_kelly_pct DECIMAL(8,4),

  -- Execution details
  bet_ids BIGINT[], -- Array of bet IDs (can be multiple for middle/scalp)
  execution_time_ms INTEGER,

  -- Context at time of decision
  current_exposure DECIMAL(10,2),
  current_bankroll DECIMAL(12,2),
  bets_placed_today INTEGER,

  -- Metadata
  created_at TIMESTAMPTZ DEFAULT NOW(),

  -- Constraints
  CONSTRAINT valid_stake CHECK (calculated_stake IS NULL OR calculated_stake > 0),
  CONSTRAINT valid_execution_time CHECK (execution_time_ms IS NULL OR execution_time_ms >= 0)
);

-- Indexes for fast queries
CREATE INDEX idx_auto_decisions_user_created ON auto_betting_decisions(user_id, created_at DESC);
CREATE INDEX idx_auto_decisions_opportunity ON auto_betting_decisions(opportunity_id);
CREATE INDEX idx_auto_decisions_decision ON auto_betting_decisions(decision, created_at DESC);
CREATE INDEX idx_auto_decisions_user_decision ON auto_betting_decisions(user_id, decision, created_at DESC);

-- Comments for documentation
COMMENT ON TABLE auto_betting_decisions IS 'Audit trail of all automated betting decisions including placed bets and skipped opportunities';
COMMENT ON COLUMN auto_betting_decisions.decision IS 'Outcome: placed (bet executed), skipped_filter (filtered out), skipped_risk (risk limits), skipped_disabled (auto-betting off), error (execution failed)';
COMMENT ON COLUMN auto_betting_decisions.filter_results IS 'JSON object containing results from each filter (user_preferences, risk_management, book_health, timing, correlation)';
COMMENT ON COLUMN auto_betting_decisions.bet_ids IS 'Array of bet IDs from bets table (multiple for middle/scalp opportunities)';
COMMENT ON COLUMN auto_betting_decisions.current_bankroll IS 'Live bot balance at time of decision (fetched from bot service)';



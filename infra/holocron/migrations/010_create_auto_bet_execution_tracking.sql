-- Migration: Create execution tracking tables for multi-leg opportunities
-- Description: Tracks MIDDLE and SCALP bet execution across multiple legs with detailed state
-- Author: Fortuna System
-- Date: 2025-11-22

-- Main execution tracking table
CREATE TABLE IF NOT EXISTS auto_bet_execution_tracking (
  id BIGSERIAL PRIMARY KEY,
  auto_decision_id BIGINT REFERENCES auto_betting_decisions(id),
  opportunity_id BIGINT REFERENCES opportunities(id),
  opportunity_type VARCHAR(20) CHECK (opportunity_type IN ('edge', 'middle', 'scalp')),

  -- Execution plan
  total_legs INTEGER NOT NULL CHECK (total_legs > 0),
  legs_required_for_completion INTEGER NOT NULL CHECK (legs_required_for_completion > 0),
  execution_strategy VARCHAR(50) NOT NULL CHECK (execution_strategy IN ('sequential', 'parallel', 'priority_ordered')),

  -- Execution state
  status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'partial', 'failed', 'rolled_back')),
  legs_placed INTEGER DEFAULT 0 CHECK (legs_placed >= 0),
  legs_failed INTEGER DEFAULT 0 CHECK (legs_failed >= 0),

  -- Timing
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  total_duration_ms INTEGER CHECK (total_duration_ms >= 0),

  -- Metadata
  execution_metadata JSONB DEFAULT '{}'::JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW(),

  -- Constraints
  CHECK (legs_placed + legs_failed <= total_legs),
  CHECK (legs_required_for_completion <= total_legs)
);

-- Individual leg execution tracking
CREATE TABLE IF NOT EXISTS auto_bet_leg_execution (
  id BIGSERIAL PRIMARY KEY,
  execution_tracking_id BIGINT REFERENCES auto_bet_execution_tracking(id),
  leg_number INTEGER NOT NULL CHECK (leg_number > 0),
  opportunity_leg_id BIGINT REFERENCES opportunity_legs(id),

  -- Leg details
  book_key VARCHAR(50) NOT NULL,
  outcome_name VARCHAR(100) NOT NULL,
  calculated_stake DECIMAL(10,2) NOT NULL CHECK (calculated_stake > 0),

  -- Execution
  status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'success', 'failed', 'cancelled', 'timeout')),
  bet_id BIGINT REFERENCES bets(id),

  -- Retry tracking
  attempt_number INTEGER DEFAULT 1 CHECK (attempt_number > 0),
  max_attempts INTEGER DEFAULT 3 CHECK (max_attempts > 0),

  -- Timing
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  duration_ms INTEGER CHECK (duration_ms >= 0),

  -- Error handling
  error_message TEXT,
  should_retry BOOLEAN DEFAULT TRUE,

  created_at TIMESTAMPTZ DEFAULT NOW(),

  -- Constraints
  CHECK (attempt_number <= max_attempts)
);

-- Indexes for fast queries
CREATE INDEX idx_execution_tracking_status ON auto_bet_execution_tracking(status, created_at DESC);
CREATE INDEX idx_execution_tracking_opportunity ON auto_bet_execution_tracking(opportunity_id);
CREATE INDEX idx_execution_tracking_decision ON auto_bet_execution_tracking(auto_decision_id);

CREATE INDEX idx_leg_execution_tracking ON auto_bet_leg_execution(execution_tracking_id, leg_number);
CREATE INDEX idx_leg_execution_status ON auto_bet_leg_execution(status, created_at DESC);
CREATE INDEX idx_leg_execution_bet ON auto_bet_leg_execution(bet_id) WHERE bet_id IS NOT NULL;

-- Comments for documentation
COMMENT ON TABLE auto_bet_execution_tracking IS 'Tracks overall execution of multi-leg opportunities (middle, scalp)';
COMMENT ON COLUMN auto_bet_execution_tracking.legs_required_for_completion IS 'Minimum legs needed for success (edge: 1, middle: 1 or 2, scalp: all)';
COMMENT ON COLUMN auto_bet_execution_tracking.execution_strategy IS 'How legs are executed: sequential (one at a time), parallel (all at once), priority_ordered (by edge)';
COMMENT ON COLUMN auto_bet_execution_tracking.status IS 'Overall status: completed (all required legs placed), partial (some legs placed), failed (insufficient legs)';

COMMENT ON TABLE auto_bet_leg_execution IS 'Tracks individual leg execution within multi-leg opportunities';
COMMENT ON COLUMN auto_bet_leg_execution.leg_number IS 'Execution order (1, 2, 3...) - not the same as opportunity_leg_id';
COMMENT ON COLUMN auto_bet_leg_execution.attempt_number IS 'Current retry attempt (1 = first attempt, 2 = first retry, etc)';
COMMENT ON COLUMN auto_bet_leg_execution.should_retry IS 'Whether to retry this leg on failure (false for non-retryable errors)';



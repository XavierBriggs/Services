-- Migration: Create opportunity_actions table
-- Description: Tracks user actions on opportunities (taken, dismissed, noted)
-- Author: Fortuna System
-- Date: 2025-11-07

CREATE TABLE IF NOT EXISTS opportunity_actions (
  id BIGSERIAL PRIMARY KEY,
  opportunity_id BIGINT NOT NULL REFERENCES opportunities(id) ON DELETE CASCADE,
  action_type VARCHAR(20) NOT NULL CHECK (action_type IN ('taken', 'dismissed', 'noted')),
  action_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  operator VARCHAR(100) NOT NULL,  -- Xavier, George
  notes TEXT,
  CONSTRAINT notes_required_for_noted CHECK (
    (action_type != 'noted') OR (action_type = 'noted' AND notes IS NOT NULL)
  )
);

-- Index for finding all actions on an opportunity
CREATE INDEX idx_opportunity_actions_opportunity ON opportunity_actions(opportunity_id, action_time DESC);

-- Index for operator activity tracking
CREATE INDEX idx_opportunity_actions_operator ON opportunity_actions(operator, action_time DESC);

-- Index for action type analysis
CREATE INDEX idx_opportunity_actions_type ON opportunity_actions(action_type, action_time DESC);

-- Comments for documentation
COMMENT ON TABLE opportunity_actions IS 'Tracks operator decisions on opportunities for analysis and accountability';
COMMENT ON COLUMN opportunity_actions.action_type IS 'taken: bet was placed, dismissed: opportunity ignored, noted: operator added a comment';
COMMENT ON COLUMN opportunity_actions.operator IS 'Name of operator who took action';
COMMENT ON COLUMN opportunity_actions.notes IS 'Optional notes (required for action_type=noted)';


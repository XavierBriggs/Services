-- Migration: Create bet_execution_logs table
-- Description: Tracks detailed execution logs for automated bet placement attempts
-- Author: Fortuna System
-- Date: 2025-11-12

CREATE TABLE IF NOT EXISTS bet_execution_logs (
  id BIGSERIAL PRIMARY KEY,
  bet_id BIGINT REFERENCES bets(id),  -- NULL if bet wasn't created yet
  opportunity_id BIGINT NOT NULL,  -- No FK constraint to allow testing/logging even if opportunity deleted
  book_key VARCHAR(50) NOT NULL,
  
  -- Execution details
  trigger_source VARCHAR(20) NOT NULL CHECK (trigger_source IN ('manual', 'automated')),
  execution_stage VARCHAR(50) NOT NULL,  -- 'login', 'navigation', 'bet_placement', 'confirmation', 'error'
  status VARCHAR(20) NOT NULL CHECK (status IN ('started', 'success', 'failed', 'timeout', 'cancelled')),
  
  -- Performance metrics
  latency_ms INT,
  
  -- Error tracking
  error_message TEXT,
  error_code VARCHAR(50),
  screenshot_path TEXT,
  
  -- Metadata
  executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  -- Constraints
  CONSTRAINT valid_latency CHECK (latency_ms >= 0 OR latency_ms IS NULL)
);

-- Index for bet tracking
CREATE INDEX idx_bet_execution_logs_bet ON bet_execution_logs(bet_id) 
  WHERE bet_id IS NOT NULL;

-- Index for opportunity analysis
CREATE INDEX idx_bet_execution_logs_opportunity ON bet_execution_logs(opportunity_id);

-- Index for time-based queries (recent logs first)
CREATE INDEX idx_bet_execution_logs_time ON bet_execution_logs(executed_at DESC);

-- Index for error analysis
CREATE INDEX idx_bet_execution_logs_errors ON bet_execution_logs(status, executed_at DESC) 
  WHERE status IN ('failed', 'timeout');

-- Index for book performance
CREATE INDEX idx_bet_execution_logs_book ON bet_execution_logs(book_key, executed_at DESC);

-- Index for trigger source analysis
CREATE INDEX idx_bet_execution_logs_trigger ON bet_execution_logs(trigger_source, executed_at DESC);

-- Comments for documentation
COMMENT ON TABLE bet_execution_logs IS 'Detailed execution logs for automated bet placement. Used for debugging, performance analysis, and success rate tracking.';
COMMENT ON COLUMN bet_execution_logs.bet_id IS 'Link to bet record (NULL if bet placement failed before record creation)';
COMMENT ON COLUMN bet_execution_logs.opportunity_id IS 'Link to opportunity that triggered this execution';
COMMENT ON COLUMN bet_execution_logs.book_key IS 'Sportsbook where bet placement was attempted';
COMMENT ON COLUMN bet_execution_logs.trigger_source IS 'How execution was triggered: manual (UI button) or automated (stream consumer)';
COMMENT ON COLUMN bet_execution_logs.execution_stage IS 'Stage where execution occurred: login, navigation, bet_placement, confirmation, error';
COMMENT ON COLUMN bet_execution_logs.status IS 'Execution outcome: started, success, failed, timeout, cancelled';
COMMENT ON COLUMN bet_execution_logs.latency_ms IS 'Execution time in milliseconds';
COMMENT ON COLUMN bet_execution_logs.error_message IS 'Human-readable error message if execution failed';
COMMENT ON COLUMN bet_execution_logs.error_code IS 'Machine-readable error code for categorization (e.g., ODDS_MOVED, LOGIN_FAILED, TIMEOUT)';
COMMENT ON COLUMN bet_execution_logs.screenshot_path IS 'Path to screenshot taken for debugging (if available)';


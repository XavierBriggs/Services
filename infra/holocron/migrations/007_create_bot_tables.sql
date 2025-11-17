-- Migration: Create bot management tables
-- Description: Creates tables for bot credentials and execution logging
-- Author: Fortuna Bot System
-- Date: 2025-01-15

-- ============================================================================
-- BOT CREDENTIALS TABLE
-- ============================================================================
-- Stores encrypted credentials for sportsbook bot authentication
CREATE TABLE IF NOT EXISTS bot_credentials (
  id BIGSERIAL PRIMARY KEY,
  book_key VARCHAR(50) NOT NULL UNIQUE,
  username_encrypted TEXT NOT NULL,
  password_encrypted TEXT NOT NULL,
  two_fa_secret_encrypted TEXT,
  session_data JSONB,
  last_login_at TIMESTAMPTZ,
  status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'locked', 'suspended')),
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for active credentials
CREATE INDEX IF NOT EXISTS idx_bot_credentials_status ON bot_credentials(status);

-- Comments
COMMENT ON TABLE bot_credentials IS 'Encrypted credentials for sportsbook bot authentication';
COMMENT ON COLUMN bot_credentials.book_key IS 'Sportsbook identifier (e.g., betonline, fanduel)';
COMMENT ON COLUMN bot_credentials.username_encrypted IS 'AES-256-GCM encrypted username';
COMMENT ON COLUMN bot_credentials.password_encrypted IS 'AES-256-GCM encrypted password';
COMMENT ON COLUMN bot_credentials.two_fa_secret_encrypted IS 'AES-256-GCM encrypted 2FA secret (optional)';
COMMENT ON COLUMN bot_credentials.session_data IS 'Saved session cookies and localStorage for session persistence';
COMMENT ON COLUMN bot_credentials.status IS 'Credential status: active, locked (failed logins), suspended (manual)';

-- ============================================================================
-- BET EXECUTION LOGS TABLE
-- ============================================================================
-- Logs every bet execution attempt for monitoring and debugging
CREATE TABLE IF NOT EXISTS bet_execution_logs (
  id BIGSERIAL PRIMARY KEY,
  bet_id BIGINT REFERENCES bets(id),
  opportunity_id BIGINT REFERENCES opportunities(id),
  book_key VARCHAR(50) NOT NULL,
  trigger_source VARCHAR(20) NOT NULL CHECK (trigger_source IN ('manual', 'automated')),
  execution_stage VARCHAR(50) NOT NULL,
  status VARCHAR(20) NOT NULL CHECK (status IN ('started', 'success', 'failed', 'odds_moved', 'timeout')),
  latency_ms INT,
  error_message TEXT,
  screenshot_path TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for querying and debugging
CREATE INDEX IF NOT EXISTS idx_bet_execution_logs_status ON bet_execution_logs(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bet_execution_logs_bet_id ON bet_execution_logs(bet_id);
CREATE INDEX IF NOT EXISTS idx_bet_execution_logs_opportunity ON bet_execution_logs(opportunity_id);
CREATE INDEX IF NOT EXISTS idx_bet_execution_logs_trigger ON bet_execution_logs(trigger_source, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bet_execution_logs_book ON bet_execution_logs(book_key, created_at DESC);

-- Comments
COMMENT ON TABLE bet_execution_logs IS 'Detailed logs of every bet execution attempt for monitoring and debugging';
COMMENT ON COLUMN bet_execution_logs.execution_stage IS 'Stage of execution: login, navigation, bet_placement, confirmation, error';
COMMENT ON COLUMN bet_execution_logs.status IS 'Execution status: started, success, failed, odds_moved, timeout';
COMMENT ON COLUMN bet_execution_logs.screenshot_path IS 'Path to screenshot saved on error for debugging';

-- ============================================================================
-- SEED DATA
-- ============================================================================
-- No seed data for bot tables - credentials must be added via API




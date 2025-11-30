-- Migration: Create slack_filter_preferences table
-- Description: Stores per-user Slack alert and bet filtering preferences
-- Author: Fortuna System
-- Date: 2025-11-28

CREATE TABLE IF NOT EXISTS slack_filter_preferences (
  slack_user_id VARCHAR(20) PRIMARY KEY,
  books_whitelist TEXT[] DEFAULT '{}',
  min_edge_percent DECIMAL(5,2) DEFAULT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  default_stake_cents INTEGER DEFAULT 10000,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for enabled users (for bulk alert filtering)
CREATE INDEX idx_slack_filter_preferences_enabled ON slack_filter_preferences(enabled) WHERE enabled = TRUE;

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_slack_filter_preferences_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_slack_filter_preferences_timestamp
  BEFORE UPDATE ON slack_filter_preferences
  FOR EACH ROW
  EXECUTE FUNCTION update_slack_filter_preferences_timestamp();

-- Comments for documentation
COMMENT ON TABLE slack_filter_preferences IS 'Per-user Slack alert and bet filtering preferences';
COMMENT ON COLUMN slack_filter_preferences.slack_user_id IS 'Slack user ID (e.g., U12345678)';
COMMENT ON COLUMN slack_filter_preferences.books_whitelist IS 'Array of allowed book keys. Empty array means all books allowed.';
COMMENT ON COLUMN slack_filter_preferences.min_edge_percent IS 'Minimum edge percentage for alerts/bets. NULL means no minimum.';
COMMENT ON COLUMN slack_filter_preferences.enabled IS 'Whether alerts and Slack betting are enabled for this user';
COMMENT ON COLUMN slack_filter_preferences.default_stake_cents IS 'Default stake amount in cents for bet confirmation modal';

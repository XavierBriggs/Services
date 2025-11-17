-- Migration: Create user_settings table
-- Description: Stores user configuration including per-book bankrolls and Kelly settings
-- Author: Fortuna System
-- Date: 2025-11-12

CREATE TABLE IF NOT EXISTS user_settings (
  id BIGSERIAL PRIMARY KEY,
  user_id VARCHAR(50) NOT NULL DEFAULT 'default',  -- Future: support multiple users
  
  -- Per-book bankrolls (stored as JSON for flexibility)
  bankrolls JSONB NOT NULL DEFAULT '{}'::jsonb,
  
  -- Kelly settings
  kelly_fraction DECIMAL(4,2) NOT NULL DEFAULT 0.25 CHECK (kelly_fraction > 0 AND kelly_fraction <= 1.0),
  min_edge_threshold DECIMAL(5,2) NOT NULL DEFAULT 1.0 CHECK (min_edge_threshold >= 0),
  max_stake_pct DECIMAL(5,2) NOT NULL DEFAULT 10.0 CHECK (max_stake_pct > 0 AND max_stake_pct <= 100),
  
  -- Metadata
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  -- Ensure one settings row per user
  CONSTRAINT unique_user UNIQUE (user_id)
);

-- Index for fast user lookup
CREATE INDEX idx_user_settings_user ON user_settings(user_id);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_user_settings_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_user_settings_timestamp
  BEFORE UPDATE ON user_settings
  FOR EACH ROW
  EXECUTE FUNCTION update_user_settings_timestamp();

-- Insert default settings for 'default' user
INSERT INTO user_settings (user_id, bankrolls, kelly_fraction, min_edge_threshold, max_stake_pct)
VALUES (
  'default',
  '{
    "fanduel": 0,
    "draftkings": 0,
    "betmgm": 0,
    "caesars": 0,
    "pointsbet": 0,
    "betrivers": 0,
    "hardrock": 0,
    "espnbet": 0
  }'::jsonb,
  0.25,
  1.0,
  10.0
)
ON CONFLICT (user_id) DO NOTHING;

-- Comments for documentation
COMMENT ON TABLE user_settings IS 'User configuration for bankroll management and Kelly sizing';
COMMENT ON COLUMN user_settings.bankrolls IS 'Per-book bankroll amounts in dollars (JSON: {"fanduel": 5000, "draftkings": 3000, ...})';
COMMENT ON COLUMN user_settings.kelly_fraction IS 'Kelly Criterion fraction (0.25 = 1/4 Kelly, 0.50 = 1/2 Kelly)';
COMMENT ON COLUMN user_settings.min_edge_threshold IS 'Minimum edge percentage threshold for alerts/bets';
COMMENT ON COLUMN user_settings.max_stake_pct IS 'Maximum stake as percentage of bankroll (safety cap)';




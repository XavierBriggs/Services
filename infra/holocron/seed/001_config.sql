-- Seed: Initial configuration for edge detection
-- Description: Default thresholds and settings for opportunity detection
-- Author: Fortuna System
-- Date: 2025-11-07

-- Note: These values can be overridden by environment variables
-- This seed is for reference and to establish baseline values

-- Configuration table (optional - can use env vars only)
-- Uncomment if you want database-backed configuration

/*
CREATE TABLE IF NOT EXISTS detector_config (
  config_key VARCHAR(50) PRIMARY KEY,
  config_value TEXT NOT NULL,
  description TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO detector_config (config_key, config_value, description) VALUES
  ('min_edge_pct', '0.01', 'Minimum edge percentage to create opportunity (1% = 0.01)'),
  ('max_data_age_seconds', '10', 'Maximum allowed data age for alerts'),
  ('alert_rate_limit', '10', 'Maximum alerts per minute'),
  ('dedup_ttl_minutes', '5', 'Deduplication TTL in minutes'),
  ('enable_middles', 'true', 'Enable middle opportunity detection'),
  ('enable_scalps', 'true', 'Enable scalp/arbitrage detection'),
  ('sharp_book_minimum', '1', 'Minimum number of sharp books required for consensus')
ON CONFLICT (config_key) DO NOTHING;
*/

-- Initial test data (optional - for development)
-- Uncomment to seed some test opportunities for UI development

/*
-- Sample opportunity for UI testing
INSERT INTO opportunities (opportunity_type, sport_key, event_id, market_key, edge_pct, fair_price, data_age_seconds)
VALUES 
  ('edge', 'basketball_nba', 'test_event_001', 'spreads', 2.500, -110, 3),
  ('middle', 'basketball_nba', 'test_event_002', 'totals', 3.200, NULL, 5),
  ('scalp', 'basketball_nba', 'test_event_003', 'h2h', 1.800, NULL, 2);

-- Sample legs for the test opportunities
INSERT INTO opportunity_legs (opportunity_id, book_key, outcome_name, price, point, leg_edge_pct)
VALUES 
  (1, 'fanduel', 'LAL +7.5', -105, 7.5, 2.500),
  (2, 'fanduel', 'Over 223.5', -108, 223.5, 1.800),
  (2, 'draftkings', 'Under 223.5', -110, 223.5, 1.400),
  (3, 'betmgm', 'LAL', -105, NULL, 0.900),
  (3, 'draftkings', 'LAC', -105, NULL, 0.900);
*/

-- Comments
COMMENT ON DATABASE holocron IS 'Fortuna betting opportunities and bet tracking database';



-- Migration: Add auto-betting configuration to user_settings
-- Description: Extends user_settings table with comprehensive auto-betting controls
-- Author: Fortuna System
-- Date: 2025-11-22

ALTER TABLE user_settings
  -- Master toggle
  ADD COLUMN IF NOT EXISTS auto_betting_enabled BOOLEAN DEFAULT FALSE,

  -- Edge & opportunity filters
  ADD COLUMN IF NOT EXISTS auto_min_edge_pct DECIMAL(6,3) DEFAULT 2.0,
  ADD COLUMN IF NOT EXISTS auto_enabled_opportunity_types VARCHAR[] DEFAULT ARRAY['edge']::VARCHAR[],
  ADD COLUMN IF NOT EXISTS auto_enabled_markets VARCHAR[] DEFAULT ARRAY['spreads', 'totals', 'h2h']::VARCHAR[],
  ADD COLUMN IF NOT EXISTS auto_enabled_books VARCHAR[] DEFAULT ARRAY[]::VARCHAR[],
  ADD COLUMN IF NOT EXISTS auto_disabled_books VARCHAR[] DEFAULT ARRAY[]::VARCHAR[],

  -- Risk management
  ADD COLUMN IF NOT EXISTS auto_max_stake_per_bet DECIMAL(10,2) DEFAULT 100.00,
  ADD COLUMN IF NOT EXISTS auto_max_exposure_per_event DECIMAL(10,2) DEFAULT 200.00,
  ADD COLUMN IF NOT EXISTS auto_max_exposure_total DECIMAL(10,2) DEFAULT 1000.00,
  ADD COLUMN IF NOT EXISTS auto_max_bets_per_hour INTEGER DEFAULT 10,
  ADD COLUMN IF NOT EXISTS auto_max_bets_per_day INTEGER DEFAULT 50,

  -- Kelly sizing parameters
  -- Note: kelly_fraction already exists in user_settings, reuse it for auto-betting
  ADD COLUMN IF NOT EXISTS auto_max_kelly_pct DECIMAL(5,2) DEFAULT 5.00,
  ADD COLUMN IF NOT EXISTS auto_min_stake DECIMAL(6,2) DEFAULT 10.00,

  -- Timing controls
  ADD COLUMN IF NOT EXISTS auto_max_data_age_seconds INTEGER DEFAULT 30,
  ADD COLUMN IF NOT EXISTS auto_min_time_to_start_hours INTEGER DEFAULT 1,
  ADD COLUMN IF NOT EXISTS auto_max_time_to_start_hours INTEGER DEFAULT 72,

  -- Safety features
  ADD COLUMN IF NOT EXISTS auto_pause_on_loss_streak INTEGER DEFAULT 5,
  ADD COLUMN IF NOT EXISTS auto_pause_on_daily_loss DECIMAL(10,2) DEFAULT 500.00,

  -- Advanced features
  ADD COLUMN IF NOT EXISTS auto_correlation_discount DECIMAL(4,3) DEFAULT 0.500,

  -- Bet type specific settings
  -- MIDDLE-specific
  ADD COLUMN IF NOT EXISTS auto_middle_enabled BOOLEAN DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS auto_middle_execution_strategy VARCHAR(20) DEFAULT 'parallel',
  ADD COLUMN IF NOT EXISTS auto_middle_required_legs INTEGER DEFAULT 1,
  ADD COLUMN IF NOT EXISTS auto_middle_max_time_between_legs_sec INTEGER DEFAULT 10,

  -- SCALP-specific
  ADD COLUMN IF NOT EXISTS auto_scalp_enabled BOOLEAN DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS auto_scalp_bankroll_pct DECIMAL(5,2) DEFAULT 5.00,
  ADD COLUMN IF NOT EXISTS auto_scalp_min_profit_pct DECIMAL(5,2) DEFAULT 0.5,
  ADD COLUMN IF NOT EXISTS auto_scalp_execution_timeout_sec INTEGER DEFAULT 30,

  -- EDGE-specific
  ADD COLUMN IF NOT EXISTS auto_edge_allow_live_games BOOLEAN DEFAULT FALSE;

-- Add constraints for validation
ALTER TABLE user_settings
  ADD CONSTRAINT check_auto_min_edge_pct CHECK (auto_min_edge_pct >= 0),
  ADD CONSTRAINT check_auto_max_stake_per_bet CHECK (auto_max_stake_per_bet > 0),
  ADD CONSTRAINT check_auto_max_exposure CHECK (auto_max_exposure_total >= auto_max_exposure_per_event),
  ADD CONSTRAINT check_auto_max_kelly_pct CHECK (auto_max_kelly_pct > 0 AND auto_max_kelly_pct <= 100),
  ADD CONSTRAINT check_auto_min_stake CHECK (auto_min_stake > 0),
  ADD CONSTRAINT check_auto_timing CHECK (auto_min_time_to_start_hours >= 0 AND auto_max_time_to_start_hours > auto_min_time_to_start_hours),
  ADD CONSTRAINT check_auto_correlation_discount CHECK (auto_correlation_discount >= 0 AND auto_correlation_discount <= 1),
  ADD CONSTRAINT check_auto_middle_required_legs CHECK (auto_middle_required_legs IN (1, 2)),
  ADD CONSTRAINT check_auto_scalp_bankroll_pct CHECK (auto_scalp_bankroll_pct > 0 AND auto_scalp_bankroll_pct <= 100),
  ADD CONSTRAINT check_auto_scalp_min_profit_pct CHECK (auto_scalp_min_profit_pct >= 0);

-- Comments for documentation
COMMENT ON COLUMN user_settings.auto_betting_enabled IS 'Master toggle for automated betting system';
COMMENT ON COLUMN user_settings.auto_min_edge_pct IS 'Minimum edge percentage required to auto-bet';
COMMENT ON COLUMN user_settings.auto_enabled_opportunity_types IS 'Array of enabled opportunity types: edge, middle, scalp';
COMMENT ON COLUMN user_settings.auto_enabled_markets IS 'Array of enabled markets: h2h, spreads, totals';
COMMENT ON COLUMN user_settings.auto_max_stake_per_bet IS 'Maximum stake amount for a single bet in dollars';
COMMENT ON COLUMN user_settings.auto_max_exposure_total IS 'Maximum total exposure across all active bets in dollars';
COMMENT ON COLUMN user_settings.auto_correlation_discount IS 'Discount factor for correlated bets (0-1, where 0.5 = 50% reduction)';
COMMENT ON COLUMN user_settings.auto_middle_enabled IS 'Enable automated middle betting';
COMMENT ON COLUMN user_settings.auto_scalp_enabled IS 'Enable automated scalp (arbitrage) betting';



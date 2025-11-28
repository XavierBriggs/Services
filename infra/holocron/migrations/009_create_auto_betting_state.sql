-- Migration: Create auto_betting_state table
-- Description: Tracks real-time state per user for auto-betting (exposure, rate limits, circuit breakers)
-- Author: Fortuna System
-- Date: 2025-11-22

CREATE TABLE IF NOT EXISTS auto_betting_state (
  user_id VARCHAR(50) PRIMARY KEY,

  -- Current exposure tracking
  total_exposure DECIMAL(12,2) DEFAULT 0.00 CHECK (total_exposure >= 0),
  exposure_by_event JSONB DEFAULT '{}'::JSONB,
  exposure_by_book JSONB DEFAULT '{}'::JSONB,

  -- Rate limiting
  bets_placed_last_hour INTEGER DEFAULT 0 CHECK (bets_placed_last_hour >= 0),
  bets_placed_today INTEGER DEFAULT 0 CHECK (bets_placed_today >= 0),
  last_bet_placed_at TIMESTAMPTZ,

  -- Performance tracking
  todays_pnl DECIMAL(10,2) DEFAULT 0.00,
  current_loss_streak INTEGER DEFAULT 0 CHECK (current_loss_streak >= 0),
  total_bets_placed BIGINT DEFAULT 0 CHECK (total_bets_placed >= 0),
  total_bets_won BIGINT DEFAULT 0 CHECK (total_bets_won >= 0),
  total_bets_lost BIGINT DEFAULT 0 CHECK (total_bets_lost >= 0),

  -- Safety circuit breakers
  is_paused BOOLEAN DEFAULT FALSE,
  pause_reason TEXT,
  paused_at TIMESTAMPTZ,
  paused_until TIMESTAMPTZ,

  -- Metadata
  last_updated TIMESTAMPTZ DEFAULT NOW(),

  -- Constraints
  CHECK (total_bets_won + total_bets_lost <= total_bets_placed)
);

-- Insert default state for 'default' user
INSERT INTO auto_betting_state (user_id)
VALUES ('default')
ON CONFLICT (user_id) DO NOTHING;

-- Create function to update last_updated timestamp
CREATE OR REPLACE FUNCTION update_auto_betting_state_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.last_updated = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to auto-update timestamp
CREATE TRIGGER trigger_update_auto_betting_state_timestamp
  BEFORE UPDATE ON auto_betting_state
  FOR EACH ROW
  EXECUTE FUNCTION update_auto_betting_state_timestamp();

-- Comments for documentation
COMMENT ON TABLE auto_betting_state IS 'Real-time state tracking for automated betting per user (exposure, rate limits, circuit breakers)';
COMMENT ON COLUMN auto_betting_state.total_exposure IS 'Sum of all active bet stakes (decreases when bets settle)';
COMMENT ON COLUMN auto_betting_state.exposure_by_event IS 'JSON object mapping event_id to exposure amount for that event';
COMMENT ON COLUMN auto_betting_state.exposure_by_book IS 'JSON object mapping book_key to exposure amount at that book';
COMMENT ON COLUMN auto_betting_state.bets_placed_last_hour IS 'Rolling counter of bets placed in last 60 minutes (for rate limiting)';
COMMENT ON COLUMN auto_betting_state.is_paused IS 'Circuit breaker flag - auto-betting is paused for this user';
COMMENT ON COLUMN auto_betting_state.pause_reason IS 'Human-readable reason for pause (loss streak, daily loss, manual, etc)';



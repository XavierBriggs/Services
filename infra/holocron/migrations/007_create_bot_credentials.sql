-- Migration: Create bot_credentials table
-- Description: Stores encrypted credentials for automated betting bots to login to sportsbooks
-- Author: Fortuna System
-- Date: 2025-11-12

CREATE TABLE IF NOT EXISTS bot_credentials (
  id BIGSERIAL PRIMARY KEY,
  book_key VARCHAR(50) NOT NULL,
  username_encrypted TEXT NOT NULL,
  password_encrypted TEXT NOT NULL,
  two_fa_secret_encrypted TEXT,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  
  -- Session management
  session_data JSONB,
  last_login_at TIMESTAMPTZ,
  session_valid_until TIMESTAMPTZ,
  login_failure_count INT NOT NULL DEFAULT 0,
  
  -- Metadata
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  
  -- Constraints
  CONSTRAINT unique_book_credentials UNIQUE (book_key),
  CONSTRAINT non_empty_username CHECK (length(username_encrypted) > 0),
  CONSTRAINT non_empty_password CHECK (length(password_encrypted) > 0),
  CONSTRAINT valid_failure_count CHECK (login_failure_count >= 0),
  CONSTRAINT valid_status CHECK (status IN ('active', 'inactive', 'disabled'))
);

-- Index for fast book lookup
CREATE INDEX idx_bot_credentials_book ON bot_credentials(book_key);

-- Index for active credentials lookup
CREATE INDEX idx_bot_credentials_status ON bot_credentials(status) 
  WHERE status = 'active';

-- Index for session expiry checks
CREATE INDEX idx_bot_credentials_session ON bot_credentials(session_valid_until) 
  WHERE session_valid_until IS NOT NULL;

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_bot_credentials_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_bot_credentials_timestamp
  BEFORE UPDATE ON bot_credentials
  FOR EACH ROW
  EXECUTE FUNCTION update_bot_credentials_timestamp();

-- Comments for documentation
COMMENT ON TABLE bot_credentials IS 'Encrypted credentials for automated betting bots. Uses AES-256 encryption via BOT_ENCRYPTION_KEY.';
COMMENT ON COLUMN bot_credentials.book_key IS 'Sportsbook identifier (betonline, fanduel, draftkings, etc.)';
COMMENT ON COLUMN bot_credentials.username_encrypted IS 'AES-256 encrypted username/email for sportsbook login';
COMMENT ON COLUMN bot_credentials.password_encrypted IS 'AES-256 encrypted password for sportsbook login';
COMMENT ON COLUMN bot_credentials.two_fa_secret_encrypted IS 'Optional AES-256 encrypted 2FA/TOTP secret for sportsbook (if applicable)';
COMMENT ON COLUMN bot_credentials.status IS 'Credential status: active (in use), inactive (temporarily disabled), disabled (permanently disabled)';
COMMENT ON COLUMN bot_credentials.session_data IS 'JSON data for session reuse (cookies, tokens, etc.) to avoid frequent logins';
COMMENT ON COLUMN bot_credentials.last_login_at IS 'Timestamp of last successful bot login to this sportsbook';
COMMENT ON COLUMN bot_credentials.session_valid_until IS 'Timestamp when current session expires (for session reuse optimization)';
COMMENT ON COLUMN bot_credentials.login_failure_count IS 'Counter for consecutive login failures (reset on success, alert if >3)';


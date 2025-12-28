-- Migration: Add game_status to opportunities table
-- Description: Track whether opportunity was detected for live or pregame market
-- Author: Fortuna System
-- Date: 2025-11-27

-- Add game_status column to opportunities
ALTER TABLE opportunities 
ADD COLUMN IF NOT EXISTS game_status VARCHAR(20) DEFAULT 'upcoming' 
CHECK (game_status IN ('upcoming', 'live'));

-- Add index for filtering
CREATE INDEX IF NOT EXISTS idx_opportunities_game_status 
ON opportunities(game_status, detected_at DESC);

-- Comment for documentation
COMMENT ON COLUMN opportunities.game_status IS 'Game status when opportunity detected: upcoming (pregame) or live (in-progress)';





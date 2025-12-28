-- Migration: Update analytics_book_stats primary key to include game_status
-- Description: Add game_status to primary key for proper pregame vs live segregation
-- Author: Fortuna System
-- Date: 2025-11-27

-- Step 1: Add game_status column if it doesn't exist
ALTER TABLE analytics_book_stats 
ADD COLUMN IF NOT EXISTS game_status VARCHAR(20) DEFAULT 'upcoming' 
CHECK (game_status IN ('upcoming', 'live', 'pregame'));

-- Step 2: Update existing records to have a default game_status
UPDATE analytics_book_stats 
SET game_status = 'upcoming' 
WHERE game_status IS NULL;

-- Step 3: Drop the old primary key constraint
ALTER TABLE analytics_book_stats 
DROP CONSTRAINT IF EXISTS analytics_book_stats_pkey;

-- Step 4: Create new primary key including game_status
ALTER TABLE analytics_book_stats 
ADD PRIMARY KEY (timestamp_bucket, book_key, opportunity_type, game_status);

-- Step 5: Add index for game_status filtering
CREATE INDEX IF NOT EXISTS idx_analytics_game_status 
ON analytics_book_stats(game_status, timestamp_bucket DESC);

-- Step 6: Add composite index for common queries with game_status
CREATE INDEX IF NOT EXISTS idx_analytics_book_type_status 
ON analytics_book_stats(book_key, opportunity_type, game_status, timestamp_bucket DESC);

-- Comments for documentation
COMMENT ON COLUMN analytics_book_stats.game_status IS 'Game status when opportunity detected: upcoming (pregame), live (in-progress)';





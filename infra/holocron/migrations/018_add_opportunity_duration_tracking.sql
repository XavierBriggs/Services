-- Migration 018: Add duration tracking columns to opportunities
-- This enables accurate analytics by tracking unique opportunities rather than repeated emissions

-- Add new columns for duration tracking
ALTER TABLE opportunities
ADD COLUMN IF NOT EXISTS first_seen_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS duration_seconds INT,
ADD COLUMN IF NOT EXISTS emission_count INT DEFAULT 1,
ADD COLUMN IF NOT EXISTS signature VARCHAR(500);

-- Set first_seen_at and last_seen_at to detected_at for existing records
UPDATE opportunities 
SET first_seen_at = detected_at,
    last_seen_at = detected_at
WHERE first_seen_at IS NULL;

-- Make first_seen_at NOT NULL after backfill
ALTER TABLE opportunities 
ALTER COLUMN first_seen_at SET NOT NULL,
ALTER COLUMN first_seen_at SET DEFAULT NOW();

ALTER TABLE opportunities 
ALTER COLUMN last_seen_at SET NOT NULL,
ALTER COLUMN last_seen_at SET DEFAULT NOW();

-- Add unique index on signature for upsert operations
-- Signature format: "type|eventID|marketKey|book1:outcome1|book2:outcome2"
CREATE UNIQUE INDEX IF NOT EXISTS idx_opportunities_signature 
ON opportunities(signature) 
WHERE signature IS NOT NULL;

-- Add index for finding stale opportunities (for finalization)
CREATE INDEX IF NOT EXISTS idx_opportunities_last_seen 
ON opportunities(last_seen_at DESC);

-- Add index for duration analytics
CREATE INDEX IF NOT EXISTS idx_opportunities_duration 
ON opportunities(duration_seconds) 
WHERE duration_seconds IS NOT NULL;

-- Add constraint for valid duration
ALTER TABLE opportunities
ADD CONSTRAINT valid_duration CHECK (duration_seconds IS NULL OR duration_seconds >= 0);

-- Add constraint for emission count
ALTER TABLE opportunities
ADD CONSTRAINT valid_emission_count CHECK (emission_count >= 1);

-- Comments
COMMENT ON COLUMN opportunities.first_seen_at IS 'When this opportunity was first detected';
COMMENT ON COLUMN opportunities.last_seen_at IS 'When this opportunity was last seen (updated on each poll)';
COMMENT ON COLUMN opportunities.duration_seconds IS 'Total duration from first to last seen (set when opportunity disappears)';
COMMENT ON COLUMN opportunities.emission_count IS 'Number of times this opportunity was emitted to stream';
COMMENT ON COLUMN opportunities.signature IS 'Unique signature for deduplication: type|event|market|legs';





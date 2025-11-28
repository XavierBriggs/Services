-- Migration: Add opportunity CLV columns to analytics_book_stats
-- Description: Adds aggregated opportunity CLV metrics for unified dashboard
-- Author: Fortuna System
-- Date: 2025-11-27

-- Add opportunity CLV summary columns to analytics_book_stats
ALTER TABLE analytics_book_stats 
ADD COLUMN IF NOT EXISTS avg_opportunity_clv DECIMAL(10,2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS opportunity_clv_count INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS opportunity_clv_accuracy DECIMAL(5,2) DEFAULT 0;

-- Comments for documentation
COMMENT ON COLUMN analytics_book_stats.avg_opportunity_clv IS 'Average CLV for detected opportunities in this bucket (validates edge detector accuracy)';
COMMENT ON COLUMN analytics_book_stats.opportunity_clv_count IS 'Number of opportunity legs with CLV calculated in this bucket';
COMMENT ON COLUMN analytics_book_stats.opportunity_clv_accuracy IS 'Percentage of opportunities with positive CLV (edge was real)';

-- Also add to analytics_book_pairs for scalp/middle validation
ALTER TABLE analytics_book_pairs
ADD COLUMN IF NOT EXISTS avg_opportunity_clv DECIMAL(10,2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS opportunity_clv_count INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS opportunity_clv_accuracy DECIMAL(5,2) DEFAULT 0;

COMMENT ON COLUMN analytics_book_pairs.avg_opportunity_clv IS 'Average CLV for this book pair combination';
COMMENT ON COLUMN analytics_book_pairs.opportunity_clv_count IS 'Number of opportunity legs with CLV calculated';
COMMENT ON COLUMN analytics_book_pairs.opportunity_clv_accuracy IS 'Percentage of opportunities with positive CLV';


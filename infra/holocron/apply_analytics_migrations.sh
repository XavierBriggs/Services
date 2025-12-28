#!/bin/bash
# ==============================================================================
# Apply Analytics Migrations to Holocron Database
# ==============================================================================
# This script applies the analytics-related migrations (011-015) to an existing
# Holocron database. Run this if your database was created before these 
# migrations were added.
#
# Usage:
#   ./apply_analytics_migrations.sh [--dry-run]
#
# Environment Variables:
#   HOLOCRON_HOST     - Database host (default: localhost)
#   HOLOCRON_PORT     - Database port (default: 5436)
#   HOLOCRON_USER     - Database user (default: fortuna)
#   HOLOCRON_PASSWORD - Database password (default: fortuna_dev_password)
#   HOLOCRON_DB       - Database name (default: holocron)
# ==============================================================================

set -e

# Configuration
HOLOCRON_HOST="${HOLOCRON_HOST:-localhost}"
HOLOCRON_PORT="${HOLOCRON_PORT:-5436}"
HOLOCRON_USER="${HOLOCRON_USER:-fortuna}"
HOLOCRON_PASSWORD="${HOLOCRON_PASSWORD:-fortuna_dev_password}"
HOLOCRON_DB="${HOLOCRON_DB:-holocron}"

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIGRATIONS_DIR="$SCRIPT_DIR/migrations"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check for dry-run mode
DRY_RUN=false
if [[ "$1" == "--dry-run" ]]; then
    DRY_RUN=true
    echo -e "${YELLOW}=== DRY RUN MODE ===${NC}"
fi

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}  Holocron Analytics Migrations${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""
echo -e "Host: ${GREEN}$HOLOCRON_HOST:$HOLOCRON_PORT${NC}"
echo -e "Database: ${GREEN}$HOLOCRON_DB${NC}"
echo -e "User: ${GREEN}$HOLOCRON_USER${NC}"
echo ""

# Analytics migrations to apply
MIGRATIONS=(
    "011_create_analytics_book_stats.sql"
    "012_enhance_analytics.sql"
    "013_add_game_status_to_opportunities.sql"
    "014_update_analytics_pk_game_status.sql"
    "015_create_analytics_book_pairs.sql"
)

# Function to run a migration
run_migration() {
    local migration="$1"
    local migration_path="$MIGRATIONS_DIR/$migration"
    
    if [[ ! -f "$migration_path" ]]; then
        echo -e "${RED}✗ Migration file not found: $migration${NC}"
        return 1
    fi
    
    echo -e "${YELLOW}→ Applying: $migration${NC}"
    
    if [[ "$DRY_RUN" == true ]]; then
        echo -e "  ${BLUE}(dry run - would execute $migration_path)${NC}"
        return 0
    fi
    
    # Run the migration
    PGPASSWORD="$HOLOCRON_PASSWORD" psql \
        -h "$HOLOCRON_HOST" \
        -p "$HOLOCRON_PORT" \
        -U "$HOLOCRON_USER" \
        -d "$HOLOCRON_DB" \
        -f "$migration_path" \
        -v ON_ERROR_STOP=1 \
        2>&1 | while read line; do
            # Suppress common noise but show important messages
            if [[ "$line" == *"ERROR"* ]]; then
                echo -e "  ${RED}$line${NC}"
            elif [[ "$line" == *"NOTICE"* ]]; then
                echo -e "  ${YELLOW}$line${NC}"
            elif [[ "$line" != "" && "$line" != *"SET"* && "$line" != *"ALTER TABLE"* && "$line" != *"CREATE"* ]]; then
                echo -e "  $line"
            fi
        done
    
    if [[ $? -eq 0 ]]; then
        echo -e "${GREEN}✓ Applied: $migration${NC}"
    else
        echo -e "${RED}✗ Failed: $migration${NC}"
        return 1
    fi
}

# Check if psql is available
if ! command -v psql &> /dev/null; then
    echo -e "${RED}Error: psql command not found. Please install PostgreSQL client.${NC}"
    echo ""
    echo "On macOS: brew install libpq && brew link --force libpq"
    echo "On Ubuntu: sudo apt-get install postgresql-client"
    exit 1
fi

# Test database connection
echo -e "${BLUE}Testing database connection...${NC}"
if ! PGPASSWORD="$HOLOCRON_PASSWORD" psql -h "$HOLOCRON_HOST" -p "$HOLOCRON_PORT" -U "$HOLOCRON_USER" -d "$HOLOCRON_DB" -c "SELECT 1" > /dev/null 2>&1; then
    echo -e "${RED}Error: Cannot connect to database. Please check your connection settings.${NC}"
    echo ""
    echo "Make sure Holocron is running:"
    echo "  cd /path/to/fortuna/deploy && docker-compose up -d holocron"
    exit 1
fi
echo -e "${GREEN}✓ Connected to database${NC}"
echo ""

# Apply each migration
echo -e "${BLUE}Applying migrations...${NC}"
echo ""

for migration in "${MIGRATIONS[@]}"; do
    run_migration "$migration"
    echo ""
done

echo -e "${GREEN}======================================${NC}"
echo -e "${GREEN}  All migrations applied successfully!${NC}"
echo -e "${GREEN}======================================${NC}"
echo ""
echo -e "You can verify the tables with:"
echo -e "  ${BLUE}psql -h $HOLOCRON_HOST -p $HOLOCRON_PORT -U $HOLOCRON_USER -d $HOLOCRON_DB${NC}"
echo ""
echo "  \\dt analytics*    -- List analytics tables"
echo "  \\d+ analytics_book_stats   -- Show book stats schema"
echo "  \\d+ analytics_book_pairs   -- Show book pairs schema"





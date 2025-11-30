package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/slack-gateway/pkg/models"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

const (
	// Redis cache key prefix
	prefsCachePrefix = "slack_prefs:"
	// Cache TTL
	prefsCacheTTL = 60 * time.Second
)

// PreferenceStore handles loading and saving Slack filter preferences
type PreferenceStore struct {
	db    *sql.DB
	redis *redis.Client
}

// NewPreferenceStore creates a new preference store
func NewPreferenceStore(db *sql.DB, redisClient *redis.Client) *PreferenceStore {
	return &PreferenceStore{
		db:    db,
		redis: redisClient,
	}
}

// LoadPreference loads user preferences, first checking Redis cache
func (s *PreferenceStore) LoadPreference(ctx context.Context, slackUserID string) (*models.SlackFilterPreference, error) {
	// Try cache first
	cacheKey := prefsCachePrefix + slackUserID
	cached, err := s.redis.Get(ctx, cacheKey).Bytes()
	if err == nil {
		var pref models.SlackFilterPreference
		if err := json.Unmarshal(cached, &pref); err == nil {
			return &pref, nil
		}
	}

	// Cache miss, load from database
	pref, err := s.loadFromDB(ctx, slackUserID)
	if err != nil {
		return nil, err
	}

	// If no preference exists, return nil (use defaults)
	if pref == nil {
		return nil, nil
	}

	// Cache the result
	if data, err := json.Marshal(pref); err == nil {
		s.redis.Set(ctx, cacheKey, data, prefsCacheTTL)
	}

	return pref, nil
}

// loadFromDB loads preferences directly from the database
func (s *PreferenceStore) loadFromDB(ctx context.Context, slackUserID string) (*models.SlackFilterPreference, error) {
	query := `
		SELECT slack_user_id, books_whitelist, min_edge_percent, enabled, default_stake_cents, updated_at
		FROM slack_filter_preferences
		WHERE slack_user_id = $1
	`

	var pref models.SlackFilterPreference
	var minEdge sql.NullFloat64
	var booksWhitelist pq.StringArray

	err := s.db.QueryRowContext(ctx, query, slackUserID).Scan(
		&pref.SlackUserID,
		&booksWhitelist,
		&minEdge,
		&pref.Enabled,
		&pref.DefaultStakeCents,
		&pref.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load preference: %w", err)
	}

	pref.BooksWhitelist = []string(booksWhitelist)
	if minEdge.Valid {
		pref.MinEdgePercent = &minEdge.Float64
	}

	return &pref, nil
}

// SavePreference saves user preferences to the database and updates cache
func (s *PreferenceStore) SavePreference(ctx context.Context, pref *models.SlackFilterPreference) error {
	query := `
		INSERT INTO slack_filter_preferences (slack_user_id, books_whitelist, min_edge_percent, enabled, default_stake_cents)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (slack_user_id) DO UPDATE SET
			books_whitelist = EXCLUDED.books_whitelist,
			min_edge_percent = EXCLUDED.min_edge_percent,
			enabled = EXCLUDED.enabled,
			default_stake_cents = EXCLUDED.default_stake_cents,
			updated_at = NOW()
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		pref.SlackUserID,
		pq.Array(pref.BooksWhitelist),
		pref.MinEdgePercent,
		pref.Enabled,
		pref.DefaultStakeCents,
	)

	if err != nil {
		return fmt.Errorf("failed to save preference: %w", err)
	}

	// Invalidate cache
	cacheKey := prefsCachePrefix + pref.SlackUserID
	s.redis.Del(ctx, cacheKey)

	return nil
}

// GetDefaultPreference returns default preferences for a user
func GetDefaultPreference(slackUserID string) *models.SlackFilterPreference {
	return &models.SlackFilterPreference{
		SlackUserID:       slackUserID,
		BooksWhitelist:    []string{}, // Empty = all books
		MinEdgePercent:    nil,        // No minimum
		Enabled:           true,
		DefaultStakeCents: 10000, // $100 default
		UpdatedAt:         time.Now(),
	}
}

// OpportunityStore handles loading opportunities from Holocron
type OpportunityStore struct {
	db *sql.DB
}

// NewOpportunityStore creates a new opportunity store
func NewOpportunityStore(db *sql.DB) *OpportunityStore {
	return &OpportunityStore{db: db}
}

// LoadOpportunity loads an opportunity by ID
func (s *OpportunityStore) LoadOpportunity(ctx context.Context, oppID int64) (*models.Opportunity, error) {
	query := `
		SELECT id, opportunity_type, sport_key, event_id, market_key, edge_pct, fair_price, detected_at, data_age_seconds
		FROM opportunities
		WHERE id = $1
	`

	var opp models.Opportunity
	var fairPrice sql.NullInt64

	err := s.db.QueryRowContext(ctx, query, oppID).Scan(
		&opp.ID,
		&opp.OpportunityType,
		&opp.SportKey,
		&opp.EventID,
		&opp.MarketKey,
		&opp.EdgePercent,
		&fairPrice,
		&opp.DetectedAt,
		&opp.DataAgeSeconds,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("opportunity not found: %d", oppID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load opportunity: %w", err)
	}

	if fairPrice.Valid {
		fp := int(fairPrice.Int64)
		opp.FairPrice = &fp
	}

	// Load legs
	legs, err := s.loadOpportunityLegs(ctx, oppID)
	if err != nil {
		return nil, err
	}
	opp.Legs = legs

	return &opp, nil
}

// loadOpportunityLegs loads all legs for an opportunity
func (s *OpportunityStore) loadOpportunityLegs(ctx context.Context, oppID int64) ([]models.OpportunityLeg, error) {
	query := `
		SELECT book_key, outcome_name, price, point, leg_edge_pct
		FROM opportunity_legs
		WHERE opportunity_id = $1
		ORDER BY id
	`

	rows, err := s.db.QueryContext(ctx, query, oppID)
	if err != nil {
		return nil, fmt.Errorf("failed to load legs: %w", err)
	}
	defer rows.Close()

	var legs []models.OpportunityLeg
	for rows.Next() {
		var leg models.OpportunityLeg
		var point, legEdge sql.NullFloat64

		err := rows.Scan(
			&leg.BookKey,
			&leg.OutcomeName,
			&leg.Price,
			&point,
			&legEdge,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan leg: %w", err)
		}

		if point.Valid {
			leg.Point = &point.Float64
		}
		if legEdge.Valid {
			leg.LegEdgePercent = &legEdge.Float64
		}

		legs = append(legs, leg)
	}

	return legs, nil
}

// GetEventInfo fetches event info from Alexandria database
func (s *OpportunityStore) GetEventInfo(ctx context.Context, alexandriaDB *sql.DB, eventID string) (homeTeam, awayTeam string, err error) {
	query := `
		SELECT home_team, away_team
		FROM events
		WHERE event_id = $1
	`

	err = alexandriaDB.QueryRowContext(ctx, query, eventID).Scan(&homeTeam, &awayTeam)
	if err != nil {
		return "", "", fmt.Errorf("failed to get event info: %w", err)
	}

	return homeTeam, awayTeam, nil
}


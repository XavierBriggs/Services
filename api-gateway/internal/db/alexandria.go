package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/XavierBriggs/fortuna/services/api-gateway/pkg/models"
	_ "github.com/lib/pq"
)

// AlexandriaDB defines the interface for Alexandria database operations
type AlexandriaDB interface {
	GetEvents(ctx context.Context, filters EventFilters) ([]models.Event, error)
	GetEvent(ctx context.Context, eventID string) (*models.Event, error)
	GetCurrentOdds(ctx context.Context, filters OddsFilters) ([]models.CurrentOdds, error)
	GetOddsHistory(ctx context.Context, filters OddsHistoryFilters) ([]models.OddsHistory, error)
	GetEventWithOdds(ctx context.Context, eventID string) (*models.EventWithOdds, error)
	Close() error
	Ping(ctx context.Context) error
}

// EventFilters contains filters for querying events
type EventFilters struct {
	SportKey     string
	EventStatus  string // upcoming, live, completed
	Limit        int
	Offset       int
}

// OddsFilters contains filters for querying current odds
type OddsFilters struct {
	EventID   string
	SportKey  string
	MarketKey string
	BookKey   string
	Limit     int
	Offset    int
}

// OddsHistoryFilters contains filters for querying odds history
type OddsHistoryFilters struct {
	EventID   string
	MarketKey string
	BookKey   string
	Since     *time.Time
	Until     *time.Time
	Limit     int
	Offset    int
}

// Client implements AlexandriaDB interface
type Client struct {
	db *sql.DB
}

// NewClient creates a new Alexandria DB client
func NewClient(dsn string) (*Client, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Client{db: db}, nil
}

// GetEvents retrieves events with optional filtering
func (c *Client) GetEvents(ctx context.Context, filters EventFilters) ([]models.Event, error) {
	query := `
		SELECT event_id, sport_key, home_team, away_team, commence_time,
		       event_status, discovered_at, last_seen_at
		FROM events
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if filters.SportKey != "" {
		query += fmt.Sprintf(" AND sport_key = $%d", argIdx)
		args = append(args, filters.SportKey)
		argIdx++
	}

	if filters.EventStatus != "" {
		query += fmt.Sprintf(" AND event_status = $%d", argIdx)
		args = append(args, filters.EventStatus)
		argIdx++
	}

	query += " ORDER BY commence_time ASC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filters.Limit)
		argIdx++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filters.Offset)
	}

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(
			&e.EventID, &e.SportKey, &e.HomeTeam, &e.AwayTeam,
			&e.CommenceTime, &e.EventStatus, &e.DiscoveredAt, &e.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}

// GetEvent retrieves a single event by ID
func (c *Client) GetEvent(ctx context.Context, eventID string) (*models.Event, error) {
	query := `
		SELECT event_id, sport_key, home_team, away_team, commence_time,
		       event_status, discovered_at, last_seen_at
		FROM events
		WHERE event_id = $1
	`

	var e models.Event
	err := c.db.QueryRowContext(ctx, query, eventID).Scan(
		&e.EventID, &e.SportKey, &e.HomeTeam, &e.AwayTeam,
		&e.CommenceTime, &e.EventStatus, &e.DiscoveredAt, &e.LastSeenAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query event: %w", err)
	}

	return &e, nil
}

// GetCurrentOdds retrieves the latest odds with filtering
func (c *Client) GetCurrentOdds(ctx context.Context, filters OddsFilters) ([]models.CurrentOdds, error) {
	query := `
		SELECT event_id, sport_key, market_key, book_key, outcome_name,
		       price, point, vendor_last_update, received_at,
		       EXTRACT(EPOCH FROM (NOW() - received_at)) as data_age_seconds
		FROM odds_raw
		WHERE is_latest = true
	`
	args := []interface{}{}
	argIdx := 1

	if filters.EventID != "" {
		query += fmt.Sprintf(" AND event_id = $%d", argIdx)
		args = append(args, filters.EventID)
		argIdx++
	}

	if filters.SportKey != "" {
		query += fmt.Sprintf(" AND sport_key = $%d", argIdx)
		args = append(args, filters.SportKey)
		argIdx++
	}

	if filters.MarketKey != "" {
		query += fmt.Sprintf(" AND market_key = $%d", argIdx)
		args = append(args, filters.MarketKey)
		argIdx++
	}

	if filters.BookKey != "" {
		query += fmt.Sprintf(" AND book_key = $%d", argIdx)
		args = append(args, filters.BookKey)
		argIdx++
	}

	query += " ORDER BY received_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filters.Limit)
		argIdx++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filters.Offset)
	}

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query current odds: %w", err)
	}
	defer rows.Close()

	var odds []models.CurrentOdds
	for rows.Next() {
		var o models.CurrentOdds
		if err := rows.Scan(
			&o.EventID, &o.SportKey, &o.MarketKey, &o.BookKey, &o.OutcomeName,
			&o.Price, &o.Point, &o.VendorLastUpdate, &o.ReceivedAt, &o.DataAge,
		); err != nil {
			return nil, fmt.Errorf("scan odds: %w", err)
		}
		odds = append(odds, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate odds: %w", err)
	}

	return odds, nil
}

// GetOddsHistory retrieves historical odds data
func (c *Client) GetOddsHistory(ctx context.Context, filters OddsHistoryFilters) ([]models.OddsHistory, error) {
	query := `
		SELECT id, event_id, sport_key, market_key, book_key, outcome_name,
		       price, point, vendor_last_update, received_at, is_latest
		FROM odds_raw
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if filters.EventID != "" {
		query += fmt.Sprintf(" AND event_id = $%d", argIdx)
		args = append(args, filters.EventID)
		argIdx++
	}

	if filters.MarketKey != "" {
		query += fmt.Sprintf(" AND market_key = $%d", argIdx)
		args = append(args, filters.MarketKey)
		argIdx++
	}

	if filters.BookKey != "" {
		query += fmt.Sprintf(" AND book_key = $%d", argIdx)
		args = append(args, filters.BookKey)
		argIdx++
	}

	if filters.Since != nil {
		query += fmt.Sprintf(" AND received_at >= $%d", argIdx)
		args = append(args, *filters.Since)
		argIdx++
	}

	if filters.Until != nil {
		query += fmt.Sprintf(" AND received_at <= $%d", argIdx)
		args = append(args, *filters.Until)
		argIdx++
	}

	query += " ORDER BY received_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filters.Limit)
		argIdx++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filters.Offset)
	}

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query odds history: %w", err)
	}
	defer rows.Close()

	var history []models.OddsHistory
	for rows.Next() {
		var h models.OddsHistory
		if err := rows.Scan(
			&h.ID, &h.EventID, &h.SportKey, &h.MarketKey, &h.BookKey, &h.OutcomeName,
			&h.Price, &h.Point, &h.VendorLastUpdate, &h.ReceivedAt, &h.IsLatest,
		); err != nil {
			return nil, fmt.Errorf("scan odds history: %w", err)
		}
		history = append(history, h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate odds history: %w", err)
	}

	return history, nil
}

// GetEventWithOdds retrieves an event with its current odds
func (c *Client) GetEventWithOdds(ctx context.Context, eventID string) (*models.EventWithOdds, error) {
	// Get event
	event, err := c.GetEvent(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	if event == nil {
		return nil, nil
	}

	// Get current odds for event
	odds, err := c.GetCurrentOdds(ctx, OddsFilters{EventID: eventID})
	if err != nil {
		return nil, fmt.Errorf("get current odds: %w", err)
	}

	return &models.EventWithOdds{
		Event:       *event,
		CurrentOdds: odds,
	}, nil
}

// Close closes the database connection
func (c *Client) Close() error {
	return c.db.Close()
}

// Ping checks database connectivity
func (c *Client) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}


package tradenexus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

type Filter struct {
	Server string
	Query  string
	From   *time.Time
	To     *time.Time
	Cursor string
	Limit  int
}

type Page struct {
	Sightings  []Sighting `json:"sightings"`
	NextCursor string     `json:"next_cursor,omitempty"`
	Total      int        `json:"total"`
}

type Cursor struct {
	ReceivedAt time.Time
	SightingID string
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Insert(ctx context.Context, sighting Sighting) error {
	if s == nil || s.pool == nil {
		return errors.New("thief sighting store is unavailable")
	}
	var region any
	var x any
	var y any
	var z any
	if sighting.Position != nil {
		region = sighting.Position.Region
		x = sighting.Position.X
		y = sighting.Position.Y
		if sighting.Position.Z != nil {
			z = *sighting.Position.Z
		}
	}
	var eventID any
	if sighting.EventID != "" {
		eventID = sighting.EventID
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO thief_sightings (
    sighting_id, server_name, thief_name, region, x, y, z, position_source,
    reporter_name, reporter_app, reporter_version, origin, event_id, observed_at, received_at
) VALUES (
    $1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::uuid, $14, $15
)`, sighting.ID, sighting.Server, sighting.ThiefName, region, x, y, z, sighting.PositionSource,
		sighting.Reporter.Name, sighting.Reporter.App, sighting.Reporter.Version, sighting.Origin,
		eventID, sighting.ObservedAt.UTC(), sighting.ReceivedAt.UTC())
	if err != nil {
		return fmt.Errorf("insert thief sighting: %w", err)
	}
	return nil
}

func (s *Store) Recent(ctx context.Context, server string, since time.Time, limit int) ([]Sighting, bool, error) {
	if s == nil || s.pool == nil {
		return nil, false, errors.New("thief sighting store is unavailable")
	}
	if limit < 1 {
		limit = MaxRecentSightings
	}
	rows, err := s.pool.Query(ctx, `SELECT sighting_id::text, server_name, thief_name, region, x, y, z,
       position_source, reporter_name, reporter_app, reporter_version, origin,
       event_id::text, observed_at, received_at
FROM (
    SELECT DISTINCT ON (lower(thief_name))
           sighting_id, server_name, thief_name, region, x, y, z, position_source,
           reporter_name, reporter_app, reporter_version, origin, event_id, observed_at, received_at
    FROM thief_sightings
    WHERE lower(server_name) = lower($1) AND received_at >= $2
    ORDER BY lower(thief_name), received_at DESC, sighting_id DESC
) latest
ORDER BY received_at DESC, sighting_id DESC
LIMIT $3`, server, since.UTC(), limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("list recent thief sightings: %w", err)
	}
	defer rows.Close()
	sightings, err := scanSightings(rows)
	if err != nil {
		return nil, false, err
	}
	truncated := len(sightings) > limit
	if truncated {
		sightings = sightings[:limit]
	}
	return sightings, truncated, nil
}

func (s *Store) List(ctx context.Context, filter Filter) (Page, error) {
	if s == nil || s.pool == nil {
		return Page{}, errors.New("thief sighting store is unavailable")
	}
	if filter.Limit < 1 || filter.Limit > MaxPageSize {
		filter.Limit = 10
	}
	cursor, err := DecodeCursor(filter.Cursor)
	if err != nil {
		return Page{}, err
	}
	pattern := likeContains(strings.TrimSpace(filter.Query))
	var cursorTime any
	var cursorID any
	if filter.Cursor != "" {
		cursorTime = cursor.ReceivedAt.UTC()
		cursorID = cursor.SightingID
	}
	const where = `WHERE ($1 = '' OR lower(server_name) = lower($1))
  AND ($2 = '' OR thief_name ILIKE $2 ESCAPE '\' OR reporter_name ILIKE $2 ESCAPE '\')
  AND ($3::timestamptz IS NULL OR received_at >= $3)
  AND ($4::timestamptz IS NULL OR received_at < $4)`
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM thief_sightings `+where,
		filter.Server, pattern, filter.From, filter.To).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count thief sightings: %w", err)
	}
	rows, err := s.pool.Query(ctx, `SELECT sighting_id::text, server_name, thief_name, region, x, y, z,
       position_source, reporter_name, reporter_app, reporter_version, origin,
       event_id::text, observed_at, received_at
FROM thief_sightings
`+where+`
  AND ($5::timestamptz IS NULL OR (received_at, sighting_id) < ($5, $6::uuid))
ORDER BY received_at DESC, sighting_id DESC
LIMIT $7`, filter.Server, pattern, filter.From, filter.To, cursorTime, cursorID, filter.Limit+1)
	if err != nil {
		return Page{}, fmt.Errorf("list thief sightings: %w", err)
	}
	defer rows.Close()
	sightings, err := scanSightings(rows)
	if err != nil {
		return Page{}, err
	}
	page := Page{Sightings: sightings, Total: total}
	if len(sightings) > filter.Limit {
		page.Sightings = sightings[:filter.Limit]
		last := page.Sightings[len(page.Sightings)-1]
		page.NextCursor = EncodeCursor(Cursor{ReceivedAt: last.ReceivedAt, SightingID: last.ID})
	}
	if page.Sightings == nil {
		page.Sightings = []Sighting{}
	}
	return page, nil
}

func (s *Store) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	if days < 1 {
		return 0, errors.New("retention days must be positive")
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM thief_sightings WHERE received_at < now() - make_interval(days => $1)`, days)
	if err != nil {
		return 0, fmt.Errorf("delete old thief sightings: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (s *Store) RunRetention(ctx context.Context, days int, every time.Duration) {
	if every <= 0 {
		every = time.Hour
	}
	run := func() {
		deleteCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := s.DeleteOlderThan(deleteCtx, days); err != nil && ctx.Err() == nil {
			slog.Warn("thief sighting retention failed", "reason", err.Error())
		}
	}
	run()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

type rowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanSightings(rows rowScanner) ([]Sighting, error) {
	var sightings []Sighting
	for rows.Next() {
		var sighting Sighting
		var region *int
		var x *float64
		var y *float64
		var z *float64
		var eventID *string
		if err := rows.Scan(
			&sighting.ID, &sighting.Server, &sighting.ThiefName, &region, &x, &y, &z,
			&sighting.PositionSource, &sighting.Reporter.Name, &sighting.Reporter.App, &sighting.Reporter.Version,
			&sighting.Origin, &eventID, &sighting.ObservedAt, &sighting.ReceivedAt,
		); err != nil {
			return nil, fmt.Errorf("scan thief sighting: %w", err)
		}
		if region != nil && x != nil && y != nil {
			sighting.Position = &Position{Region: *region, X: *x, Y: *y, Z: z}
		}
		if eventID != nil {
			sighting.EventID = *eventID
		}
		sightings = append(sightings, sighting)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thief sightings: %w", err)
	}
	return sightings, nil
}

func likeContains(value string) string {
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(value) + "%"
}

func EncodeCursor(cursor Cursor) string {
	return cursor.ReceivedAt.UTC().Format(time.RFC3339Nano) + "|" + cursor.SightingID
}

func DecodeCursor(value string) (Cursor, error) {
	if value == "" {
		return Cursor{}, nil
	}
	if len(value) > 256 {
		return Cursor{}, errors.New("cursor too long")
	}
	stamp, id, ok := strings.Cut(value, "|")
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if !ok || err != nil || len(id) != 36 {
		return Cursor{}, errors.New("invalid cursor")
	}
	return Cursor{ReceivedAt: parsed.UTC(), SightingID: id}, nil
}

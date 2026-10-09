package tradenexus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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

func (s *Store) Latest(ctx context.Context, server, thief string) (Sighting, bool, error) {
	if s == nil || s.pool == nil {
		return Sighting{}, false, errors.New("thief sighting store is unavailable")
	}
	rows, err := s.pool.Query(ctx, `SELECT sighting_id::text, server_name, thief_name, region, x, y, z,
       position_source, reporter_name, reporter_app, reporter_version, origin,
       event_id::text, observed_at, received_at
FROM thief_sightings
WHERE lower(server_name) = lower($1) AND lower(thief_name) = lower($2)
ORDER BY received_at DESC, sighting_id DESC
LIMIT 1`, server, thief)
	if err != nil {
		return Sighting{}, false, fmt.Errorf("load latest thief sighting: %w", err)
	}
	defer rows.Close()
	sightings, err := scanSightings(rows)
	if err != nil {
		return Sighting{}, false, err
	}
	if len(sightings) == 0 {
		return Sighting{}, false, nil
	}
	return sightings[0], true, nil
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
	gapSeconds := int(EncounterGap / time.Second)
	const encounters = `WITH filtered AS (
    SELECT sighting_id, server_name, thief_name, region, x, y, z, position_source,
           reporter_name, reporter_app, reporter_version, origin, event_id, observed_at, received_at
    FROM thief_sightings
    WHERE ($1 = '' OR lower(server_name) = lower($1))
      AND ($2 = '' OR thief_name ILIKE $2 ESCAPE '\' OR reporter_name ILIKE $2 ESCAPE '\')
      AND ($3::timestamptz IS NULL OR received_at >= $3)
      AND ($4::timestamptz IS NULL OR received_at < $4)
),
ordered AS (
    SELECT filtered.*,
           lag(received_at) OVER encounter AS prev_received_at
    FROM filtered
    WINDOW encounter AS (
        PARTITION BY lower(server_name), lower(thief_name)
        ORDER BY received_at ASC, sighting_id ASC
    )
),
marked AS (
    SELECT *,
           CASE
               WHEN prev_received_at IS NULL
                 OR received_at > prev_received_at + ($5::int * interval '1 second')
               THEN 1 ELSE 0
           END AS encounter_boundary
    FROM ordered
),
numbered AS (
    SELECT *,
           sum(encounter_boundary) OVER (
               PARTITION BY lower(server_name), lower(thief_name)
               ORDER BY received_at ASC, sighting_id ASC
           ) AS encounter_no
    FROM marked
),
encounters AS (
    SELECT DISTINCT ON (lower(server_name), lower(thief_name), encounter_no)
           sighting_id, server_name, thief_name, region, x, y, z, position_source,
           reporter_name, reporter_app, reporter_version, origin, event_id, observed_at, received_at
    FROM numbered
    ORDER BY lower(server_name), lower(thief_name), encounter_no, received_at DESC, sighting_id DESC
)
`
	var total int
	if err := s.pool.QueryRow(ctx, encounters+`SELECT count(*) FROM encounters`,
		filter.Server, pattern, filter.From, filter.To, gapSeconds).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count thief sightings: %w", err)
	}
	rows, err := s.pool.Query(ctx, encounters+`SELECT sighting_id::text, server_name, thief_name, region, x, y, z,
       position_source, reporter_name, reporter_app, reporter_version, origin,
       event_id::text, observed_at, received_at
FROM encounters
WHERE ($6::timestamptz IS NULL OR (received_at, sighting_id) < ($6, $7::uuid))
ORDER BY received_at DESC, sighting_id DESC
LIMIT $8`, filter.Server, pattern, filter.From, filter.To, gapSeconds, cursorTime, cursorID, filter.Limit+1)
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

func (s *Store) InsertTrade(ctx context.Context, report TradeReport) (TradeReport, error) {
	if s == nil || s.pool == nil {
		return TradeReport{}, errors.New("trade report store is unavailable")
	}
	waypoints := report.Waypoints
	if waypoints == nil {
		waypoints = []Waypoint{}
	}
	encodedWaypoints, err := json.Marshal(waypoints)
	if err != nil {
		return TradeReport{}, fmt.Errorf("encode trade waypoints: %w", err)
	}
	var goods any
	if report.Goods != nil {
		encodedGoods, err := json.Marshal(report.Goods)
		if err != nil {
			return TradeReport{}, fmt.Errorf("encode trade goods: %w", err)
		}
		goods = encodedGoods
	}
	var gold any
	if report.Gold != nil {
		gold = *report.Gold
	}
	var duration any
	if report.DurationS != nil {
		duration = *report.DurationS
	}
	var id string
	err = s.pool.QueryRow(ctx, `INSERT INTO trade_reports (
    trade_id, server_name, client_ref, outcome, reason, route_from, route_to,
    waypoints, goods, gold, duration_s, stars, detail, thief_name, transport,
    reporter_name, reporter_app, reporter_version, finished_at, received_at
) VALUES (
    $1::uuid, $2, $3, $4, $5, $6, $7,
    $8::jsonb, $9::jsonb, $10, $11, $12, $13, $14, $15,
    $16, $17, $18, $19, $20
)
ON CONFLICT (server_key, client_ref) DO NOTHING
RETURNING trade_id::text`,
		report.ID, report.Server, report.Ref, report.Outcome, report.Reason, report.Route.From, report.Route.To,
		encodedWaypoints, goods, gold, duration, nullableText(report.Stars), nullableText(report.Detail), nullableTradeName(report.Thief), nullableText(report.Transport),
		report.Reporter.Name, report.Reporter.App, report.Reporter.Version, report.FinishedAt.UTC(), report.ReceivedAt.UTC(),
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `SELECT trade_id::text FROM trade_reports WHERE server_key = lower($1) AND client_ref = $2`,
			report.Server, report.Ref).Scan(&id)
	}
	if err != nil {
		return TradeReport{}, fmt.Errorf("insert trade report: %w", err)
	}
	report.ID = id
	return report, nil
}

func (s *Store) ListTrades(ctx context.Context, filter Filter) (TradePage, error) {
	if s == nil || s.pool == nil {
		return TradePage{}, errors.New("trade report store is unavailable")
	}
	if filter.Limit < 1 || filter.Limit > MaxPageSize {
		filter.Limit = 10
	}
	cursor, err := DecodeCursor(filter.Cursor)
	if err != nil {
		return TradePage{}, err
	}
	pattern := likeContains(strings.TrimSpace(filter.Query))
	var cursorTime any
	var cursorID any
	if filter.Cursor != "" {
		cursorTime = cursor.ReceivedAt.UTC()
		cursorID = cursor.SightingID
	}
	const where = `WHERE ($1 = '' OR lower(server_name) = lower($1))
  AND ($2 = '' OR reporter_name ILIKE $2 ESCAPE '\' OR transport ILIKE $2 ESCAPE '\'
    OR thief_name ILIKE $2 ESCAPE '\' OR route_from ILIKE $2 ESCAPE '\'
    OR route_to ILIKE $2 ESCAPE '\' OR outcome ILIKE $2 ESCAPE '\'
    OR reason ILIKE $2 ESCAPE '\')
  AND ($3::timestamptz IS NULL OR finished_at >= $3)
  AND ($4::timestamptz IS NULL OR finished_at < $4)`
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM trade_reports `+where,
		filter.Server, pattern, filter.From, filter.To).Scan(&total); err != nil {
		return TradePage{}, fmt.Errorf("count trade reports: %w", err)
	}
	rows, err := s.pool.Query(ctx, `SELECT trade_id::text, server_name, client_ref, outcome, reason,
       route_from, route_to, waypoints, goods, gold, duration_s, stars, detail, thief_name, transport,
       reporter_name, reporter_app, reporter_version, finished_at, received_at
FROM trade_reports
`+where+`
  AND ($5::timestamptz IS NULL OR (finished_at, trade_id) < ($5, $6::uuid))
ORDER BY finished_at DESC, trade_id DESC
LIMIT $7`, filter.Server, pattern, filter.From, filter.To, cursorTime, cursorID, filter.Limit+1)
	if err != nil {
		return TradePage{}, fmt.Errorf("list trade reports: %w", err)
	}
	defer rows.Close()
	reports, err := scanTradeReports(rows)
	if err != nil {
		return TradePage{}, err
	}
	page := TradePage{Reports: reports, Total: total}
	if len(reports) > filter.Limit {
		page.Reports = reports[:filter.Limit]
		last := page.Reports[len(page.Reports)-1]
		page.NextCursor = EncodeCursor(Cursor{ReceivedAt: last.FinishedAt, SightingID: last.ID})
	}
	if page.Reports == nil {
		page.Reports = []TradeReport{}
	}
	return page, nil
}

func (s *Store) DeleteTradesOlderThan(ctx context.Context, days int) (int64, error) {
	if days < 1 {
		return 0, errors.New("retention days must be positive")
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM trade_reports WHERE received_at < now() - make_interval(days => $1)`, days)
	if err != nil {
		return 0, fmt.Errorf("delete old trade reports: %w", err)
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
		if _, err := s.DeleteTradesOlderThan(deleteCtx, days); err != nil && ctx.Err() == nil {
			slog.Warn("trade report retention failed", "reason", err.Error())
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

func scanTradeReports(rows rowScanner) ([]TradeReport, error) {
	var reports []TradeReport
	for rows.Next() {
		var report TradeReport
		var waypoints []byte
		var goods []byte
		var gold *int32
		var duration *int
		var stars *string
		var detail *string
		var thiefName *string
		var transport *string
		if err := rows.Scan(
			&report.ID, &report.Server, &report.Ref, &report.Outcome, &report.Reason,
			&report.Route.From, &report.Route.To, &waypoints, &goods, &gold, &duration, &stars, &detail, &thiefName, &transport,
			&report.Reporter.Name, &report.Reporter.App, &report.Reporter.Version, &report.FinishedAt, &report.ReceivedAt,
		); err != nil {
			return nil, fmt.Errorf("scan trade report: %w", err)
		}
		if err := json.Unmarshal(waypoints, &report.Waypoints); err != nil {
			return nil, fmt.Errorf("decode trade waypoints: %w", err)
		}
		if report.Waypoints == nil {
			report.Waypoints = []Waypoint{}
		}
		if goods != nil {
			decoded := []TradeGood{}
			if err := json.Unmarshal(goods, &decoded); err != nil {
				return nil, fmt.Errorf("decode trade goods: %w", err)
			}
			report.Goods = &decoded
		}
		report.Gold = gold
		report.DurationS = duration
		if stars != nil {
			report.Stars = *stars
		}
		if detail != nil {
			report.Detail = *detail
		}
		if thiefName != nil {
			report.Thief = &tradeName{Name: *thiefName}
		}
		if transport != nil {
			report.Transport = *transport
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trade reports: %w", err)
	}
	return reports, nil
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTradeName(thief *tradeName) any {
	if thief == nil || thief.Name == "" {
		return nil
	}
	return thief.Name
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

package players

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Filter struct {
	Server    string     `json:"server,omitempty"`
	Name      string     `json:"q,omitempty"`
	Guild     string     `json:"guild,omitempty"`
	MinLevel  *int       `json:"min_level,omitempty"`
	MaxLevel  *int       `json:"max_level,omitempty"`
	Job       string     `json:"job,omitempty"`
	Seen      string     `json:"seen,omitempty"`
	From      *time.Time `json:"from,omitempty"`
	To        *time.Time `json:"to,omitempty"`
	Identity  string     `json:"identity,omitempty"`
	Equipment string     `json:"equipment,omitempty"`
	Sort      string     `json:"sort"`
	Direction string     `json:"direction"`
	Limit     int        `json:"limit"`
	Cursor    string     `json:"-"`
}
type Page struct {
	Players        []Record        `json:"players"`
	Total          int             `json:"total"`
	NextCursor     string          `json:"next_cursor,omitempty"`
	PreviousCursor string          `json:"previous_cursor,omitempty"`
	Servers        []string        `json:"servers"`
	Ingestion      IngestionStatus `json:"ingestion"`
}
type HistoryPage[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}
type cursor struct {
	Context  string    `json:"context"`
	Key      string    `json:"key"`
	ID       string    `json:"id"`
	At       time.Time `json:"at"`
	Previous bool      `json:"previous,omitempty"`
}

func contextHash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:16])
}
func encodeCursor(c cursor) string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}
func decodeCursor(value, context string) (cursor, error) {
	var c cursor
	if value == "" {
		return c, nil
	}
	if len(value) > 1024 {
		return c, ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(raw, &c) != nil || c.Context != context || !ValidID(c.ID) || c.At.IsZero() || c.At.After(time.Now().Add(LiveFutureSkew)) {
		return c, ErrInvalid
	}
	return c, nil
}

func decodeHistoryCursor(value, context string) (cursor, error) {
	c, err := decodeCursor(value, context)
	if err != nil || value == "" {
		return c, err
	}
	if c.Previous {
		return c, ErrInvalid
	}
	if _, err = time.Parse(time.RFC3339Nano, c.Key); err != nil {
		return c, ErrInvalid
	}
	return c, nil
}

func (f *Filter) Validate() error {
	if f.Sort == "" {
		f.Sort = "last_seen"
	}
	if f.Direction == "" {
		f.Direction = "desc"
	}
	if f.Limit == 0 {
		f.Limit = 25
	}
	if !validText(f.Server, 100) || !validText(f.Name, 64) || !validText(f.Guild, 64) || f.Limit < 1 || f.Limit > 100 || f.Direction != "asc" && f.Direction != "desc" {
		return ErrInvalid
	}
	if f.Sort != "last_seen" && f.Sort != "name" && f.Sort != "level" && f.Sort != "guild" && f.Sort != "job" {
		return ErrInvalid
	}
	if f.Job != "" && !validJob(f.Job) || f.MinLevel != nil && (*f.MinLevel < 1 || *f.MinLevel > 255) || f.MaxLevel != nil && (*f.MaxLevel < 1 || *f.MaxLevel > 255) || f.MinLevel != nil && f.MaxLevel != nil && *f.MinLevel > *f.MaxLevel {
		return ErrInvalid
	}
	if f.Identity != "" && f.Identity != "resolved" && f.Identity != "unresolved" || f.Equipment != "" && f.Equipment != "complete" && f.Equipment != "partial" && f.Equipment != "unavailable" {
		return ErrInvalid
	}
	if f.Seen != "" && f.Seen != "1h" && f.Seen != "24h" && f.Seen != "7d" && f.Seen != "30d" && f.Seen != "custom" {
		return ErrInvalid
	}
	if f.From != nil && f.To != nil && !f.To.After(*f.From) {
		return ErrInvalid
	}
	c, err := decodeCursor(f.Cursor, contextHash(f))
	if err != nil {
		return err
	}
	if f.Cursor != "" {
		switch f.Sort {
		case "last_seen":
			_, err = time.Parse(time.RFC3339Nano, c.Key)
		case "level":
			_, err = strconv.Atoi(c.Key)
		}
		if err != nil {
			return ErrInvalid
		}
	}
	return nil
}
func likePattern(value string) string {
	return "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value) + "%"
}

const recordColumns = `v.id::text,v.server_key,v.server_name,v.name,COALESCE(v.observed_name,''),v.level,v.guild_name,v.job,v.job_name,v.gear,v.gear_hash,v.identity_gear_hash,v.location,v.first_seen_at,v.last_seen_at,v.resolved,v.revision,v.source_ids::text[]`

type scanner interface{ Scan(...any) error }

func scanRecord(row scanner) (Record, error) {
	var r Record
	var gear, location []byte
	err := row.Scan(&r.ID, &r.ServerKey, &r.Server, &r.Name, &r.ObservedName, &r.Level, &r.Guild, &r.Job, &r.JobName, &gear, &r.GearHash, &r.IdentityGearHash, &location, &r.FirstSeen, &r.LastSeen, &r.Resolved, &r.Revision, &r.SourceIDs)
	if err != nil {
		return r, err
	}
	if len(gear) > 0 {
		err = decodeJSON(gear, &r.Gear)
		if err != nil {
			return r, err
		}
	}
	if len(location) > 0 {
		err = decodeJSON(location, &r.Location)
	}
	return r, err
}

func (s *Store) List(ctx context.Context, f Filter) (Page, error) {
	page := Page{Players: []Record{}, Servers: []string{}, Ingestion: s.Status()}
	if err := f.Validate(); err != nil {
		return page, err
	}
	c, _ := decodeCursor(f.Cursor, contextHash(f))
	asOf := time.Now().UTC()
	if f.Cursor != "" {
		asOf = c.At
	}
	from := f.From
	if duration := map[string]time.Duration{"1h": time.Hour, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}[f.Seen]; duration > 0 {
		value := asOf.Add(-duration)
		from = &value
	}
	args := []any{}
	bind := func(value any) string { args = append(args, value); return fmt.Sprintf("$%d", len(args)) }
	where := []string{"true"}
	if f.Server != "" {
		where = append(where, "v.server_key="+bind(ServerKey(f.Server)))
	}
	if f.Name != "" {
		where = append(where, `EXISTS (SELECT 1 FROM player_aliases a WHERE a.player_id=ANY(v.source_ids) AND a.alias_name ILIKE `+bind(likePattern(f.Name))+` ESCAPE '\')`)
	}
	if f.Guild != "" {
		where = append(where, `v.guild_name ILIKE `+bind(likePattern(f.Guild))+` ESCAPE '\'`)
	}
	if f.MinLevel != nil {
		where = append(where, "v.level>="+bind(*f.MinLevel))
	}
	if f.MaxLevel != nil {
		where = append(where, "v.level<="+bind(*f.MaxLevel))
	}
	if f.Job != "" {
		where = append(where, "COALESCE(v.job,'unknown')="+bind(f.Job))
	}
	if from != nil {
		where = append(where, "v.last_seen_at>="+bind(*from))
	}
	if f.To != nil {
		where = append(where, "v.last_seen_at<"+bind(*f.To))
	}
	if f.Identity == "resolved" {
		where = append(where, "v.resolved")
	}
	if f.Identity == "unresolved" {
		where = append(where, "NOT v.resolved")
	}
	if f.Equipment != "" {
		availability := map[string]string{"complete": "observed_complete", "partial": "observed_partial", "unavailable": "unavailable"}[f.Equipment]
		where = append(where, "COALESCE(v.gear->>'last_availability',v.gear->>'availability','unavailable')="+bind(availability))
	}
	base := " FROM player_registry v WHERE " + strings.Join(where, " AND ")
	if err := s.pool.QueryRow(ctx, "SELECT count(*)"+base, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	expr, cast := map[string]string{"last_seen": "v.last_seen_at", "name": "lower(COALESCE(v.name,v.observed_name,''))", "level": "COALESCE(v.level,0)", "guild": "lower(COALESCE(v.guild_name,''))", "job": "COALESCE(v.job,'unknown')"}[f.Sort], "text"
	if f.Sort == "last_seen" {
		cast = "timestamptz"
	}
	if f.Sort == "level" {
		cast = "integer"
	}
	direction, comparison := "DESC", "<"
	if f.Direction == "asc" {
		direction = "ASC"
		comparison = ">"
	}
	if c.Previous {
		if direction == "ASC" {
			direction, comparison = "DESC", "<"
		} else {
			direction, comparison = "ASC", ">"
		}
	}
	if f.Cursor != "" {
		base += " AND (" + expr + ",v.id) " + comparison + " (" + bind(c.Key) + "::" + cast + "," + bind(c.ID) + "::uuid)"
	}
	query := "SELECT " + recordColumns + base + " ORDER BY " + expr + " " + direction + ",v.id " + direction + " LIMIT " + bind(f.Limit+1)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return page, err
	}
	for rows.Next() {
		r, e := scanRecord(rows)
		if e != nil {
			rows.Close()
			return page, e
		}
		page.Players = append(page.Players, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return page, err
	}
	hasMore := len(page.Players) > f.Limit
	if hasMore {
		page.Players = page.Players[:f.Limit]
	}
	if c.Previous {
		for i, j := 0, len(page.Players)-1; i < j; i, j = i+1, j-1 {
			page.Players[i], page.Players[j] = page.Players[j], page.Players[i]
		}
	}
	if len(page.Players) > 0 {
		if (!c.Previous && hasMore) || (c.Previous && f.Cursor != "") {
			r := page.Players[len(page.Players)-1]
			page.NextCursor = encodeCursor(cursor{contextHash(f), registryCursorKey(r, f.Sort), r.ID, asOf, false})
		}
		if (c.Previous && hasMore) || (!c.Previous && f.Cursor != "") {
			r := page.Players[0]
			page.PreviousCursor = encodeCursor(cursor{contextHash(f), registryCursorKey(r, f.Sort), r.ID, asOf, true})
		}
	}
	page.Servers, err = s.Servers(ctx)
	return page, err
}

func registryCursorKey(r Record, sort string) string {
	switch sort {
	case "last_seen":
		return r.LastSeen.UTC().Format(time.RFC3339Nano)
	case "level":
		if r.Level != nil {
			return strconv.Itoa(*r.Level)
		}
		return "0"
	case "name":
		if r.Name != nil {
			return strings.ToLower(*r.Name)
		}
		return strings.ToLower(r.ObservedName)
	case "guild":
		if r.Guild != nil {
			return strings.ToLower(*r.Guild)
		}
		return ""
	case "job":
		if r.Job != nil {
			return *r.Job
		}
		return "unknown"
	}
	return ""
}

func (s *Store) Servers(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT min(server_name) FROM players GROUP BY server_key ORDER BY min(server_name)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var server string
		if err = rows.Scan(&server); err != nil {
			return nil, err
		}
		out = append(out, server)
	}
	return out, rows.Err()
}

func (s *Store) Get(ctx context.Context, id string) (Record, error) {
	if !ValidID(id) {
		return Record{}, ErrInvalid
	}
	r, err := scanRecord(s.pool.QueryRow(ctx, `SELECT `+recordColumns+` FROM player_registry v WHERE v.id=COALESCE((SELECT canonical_player_id FROM player_identity_links WHERE linked_player_id=$1::uuid AND status='confirmed'),$1::uuid)`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return r, err
}

func (s *Store) GetSource(ctx context.Context, id string) (Record, error) {
	if !ValidID(id) {
		return Record{}, ErrInvalid
	}
	r, err := scanRecord(s.pool.QueryRow(ctx, `SELECT `+recordColumns+` FROM (SELECT p.*, ARRAY[p.id] AS source_ids,
 (SELECT alias_name FROM player_aliases a WHERE a.player_id=p.id ORDER BY last_seen_at DESC,id LIMIT 1) AS observed_name,
 EXISTS(SELECT 1 FROM player_aliases a WHERE a.player_id=p.id AND a.alias_type='normal' AND a.confirmation_status='confirmed') AS resolved
 FROM players p WHERE p.id=$1::uuid) v`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return r, err
}

func (s *Store) Aliases(ctx context.Context, id string, limit int, token string) (HistoryPage[Alias], error) {
	page := HistoryPage[Alias]{Items: []Alias{}}
	ids, err := s.sourceIDs(ctx, id)
	if err != nil {
		return page, err
	}
	context := contextHash([]string{id, "aliases"})
	c, err := decodeHistoryCursor(token, context)
	if err != nil {
		return page, err
	}
	if limit < 1 || limit > 100 {
		return page, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,player_id::text,alias_name,alias_type,job_type,match_method,confirmation_status,first_seen_at,last_seen_at FROM player_aliases WHERE player_id=ANY($1::uuid[]) AND ($2='' OR (last_seen_at,id)<($3::timestamptz,$4::uuid)) ORDER BY last_seen_at DESC,id DESC LIMIT $5`, ids, token, nullableString(c.Key), nullableString(c.ID), limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Alias
		if err = rows.Scan(&a.ID, &a.PlayerID, &a.Name, &a.Type, &a.Job, &a.Method, &a.Status, &a.FirstSeen, &a.LastSeen); err != nil {
			return page, err
		}
		page.Items = append(page.Items, a)
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		a := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(cursor{context, a.LastSeen.UTC().Format(time.RFC3339Nano), a.ID, time.Now().UTC(), false})
	}
	return page, rows.Err()
}

func (s *Store) Observations(ctx context.Context, id string, limit int, token string) (HistoryPage[Observation], error) {
	page := HistoryPage[Observation]{Items: []Observation{}}
	ids, err := s.sourceIDs(ctx, id)
	if err != nil {
		return page, err
	}
	context := contextHash([]string{id, "observations"})
	c, err := decodeHistoryCursor(token, context)
	if err != nil {
		return page, err
	}
	if limit < 1 || limit > 100 {
		return page, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,observed_at,evidence FROM player_observations WHERE player_id=ANY($1::uuid[]) AND ($2='' OR (observed_at,id)<($3::timestamptz,$4::uuid)) ORDER BY observed_at DESC,id DESC LIMIT $5`, ids, token, nullableString(c.Key), nullableString(c.ID), limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var lastID string
	var lastObserved time.Time
	for rows.Next() {
		var raw []byte
		var o Observation
		var storedID string
		var storedObserved time.Time
		if err = rows.Scan(&storedID, &storedObserved, &raw); err != nil {
			return page, err
		}
		if err = decodeJSON(raw, &o); err != nil {
			return page, err
		}
		page.Items = append(page.Items, o)
		if len(page.Items) <= limit {
			lastID, lastObserved = storedID, storedObserved
		}
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		page.NextCursor = encodeCursor(cursor{context, lastObserved.UTC().Format(time.RFC3339Nano), lastID, time.Now().UTC(), false})
	}
	return page, rows.Err()
}

func (s *Store) EquipmentHistory(ctx context.Context, id string, limit int, token string) (HistoryPage[EquipmentHistory], error) {
	page := HistoryPage[EquipmentHistory]{Items: []EquipmentHistory{}}
	ids, err := s.sourceIDs(ctx, id)
	if err != nil {
		return page, err
	}
	context := contextHash([]string{id, "equipment"})
	c, err := decodeHistoryCursor(token, context)
	if err != nil {
		return page, err
	}
	if limit < 1 || limit > 100 {
		return page, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,player_id::text,observation_id::text,equipment_json,last_evidence,gear_hash,identity_gear_hash,first_seen_at,last_seen_at FROM player_equipment_history WHERE player_id=ANY($1::uuid[]) AND ($2='' OR (first_seen_at,id)<($3::timestamptz,$4::uuid)) ORDER BY first_seen_at DESC,id DESC LIMIT $5`, ids, token, nullableString(c.Key), nullableString(c.ID), limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var h EquipmentHistory
		var raw, endpoint []byte
		if err = rows.Scan(&h.ID, &h.PlayerID, &h.ObservationID, &raw, &endpoint, &h.GearHash, &h.IdentityGearHash, &h.FirstSeen, &h.LastSeen); err != nil {
			return page, err
		}
		if err = decodeJSON(raw, &h.Equipment); err != nil {
			return page, err
		}
		if err = decodeJSON(endpoint, &h.LastEvidence); err != nil {
			return page, err
		}
		page.Items = append(page.Items, h)
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		h := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(cursor{context, h.FirstSeen.UTC().Format(time.RFC3339Nano), h.ID, time.Now().UTC(), false})
	}
	return page, rows.Err()
}

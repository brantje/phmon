package players

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool    *pgxpool.Pool
	pending *pendingObservations
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool, pending: newPending()} }
func decodeJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(target)
}
func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// ApplyObservations commits a bounded batch. The server-scoped transaction lock
// serializes alias admission with review decisions without trusting runtime IDs.
func (s *Store) ApplyObservations(ctx context.Context, observations []Observation) error {
	observations = append([]Observation{}, observations...)
	if s == nil || s.pool == nil {
		return errors.New("player store unavailable")
	}
	if len(observations) > MaxPlayers {
		return ErrInvalid
	}
	keys := map[string]bool{}
	for i := range observations {
		if err := observations[i].normalize(); err != nil {
			return err
		}
		keys[ServerKey(observations[i].Server)] = true
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	servers := []string{}
	for key := range keys {
		servers = append(servers, key)
	}
	sort.Strings(servers)
	for _, server := range servers {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 78113))`, server); err != nil {
			return err
		}
	}
	for _, o := range observations {
		if err = s.apply(ctx, tx, o); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type storedPlayer struct {
	ID                  string
	Name                *string
	Level               *int
	Guild, Job, JobName *string
	Gear                *Equipment
	Model               *int64
	Location            *Location
	FieldTimes          map[string]time.Time
	FirstSeen, LastSeen time.Time
}

func loadStored(ctx context.Context, tx pgx.Tx, id string) (storedPlayer, error) {
	var p storedPlayer
	var gear, location, times []byte
	err := tx.QueryRow(ctx, `SELECT id::text,name,level,guild_name,job,job_name,gear,character_model_id,location,field_times,first_seen_at,last_seen_at FROM players WHERE id=$1::uuid FOR UPDATE`, id).Scan(&p.ID, &p.Name, &p.Level, &p.Guild, &p.Job, &p.JobName, &gear, &p.Model, &location, &times, &p.FirstSeen, &p.LastSeen)
	if err != nil {
		return p, err
	}
	p.FieldTimes = map[string]time.Time{}
	if err = decodeJSON(times, &p.FieldTimes); err != nil {
		return p, err
	}
	if len(gear) > 0 {
		if err = decodeJSON(gear, &p.Gear); err != nil {
			return p, err
		}
	}
	if len(location) > 0 {
		err = decodeJSON(location, &p.Location)
	}
	return p, err
}

func (s *Store) apply(ctx context.Context, tx pgx.Tx, o Observation) error {
	key := ServerKey(o.Server)
	signature := o.signature()
	var existing string
	err := tx.QueryRow(ctx, `SELECT signature FROM player_observations WHERE server_key=$1 AND source=$2 AND source_ref=$3`, key, o.Source, o.SourceRef).Scan(&existing)
	if err == nil {
		if existing != signature {
			return ErrConflict
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var id string
	// A provisional alias remains reusable only when it has a unique owner. Never
	// resolve an ambiguous name by selecting an arbitrary row.
	rows, err := tx.Query(ctx, `SELECT DISTINCT a.player_id::text FROM player_aliases a WHERE a.server_key=$1 AND lower(a.alias_name)=lower($2) AND ($3='unknown' OR a.alias_type=$3 OR (a.alias_type='unknown' AND NOT EXISTS(SELECT 1 FROM player_aliases classified WHERE classified.player_id=a.player_id AND lower(classified.alias_name)=lower(a.alias_name) AND classified.alias_type<>'unknown' AND classified.alias_type<>$3))) ORDER BY a.player_id::text LIMIT 2`, key, o.Name, o.NameType)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// Name equality does not override contradictory model or simultaneous-presence
	// evidence. Keep a distinct source record and an inspectable review candidate.
	conflictIDs := []string{}
	conflictReasons := []string{}
	if len(ids) == 0 && o.NameType != "unknown" && o.PlayerID == "" {
		contradictions, queryErr := tx.Query(ctx, `SELECT DISTINCT player_id::text FROM player_aliases WHERE server_key=$1 AND lower(alias_name)=lower($2) AND alias_type<>'unknown' AND alias_type<>$3 AND confirmation_status='confirmed' ORDER BY player_id::text LIMIT 2`, key, o.Name, o.NameType)
		if queryErr != nil {
			return queryErr
		}
		for contradictions.Next() {
			var other string
			if queryErr = contradictions.Scan(&other); queryErr != nil {
				contradictions.Close()
				return queryErr
			}
			conflictIDs = append(conflictIDs, other)
		}
		queryErr = contradictions.Err()
		contradictions.Close()
		if queryErr != nil {
			return queryErr
		}
		if len(conflictIDs) > 0 {
			o.AliasConflict = true
			conflictReasons = append(conflictReasons, "classification_conflict")
		}
	}
	if len(ids) == 1 && !o.AliasConflict {
		var model *int64
		var simultaneous bool
		err = tx.QueryRow(ctx, `SELECT character_model_id FROM players WHERE id=$1::uuid`, ids[0]).Scan(&model)
		if err != nil {
			return err
		}
		if model != nil && o.Model != nil && *model != *o.Model {
			o.AliasConflict = true
			conflictReasons = append(conflictReasons, "model_conflict")
		}
		if o.SessionID != "" && o.RuntimeID != "" {
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_observations WHERE player_id=$1::uuid AND observer_session_id=$2 AND runtime_epoch=$3 AND observed_at=$4 AND runtime_entity_id<>$5)`, ids[0], o.SessionID, o.Epoch, o.ObservedAt, o.RuntimeID).Scan(&simultaneous)
			if err != nil {
				return err
			}
			if simultaneous {
				o.AliasConflict = true
				conflictReasons = append(conflictReasons, "simultaneous_distinct_players")
			}
		}
		if !o.AliasConflict {
			id = ids[0]
		}
	}
	if len(ids) > 1 || o.AliasConflict {
		conflictIDs = append(conflictIDs, ids...)
		if len(conflictReasons) == 0 {
			conflictReasons = append(conflictReasons, "ambiguous_alias")
		}
	}

	if o.PlayerID != "" {
		if !ValidID(o.PlayerID) {
			return ErrInvalid
		}
		var targetServer string
		if err = tx.QueryRow(ctx, `SELECT server_key FROM players WHERE id=$1::uuid`, o.PlayerID).Scan(&targetServer); err != nil {
			return err
		}
		if targetServer != key {
			return ErrConflict
		}
		id = o.PlayerID
	}
	if (len(ids) > 1 || o.AliasConflict) && o.PlayerID == "" {
		// An exact source incarnation may continue its own ambiguous-name history.
		if o.SessionID != "" && o.RuntimeID != "" {
			err = tx.QueryRow(ctx, `SELECT player_id::text FROM player_observations WHERE server_key=$1 AND observer_session_id=$2 AND runtime_epoch=$3 AND runtime_entity_id=$4 AND lower(observed_name)=lower($5) AND last_seen_at>=$6 AND ($7::bigint IS NULL OR character_model_id IS NULL OR character_model_id=$7) ORDER BY observed_at DESC LIMIT 1`, key, o.SessionID, o.Epoch, o.RuntimeID, o.Name, o.ObservedAt.Add(-LiveTTL), o.Model).Scan(&id)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
	}
	if id == "" {
		id = newID()
		_, err = tx.Exec(ctx, `INSERT INTO players(id,server_key,server_name,first_seen_at,last_seen_at) VALUES($1::uuid,$2,$3,$4,$5)`, id, key, o.Server, o.ObservedAt, o.LastSeen)
		if err != nil {
			return err
		}
	}
	p, err := loadStored(ctx, tx, id)
	if err != nil {
		return err
	}
	// Independent observer checkpoints are retained, but identical 1 s snapshots
	// do not produce a durable row or rewrite a player every second.
	if o.Source == "map.players" {
		var previousSignature string
		var previousTime time.Time
		err = tx.QueryRow(ctx, `SELECT signature,last_seen_at FROM player_observations WHERE player_id=$1::uuid AND source=$2 AND observer_session_id=$3 AND runtime_epoch=$4 AND runtime_entity_id=$5 ORDER BY observed_at DESC LIMIT 1`, id, o.Source, o.SessionID, o.Epoch, o.RuntimeID).Scan(&previousSignature, &previousTime)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && signature == previousSignature && o.LastSeen.Before(previousTime.Add(CheckpointInterval)) && !o.ObservedAt.Before(p.FirstSeen) {
			return nil
		}
	}
	status, method := "provisional", "observed_name"
	if o.AliasConflict {
		status = "conflict"
		if _, err = tx.Exec(ctx, `UPDATE player_aliases SET confirmation_status='conflict' WHERE server_key=$1 AND lower(alias_name)=lower($2) AND alias_type='unknown'`, key, o.Name); err != nil {
			return err
		}
	}
	if o.NameType != "unknown" && !o.AliasConflict {
		status = "confirmed"
		method = o.Source
	}
	var aliasID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM player_aliases WHERE player_id=$1::uuid AND lower(alias_name)=lower($2) AND alias_type=$3 LIMIT 1`, id, o.Name, o.NameType).Scan(&aliasID)
	if errors.Is(err, pgx.ErrNoRows) {
		aliasID = newID()
		_, err = tx.Exec(ctx, `INSERT INTO player_aliases(id,player_id,server_key,alias_name,alias_type,job_type,match_method,confirmation_status,first_seen_at,last_seen_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10)`, aliasID, id, key, o.Name, o.NameType, o.Job, method, status, o.ObservedAt, o.LastSeen)
	} else if err == nil {
		_, err = tx.Exec(ctx, `UPDATE player_aliases SET first_seen_at=least(first_seen_at,$2),last_seen_at=greatest(last_seen_at,$3) WHERE id=$1::uuid`, aliasID, o.ObservedAt, o.LastSeen)
	}
	if err != nil {
		return err
	}
	if o.Source == "operator.classification" {
		_, err = tx.Exec(ctx, `UPDATE player_aliases SET confirmation_status='confirmed',match_method=$2,job_type=$3 WHERE id=$1::uuid`, aliasID, o.Source, o.Job)
		if err != nil {
			return err
		}
	}
	fresh := func(field string) bool { return !o.ObservedAt.Before(p.FieldTimes[field]) }
	setTime := func(field string) { p.FieldTimes[field] = o.ObservedAt }
	if o.NameType == "normal" && !o.AliasConflict && fresh("name") {
		name := o.Name
		p.Name = &name
		setTime("name")
	}
	if o.NameType == "job" && fresh("job_name") {
		name := o.Name
		p.JobName = &name
		setTime("job_name")
	}
	if o.Level != nil && fresh("level") {
		p.Level = o.Level
		setTime("level")
	}
	if o.Guild != nil && fresh("guild") {
		p.Guild = o.Guild
		setTime("guild")
	}
	if o.Job != nil && *o.Job != "unknown" && fresh("job") {
		p.Job = o.Job
		setTime("job")
	}
	if o.Model != nil && fresh("model") {
		p.Model = o.Model
		setTime("model")
	}
	if o.Location != nil && !o.Location.ObservedAt.Before(p.FieldTimes["location"]) {
		p.Location = o.Location
		p.FieldTimes["location"] = o.Location.ObservedAt
	}
	if o.Equipment != nil {
		p.Gear = mergeEquipment(p.Gear, o.Equipment)
		if fresh("gear") {
			setTime("gear")
		}
	}
	if o.ObservedAt.Before(p.FirstSeen) {
		p.FirstSeen = o.ObservedAt
	}
	if o.LastSeen.After(p.LastSeen) {
		p.LastSeen = o.LastSeen
	}
	gear, _ := json.Marshal(p.Gear)
	location, _ := json.Marshal(p.Location)
	times, _ := json.Marshal(p.FieldTimes)
	_, err = tx.Exec(ctx, `UPDATE players SET name=$2,level=$3,guild_name=$4,job=$5,job_name=$6,gear=NULLIF($7::jsonb,'null'::jsonb),gear_hash=$8,identity_gear_hash=$9,field_times=$10,location=NULLIF($11::jsonb,'null'::jsonb),character_model_id=$12,first_seen_at=$13,last_seen_at=$14,updated_at=now(),revision=revision+1 WHERE id=$1::uuid`, id, p.Name, p.Level, p.Guild, p.Job, p.JobName, gear, nullableString(gearHash(p.Gear)), nullableString(identityHash(p.Gear)), times, location, p.Model, p.FirstSeen, p.LastSeen)
	if err != nil {
		return err
	}
	// The canonical projection's revision also changes when a linked source's
	// facts change, so review submissions cannot silently use an older projection.
	_, err = tx.Exec(ctx, `UPDATE players SET revision=revision+1 WHERE id=(SELECT canonical_player_id FROM player_identity_links WHERE linked_player_id=$1::uuid AND status='confirmed')`, id)
	if err != nil {
		return err
	}
	o.PlayerID = id
	raw, err := json.Marshal(o)
	if err != nil {
		return err
	}
	equipment, _ := json.Marshal(o.Equipment)
	var region, x, y, z any
	if o.Location != nil {
		region = o.Location.Region
		x = o.Location.X
		y = o.Location.Y
		z = o.Location.Z
	}
	_, err = tx.Exec(ctx, `INSERT INTO player_observations(id,player_id,server_key,observed_name,observer_agent_id,observer_character_id,observer_session_id,runtime_entity_id,runtime_epoch,character_model_id,level,guild_name,job_type,region,x,y,z,equipment_json,gear_hash,identity_gear_hash,source,source_ref,signature,observed_at,last_seen_at,received_at,evidence,pinned) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,NULLIF($18::jsonb,'null'::jsonb),$19,$20,$21,$22,$23,$24,$25,$26,$27,$28)`, o.ID, id, key, o.Name, nullableString(o.AgentID), nullableString(o.CharacterID), nullableString(o.SessionID), nullableString(o.RuntimeID), o.Epoch, o.Model, o.Level, o.Guild, o.Job, region, x, y, z, equipment, nullableString(gearHash(o.Equipment)), nullableString(identityHash(o.Equipment)), o.Source, o.SourceRef, signature, o.ObservedAt, o.LastSeen, o.ReceivedAt, raw, o.Pinned)
	if err != nil {
		return err
	}
	for _, other := range conflictIDs {
		if other != id {
			if err = s.recordAliasConflict(ctx, tx, other, id, o, conflictReasons); err != nil {
				return err
			}
		}
	}
	if o.Equipment != nil && o.Equipment.Availability != "unavailable" {
		if err = s.recordEquipment(ctx, tx, id, o, p.Gear); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) recordEquipment(ctx context.Context, tx pgx.Tx, id string, o Observation, current *Equipment) error {
	var historyID, previousHash, previousObservation string
	var first, last time.Time
	var previousJSON, endpointJSON []byte
	err := tx.QueryRow(ctx, `SELECT id::text,gear_hash,observation_id::text,first_seen_at,last_seen_at,equipment_json,last_evidence FROM player_equipment_history WHERE player_id=$1::uuid AND first_seen_at<=$2 ORDER BY first_seen_at DESC,id DESC LIMIT 1 FOR UPDATE`, id, o.ObservedAt).Scan(&historyID, &previousHash, &previousObservation, &first, &last, &previousJSON, &endpointJSON)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	found := err == nil
	equipment := current
	var previous *Equipment
	// Reconstruct a delayed snapshot from knowledge at its preceding interval,
	// never from a newer projection. Partial snapshots retain only earlier facts.
	if current.ObservedAt.After(o.Equipment.ObservedAt) {
		if found {
			if err = decodeJSON(previousJSON, &previous); err != nil {
				return err
			}
		}
		equipment = mergeEquipment(previous, o.Equipment)
	}
	hash := gearHash(equipment)
	endpoint, _ := json.Marshal(o)
	if found && previousHash == hash {
		// Do not extend an older interval across a later, different configuration.
		_, err = tx.Exec(ctx, `UPDATE player_equipment_history SET last_evidence=CASE WHEN last_seen_at<=$2 THEN $5::jsonb ELSE last_evidence END,last_seen_at=greatest(last_seen_at,$2) WHERE id=$1::uuid AND NOT EXISTS(SELECT 1 FROM player_equipment_history h WHERE h.player_id=$3::uuid AND h.first_seen_at>$4 AND h.first_seen_at<=$2)`, historyID, o.LastSeen, id, last, endpoint)
		return err
	}
	// An observation arriving inside an extended A interval can reveal B and a
	// later return to A. Retain the endpoint's copied source evidence even after
	// routine sightings expire; do not stretch either interval across the change.
	var resume *Observation
	if found && last.After(o.ObservedAt) {
		var end Observation
		if err = decodeJSON(endpointJSON, &end); err != nil {
			return err
		}
		if end.Equipment != nil && end.ObservedAt.After(o.ObservedAt) {
			resume = &end
			// Prefer the earliest retained observation of the return configuration.
			var raw []byte
			err = tx.QueryRow(ctx, `SELECT evidence FROM player_observations WHERE player_id=$1::uuid AND gear_hash=$2 AND observed_at>$3 AND observed_at<=$4 ORDER BY observed_at,id LIMIT 1`, id, previousHash, o.ObservedAt, end.ObservedAt).Scan(&raw)
			if err == nil {
				var earliest Observation
				if err = decodeJSON(raw, &earliest); err != nil {
					return err
				}
				resume = &earliest
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}
		// Only extend the preceding interval through actual retained observations.
		_, err = tx.Exec(ctx, `UPDATE player_equipment_history SET last_seen_at=greatest(first_seen_at,COALESCE((SELECT max(last_seen_at) FROM player_observations WHERE player_id=$2::uuid AND gear_hash=$3 AND observed_at<$4 AND last_seen_at<$4),first_seen_at)),last_evidence=COALESCE((SELECT evidence FROM player_observations WHERE player_id=$2::uuid AND gear_hash=$3 AND observed_at<$4 AND last_seen_at<$4 ORDER BY last_seen_at DESC,id DESC LIMIT 1),(SELECT evidence FROM player_observations WHERE id=observation_id)) WHERE id=$1::uuid`, historyID, id, previousHash, o.ObservedAt)
		if err != nil {
			return err
		}
	}
	insert := func(e *Equipment, from, until time.Time, observationID string, evidence []byte) error {
		raw, _ := json.Marshal(e)
		_, insertErr := tx.Exec(ctx, `INSERT INTO player_equipment_history(id,player_id,observation_id,equipment_json,last_evidence,gear_hash,identity_gear_hash,first_seen_at,last_seen_at) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7,$8,$9)`, newID(), id, observationID, raw, evidence, gearHash(e), nullableString(identityHash(e)), from, until)
		return insertErr
	}
	until := o.LastSeen
	var resumed *Equipment
	if resume != nil {
		resumed = mergeEquipment(equipment, resume.Equipment)
		if gearHash(resumed) == hash {
			until = last
			endpoint = endpointJSON
		}
	}
	err = insert(equipment, o.ObservedAt, until, o.ID, endpoint)
	if err != nil {
		return err
	}
	if resume != nil && gearHash(resumed) != hash {
		// The original start observation remains owned by its player. The copied
		// endpoint identifies and preserves the actual later source independently.
		if err = insert(resumed, resume.ObservedAt, last, previousObservation, endpointJSON); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE player_observations SET pinned=true WHERE id=$1::uuid`, o.ID)
	return err
}

// ResolveNames is one query per map response, and omits ambiguous aliases.
func (s *Store) ResolveNames(ctx context.Context, server string, names []string) (map[string]string, error) {
	result := map[string]string{}
	if s == nil || len(names) == 0 {
		return result, nil
	}
	lowered := []string{}
	for _, name := range names {
		lowered = append(lowered, strings.ToLower(name))
	}
	rows, err := s.pool.Query(ctx, `SELECT lower(a.alias_name),min(COALESCE(l.canonical_player_id,a.player_id)::text) FROM player_aliases a LEFT JOIN player_identity_links l ON l.linked_player_id=a.player_id AND l.status='confirmed' WHERE a.server_key=$1 AND lower(a.alias_name)=ANY($2::text[]) GROUP BY lower(a.alias_name) HAVING count(DISTINCT COALESCE(l.canonical_player_id,a.player_id))=1`, ServerKey(server), lowered)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, id string
		if err = rows.Scan(&name, &id); err != nil {
			return nil, err
		}
		result[name] = id
	}
	return result, rows.Err()
}

func (s *Store) sourceIDs(ctx context.Context, id string) ([]string, error) {
	if !ValidID(id) {
		return nil, ErrInvalid
	}
	var ids []string
	err := s.pool.QueryRow(ctx, `SELECT source_ids::text[] FROM player_registry WHERE id=COALESCE((SELECT canonical_player_id FROM player_identity_links WHERE linked_player_id=$1::uuid AND status='confirmed'),$1::uuid)`, id).Scan(&ids)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return ids, err
}

func (s *Store) canonicalID(ctx context.Context, id string) (string, error) {
	ids, err := s.sourceIDs(ctx, id)
	if err != nil {
		return "", err
	}
	var canonical string
	err = s.pool.QueryRow(ctx, `SELECT id::text FROM player_registry WHERE source_ids @> $1::uuid[]`, ids).Scan(&canonical)
	return canonical, err
}

func (s *Store) Prune(ctx context.Context, days int) (int64, error) {
	if days < 1 || days > 3650 {
		return 0, ErrInvalid
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM player_observations WHERE NOT pinned AND id IN (SELECT id FROM player_observations WHERE NOT pinned AND received_at<now()-make_interval(days=>$1) ORDER BY received_at LIMIT 5000)`, days)
	return tag.RowsAffected(), err
}

func (s *Store) RunRetention(ctx context.Context, days int) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run, cancel := context.WithTimeout(ctx, time.Minute)
			for i := 0; i < 12; i++ {
				n, err := s.Prune(run, days)
				if err != nil || n < 5000 {
					break
				}
			}
			cancel()
		}
	}
}

func (s *Store) String() string {
	return fmt.Sprintf("player registry (%d pending)", s.Status().Pending)
}

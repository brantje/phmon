package players

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

type Link struct {
	ID          string          `json:"id"`
	Server      string          `json:"server_key"`
	CanonicalID string          `json:"canonical_player_id"`
	LinkedID    string          `json:"linked_player_id"`
	Decision    string          `json:"decision"`
	Method      string          `json:"method"`
	Score       *int            `json:"confidence_score"`
	Evidence    json.RawMessage `json:"evidence_json"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	DecidedAt   *time.Time      `json:"decided_at"`
	Actor       *string         `json:"decided_by"`
	Revision    int64           `json:"revision"`
}
type LinkRequest struct {
	CandidateID       string `json:"candidate_id,omitempty"`
	CanonicalID       string `json:"canonical_player_id"`
	LinkedID          string `json:"linked_player_id"`
	Action            string `json:"action"`
	Reason            string `json:"reason"`
	Confirmed         bool   `json:"confirmed"`
	Revision          int64  `json:"revision"`
	CanonicalRevision int64  `json:"canonical_revision"`
	LinkedRevision    int64  `json:"linked_revision"`
}
type Classification struct {
	Name      string `json:"alias_name"`
	Type      string `json:"alias_type"`
	Job       string `json:"job_type"`
	Reason    string `json:"reason"`
	Confirmed bool   `json:"confirmed"`
	Revision  int64  `json:"revision"`
}

// TransitionEvidence is only produced by a profile-verified lifecycle decoder.
// map.players disappearance is not a despawn. No production packet decoder is
// currently enabled; the matcher can be exercised by explicitly synthetic tests.
type TransitionEvidence struct {
	Normal            Observation `json:"normal"`
	Job               Observation `json:"job"`
	DespawnAt         time.Time   `json:"despawn_at"`
	LifecycleVerified bool        `json:"lifecycle_verified"`
	Simultaneous      bool        `json:"simultaneous"`
	Competing         int         `json:"competing_candidates"`
}
type MatchAssessment struct {
	Eligible        bool     `json:"eligible"`
	Automatic       bool     `json:"automatic"`
	MatchingSlots   int      `json:"matching_slots"`
	ComparableSlots int      `json:"comparable_slots"`
	Reasons         []string `json:"reasons"`
}

func AssessTransition(e TransitionEvidence) MatchAssessment {
	a := MatchAssessment{Reasons: []string{}, Automatic: false}
	reject := func(reason string) { a.Reasons = append(a.Reasons, reason) }
	n, j := e.Normal, e.Job
	if ServerKey(n.Server) != ServerKey(j.Server) {
		reject("different_servers")
	}
	if n.NameType != "normal" || j.NameType != "job" {
		reject("unclassified_identity")
	}
	if !e.LifecycleVerified || n.SessionID == "" || n.SessionID != j.SessionID || n.Epoch == "" || n.Epoch != j.Epoch {
		reject("lifecycle_unverified")
	}
	before, after := n.ObservedAt, j.ObservedAt
	if before.After(after) {
		before, after = after, before
	}
	if e.DespawnAt.Before(before) || after.Before(e.DespawnAt) || after.Sub(e.DespawnAt) > 15*time.Second || after.Sub(before) > 60*time.Second {
		reject("outside_transition_window")
	}
	if n.Location == nil || j.Location == nil || n.Location.Region != j.Location.Region || n.Location.Scope == "" || n.Location.Scope != j.Location.Scope {
		reject("location_incomparable")
	} else if math.Hypot(n.Location.X-j.Location.X, n.Location.Y-j.Location.Y) > 30 {
		reject("position_too_far")
	}
	if n.Level != nil && j.Level != nil && *n.Level != *j.Level {
		reject("level_conflict")
	}
	if n.Model == nil || j.Model == nil || *n.Model != *j.Model {
		reject("model_incomparable")
	}
	a.MatchingSlots, a.ComparableSlots, _ = ComparableEquipment(n.Equipment, j.Equipment)
	_, _, conflict := ComparableEquipment(n.Equipment, j.Equipment)
	if conflict {
		reject("gear_conflict")
	}
	if a.ComparableSlots < 4 || a.MatchingSlots != a.ComparableSlots {
		reject("insufficient_comparable_gear")
	}
	if e.Simultaneous {
		reject("simultaneous_distinct_players")
	}
	if e.Competing > 1 {
		reject("ambiguous_candidates")
	}
	a.Eligible = len(a.Reasons) == 0
	if a.Eligible {
		a.Reasons = append(a.Reasons, "strong_transition_requires_runtime_validation")
	}
	return a
}

func (s *Store) AddTransitionCandidate(ctx context.Context, e TransitionEvidence) (string, error) {
	assessment := AssessTransition(e)
	for _, reason := range assessment.Reasons {
		if reason != "ambiguous_candidates" && reason != "strong_transition_requires_runtime_validation" {
			return "", ErrInvalid
		}
	}
	n, j := e.Normal, e.Job
	if !ValidID(n.PlayerID) || !ValidID(j.PlayerID) || n.PlayerID == j.PlayerID {
		return "", ErrInvalid
	}
	if ServerKey(n.Server) != ServerKey(j.Server) {
		return "", ErrInvalid
	}
	payload, _ := json.Marshal(map[string]any{"transition": e, "assessment": assessment, "probability": nil})
	key := contextHash([]string{n.ID, j.ID, "transition-v1"})
	id := newID()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO player_identity_links(id,server_key,canonical_player_id,linked_player_id,decision,method,evidence_json,status,candidate_key) VALUES($1::uuid,$2,$3::uuid,$4::uuid,'candidate','transition-v1',$5,'pending',$6) ON CONFLICT(candidate_key) WHERE candidate_key IS NOT NULL DO NOTHING`, id, ServerKey(n.Server), n.PlayerID, j.PlayerID, payload, key)
	if err != nil {
		return "", err
	}
	err = tx.QueryRow(ctx, `SELECT id::text FROM player_identity_links WHERE candidate_key=$1`, key).Scan(&id)
	if err != nil {
		return "", err
	}
	tag, err := tx.Exec(ctx, `UPDATE player_observations SET pinned=true WHERE server_key=$1 AND ((id=$2::uuid AND player_id=$3::uuid) OR (id=$4::uuid AND player_id=$5::uuid))`, ServerKey(n.Server), n.ID, n.PlayerID, j.ID, j.PlayerID)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() != 2 {
		return "", ErrInvalid
	}
	return id, tx.Commit(ctx)
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func (s *Store) Links(ctx context.Context, server, player, status string, limit int, token string) (HistoryPage[Link], error) {
	page := HistoryPage[Link]{Items: []Link{}}
	if !validText(server, 100) || player != "" && !ValidID(player) || limit < 1 || limit > 100 || status != "" && status != "pending" && status != "confirmed" && status != "rejected" && status != "revoked" {
		return page, ErrInvalid
	}
	context := contextHash([]string{ServerKey(server), player, status, "links"})
	c, err := decodeHistoryCursor(token, context)
	if err != nil {
		return page, err
	}
	ids := []string{}
	if player != "" {
		ids, err = s.sourceIDs(ctx, player)
		if err != nil {
			return page, err
		}
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,server_key,canonical_player_id::text,linked_player_id::text,decision,method,confidence_score,evidence_json,status,created_at,decided_at,decided_by,revision FROM player_identity_links WHERE ($1='' OR server_key=$1) AND ($2='' OR canonical_player_id=ANY($3::uuid[]) OR linked_player_id=ANY($3::uuid[])) AND ($4='' OR status=$4) AND ($5='' OR (created_at,id)<($6::timestamptz,$7::uuid)) ORDER BY created_at DESC,id DESC LIMIT $8`, ServerKey(server), player, ids, status, token, nullableString(c.Key), nullableString(c.ID), limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var l Link
		if err = rows.Scan(&l.ID, &l.Server, &l.CanonicalID, &l.LinkedID, &l.Decision, &l.Method, &l.Score, &l.Evidence, &l.Status, &l.CreatedAt, &l.DecidedAt, &l.Actor, &l.Revision); err != nil {
			return page, err
		}
		page.Items = append(page.Items, l)
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		l := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(cursor{context, l.CreatedAt.UTC().Format(time.RFC3339Nano), l.ID, time.Now().UTC(), false})
	}
	return page, rows.Err()
}

func (s *Store) Decide(ctx context.Context, r LinkRequest, actor string) (string, error) {
	if !r.Confirmed || !validText(r.Reason, 500) || r.Reason == "" || !ValidID(r.CanonicalID) || !ValidID(r.LinkedID) || r.CanonicalID == r.LinkedID || r.Action != "confirm" && r.Action != "reject" || r.CandidateID != "" && !ValidID(r.CandidateID) {
		return "", ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var server string
	err = tx.QueryRow(ctx, `SELECT server_key FROM players WHERE id=$1::uuid`, r.CanonicalID).Scan(&server)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,78113))`, server); err != nil {
		return "", err
	}
	ids := []string{r.CanonicalID, r.LinkedID}
	sort.Strings(ids)
	for _, id := range ids {
		var key string
		var revision int64
		err = tx.QueryRow(ctx, `SELECT server_key,revision FROM players WHERE id=$1::uuid FOR UPDATE`, id).Scan(&key, &revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		if err != nil {
			return "", err
		}
		expected := r.CanonicalRevision
		if id == r.LinkedID {
			expected = r.LinkedRevision
		}
		if key != server || revision != expected {
			return "", ErrConflict
		}
	}
	id := r.CandidateID
	if id != "" {
		var canonical, linked, status string
		var revision int64
		err = tx.QueryRow(ctx, `SELECT canonical_player_id::text,linked_player_id::text,status,revision FROM player_identity_links WHERE id=$1::uuid FOR UPDATE`, id).Scan(&canonical, &linked, &status, &revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		if err != nil {
			return "", err
		}
		if canonical != r.CanonicalID || linked != r.LinkedID || status != "pending" || revision != r.Revision {
			return "", ErrConflict
		}
	}
	if r.Action == "confirm" {
		var conflict bool
		// Only star associations are admitted. This keeps canonical lookup bounded and
		// prevents cycles or implicitly moving an already-associated group.
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_identity_links WHERE status='confirmed' AND (linked_player_id=ANY($1::uuid[]) OR canonical_player_id=$2::uuid))`, ids, r.LinkedID).Scan(&conflict)
		if err != nil {
			return "", err
		}
		if conflict {
			return "", ErrConflict
		}
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_observations a JOIN player_observations b ON a.observer_session_id=b.observer_session_id AND a.runtime_epoch=b.runtime_epoch AND a.observed_at=b.observed_at WHERE a.player_id=$1::uuid AND b.player_id=$2::uuid AND a.source='map.players' AND b.source='map.players' AND a.runtime_entity_id<>b.runtime_entity_id)`, r.CanonicalID, r.LinkedID).Scan(&conflict)
		if err != nil {
			return "", err
		}
		if conflict {
			return "", ErrConflict
		}
	}
	now := time.Now().UTC()
	// Preserve the current supporting observation for each source in the reviewed
	// projection. Lock before pinning so retention cannot remove cited evidence;
	// routine sightings that were not used by this decision may still expire.
	rows, err := tx.Query(ctx, `SELECT o.id::text FROM player_observations o JOIN (SELECT DISTINCT ON (player_id) id FROM player_observations WHERE player_id=ANY($1::uuid[]) OR player_id IN (SELECT linked_player_id FROM player_identity_links WHERE canonical_player_id=$2::uuid AND status='confirmed') ORDER BY player_id,observed_at DESC,id DESC) latest USING(id) ORDER BY o.id FOR UPDATE OF o`, ids, r.CanonicalID)
	if err != nil {
		return "", err
	}
	evidenceIDs := []string{}
	for rows.Next() {
		var evidenceID string
		if err = rows.Scan(&evidenceID); err != nil {
			rows.Close()
			return "", err
		}
		evidenceIDs = append(evidenceIDs, evidenceID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", err
	}
	audit, _ := json.Marshal(map[string]any{"action": r.Action, "actor": actor, "reason": r.Reason, "at": now, "evidence_observation_ids": evidenceIDs, "canonical_revision": r.CanonicalRevision, "linked_revision": r.LinkedRevision})
	status := "confirmed"
	if r.Action == "reject" {
		status = "rejected"
	}
	if id == "" {
		id = newID()
		payload, _ := json.Marshal(map[string]any{"manual": true, "audit": []json.RawMessage{audit}})
		_, err = tx.Exec(ctx, `INSERT INTO player_identity_links(id,server_key,canonical_player_id,linked_player_id,decision,method,evidence_json,status,decided_at,decided_by) VALUES($1::uuid,$2,$3::uuid,$4::uuid,$5,'manual',$6,$7,$8,$9)`, id, server, r.CanonicalID, r.LinkedID, r.Action, payload, status, now, actor)
	} else {
		_, err = tx.Exec(ctx, `UPDATE player_identity_links SET decision=$2,status=$3,decided_at=$4,decided_by=$5,revision=revision+1,evidence_json=jsonb_set(evidence_json,'{audit}',COALESCE(evidence_json->'audit','[]'::jsonb)||jsonb_build_array($6::jsonb)) WHERE id=$1::uuid`, id, r.Action, status, now, actor, audit)
	}
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE player_observations SET pinned=true WHERE id=ANY($1::uuid[])`, evidenceIDs)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE players SET revision=revision+1 WHERE id=ANY($1::uuid[])`, ids)
	if err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

func (s *Store) Unlink(ctx context.Context, id string, revision int64, reason, actor string) error {
	if !ValidID(id) || revision < 1 || reason == "" || !validText(reason, 500) {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var server string
	err = tx.QueryRow(ctx, `SELECT server_key FROM player_identity_links WHERE id=$1::uuid`, id).Scan(&server)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,78113))`, server)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `SELECT id FROM players WHERE id IN (SELECT canonical_player_id FROM player_identity_links WHERE id=$1::uuid UNION SELECT linked_player_id FROM player_identity_links WHERE id=$1::uuid) ORDER BY id FOR UPDATE`, id)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	audit, _ := json.Marshal(map[string]any{"action": "unlink", "actor": actor, "reason": reason, "at": now})
	tag, err := tx.Exec(ctx, `UPDATE player_identity_links SET status='revoked',decision='unlink',decided_at=$3,decided_by=$4,revision=revision+1,evidence_json=jsonb_set(evidence_json,'{audit}',COALESCE(evidence_json->'audit','[]'::jsonb)||jsonb_build_array($5::jsonb)) WHERE id=$1::uuid AND revision=$2 AND status='confirmed'`, id, revision, now, actor, audit)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE players SET revision=revision+1 WHERE id IN (SELECT canonical_player_id FROM player_identity_links WHERE id=$1::uuid UNION SELECT linked_player_id FROM player_identity_links WHERE id=$1::uuid)`, id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Classify(ctx context.Context, id string, c Classification, actor string) error {
	if !ValidID(id) || !c.Confirmed || c.Revision < 1 || c.Name == "" || !validText(c.Name, 64) || c.Reason == "" || !validText(c.Reason, 500) || c.Type != "normal" && c.Type != "job" || c.Type == "job" && (c.Job != "trader" && c.Job != "hunter" && c.Job != "thief") {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var server string
	err = tx.QueryRow(ctx, `SELECT server_name FROM players WHERE id=$1::uuid`, id).Scan(&server)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,78113))`, ServerKey(server))
	if err != nil {
		return err
	}
	var revision int64
	var firstSeen, lastSeen time.Time
	err = tx.QueryRow(ctx, `SELECT revision,first_seen_at,last_seen_at FROM players WHERE id=$1::uuid FOR UPDATE`, id).Scan(&revision, &firstSeen, &lastSeen)
	if err != nil {
		return err
	}
	if revision != c.Revision {
		return ErrConflict
	}
	var observed bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_aliases WHERE player_id=$1::uuid AND lower(alias_name)=lower($2))`, id, c.Name).Scan(&observed)
	if err != nil {
		return err
	}
	if !observed {
		return ErrInvalid
	}
	var conflicting bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_aliases WHERE player_id<>$1::uuid AND server_key=$2 AND lower(alias_name)=lower($3) AND alias_type='normal' AND confirmation_status='confirmed')`, id, ServerKey(server), c.Name).Scan(&conflicting)
	if err != nil {
		return err
	}
	if conflicting && c.Type == "normal" {
		return ErrConflict
	}
	now := time.Now().UTC()
	// A correction supersedes classification without deleting earlier aliases or
	// observations. Conflicting classifications remain visible in the audit trail.
	_, err = tx.Exec(ctx, `UPDATE player_aliases SET confirmation_status='conflict' WHERE player_id=$1::uuid AND lower(alias_name)=lower($2) AND alias_type<>$3 AND alias_type<>'unknown'`, id, c.Name, c.Type)
	if err != nil {
		return err
	}
	if c.Type == "job" {
		_, err = tx.Exec(ctx, `UPDATE players SET name=NULL,field_times=jsonb_set(field_times,'{name}',to_jsonb($3::timestamptz)) WHERE id=$1::uuid AND lower(name)=lower($2)`, id, c.Name, now)
	} else {
		_, err = tx.Exec(ctx, `UPDATE players SET job_name=NULL,job=NULL,field_times=jsonb_set(jsonb_set(field_times,'{job_name}',to_jsonb($3::timestamptz)),'{job}',to_jsonb($3::timestamptz)) WHERE id=$1::uuid AND lower(job_name)=lower($2)`, id, c.Name, now)
	}
	if err != nil {
		return err
	}
	evidence, _ := json.Marshal(map[string]any{"actor": actor, "reason": c.Reason, "action": "classify", "previous_revision": revision})
	o := Observation{PlayerID: id, Server: server, Name: c.Name, NameType: c.Type, Source: "operator.classification", SourceRef: newID(), ObservedAt: now, Evidence: evidence, Pinned: true}
	if c.Type == "job" {
		o.Job = &c.Job
	}
	if err = o.normalize(); err != nil {
		return err
	}
	if err = s.apply(ctx, tx, o); err != nil {
		return err
	}
	// Operator decisions are evidence updates, not sightings of a live player.
	_, err = tx.Exec(ctx, `UPDATE players SET first_seen_at=$2,last_seen_at=$3 WHERE id=$1::uuid`, id, firstSeen, lastSeen)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE player_aliases SET first_seen_at=$4,last_seen_at=$5 WHERE player_id=$1::uuid AND lower(alias_name)=lower($2) AND alias_type=$3`, id, c.Name, c.Type, firstSeen, lastSeen)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// recordAliasConflict retains the records and evidence; it never associates them.
func (s *Store) recordAliasConflict(ctx context.Context, tx pgx.Tx, prior, current string, o Observation, reasons []string) error {
	ids := []string{prior, current}
	sort.Strings(ids)
	key := contextHash([]string{ServerKey(o.Server), ids[0], ids[1], "alias-conflict-v1"})
	var previous string
	err := tx.QueryRow(ctx, `SELECT id::text FROM player_observations WHERE player_id=$1::uuid ORDER BY observed_at DESC,id DESC LIMIT 1`, prior).Scan(&previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	evidence, _ := json.Marshal(map[string]any{"alias": o.Name, "reasons": reasons, "observation_ids": []string{previous, o.ID}, "assessment": "uncalibrated conflict; operator review required", "automatic": false})
	_, err = tx.Exec(ctx, `INSERT INTO player_identity_links(id,server_key,canonical_player_id,linked_player_id,decision,method,evidence_json,status,candidate_key) VALUES($1::uuid,$2,$3::uuid,$4::uuid,'candidate','alias-conflict-v1',$5,'pending',$6) ON CONFLICT(candidate_key) WHERE candidate_key IS NOT NULL DO NOTHING`, newID(), ServerKey(o.Server), prior, current, evidence, key)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE player_observations SET pinned=true WHERE id=ANY($1::uuid[])`, []string{previous, o.ID})
	return err
}

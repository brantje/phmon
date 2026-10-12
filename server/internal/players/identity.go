package players

import (
	"context"
	"encoding/json"
	"time"
)

// DisplayIdentity groups presentation only; tracking IDs and history are retained.
type DisplayIdentity struct {
	CanonicalID      string    `json:"canonical_id"`
	PlayerName       *string   `json:"player_name"`
	JobAlias         *string   `json:"job_alias"`
	Aliases          []string  `json:"aliases"`
	RelatedRecordIDs []string  `json:"related_record_ids"`
	Source           string    `json:"source"`
	Status           string    `json:"status"`
	FirstSeenAt      time.Time `json:"first_seen_at"`
	LastSeenAt       time.Time `json:"last_seen_at"`
}

// The same relational projection drives list filtering/pagination and detail
// identity. No inferred link updates a tracking row or grants live presence.
const identityCTE = `WITH identity_base AS MATERIALIZED (
 SELECT p.*,
   CASE WHEN pe.field_evidence->'model_ref_id'->>'source_identity_kind' IS DISTINCT FROM 'job_alias'
     THEN pe.model_ref_id END AS proxy_model_ref_id,
   COALESCE(pe.extended, '{}'::jsonb) AS enrichment_extended,
   COALESCE(p.player_name, CASE
     WHEN p.job_name IS NOT NULL OR p.is_jobbing IS TRUE OR EXISTS (
       SELECT 1 FROM player_observations o WHERE o.player_id=p.id AND o.source='thief_sighting'
     ) THEN NULL
     WHEN p.is_jobbing IS FALSE OR EXISTS (
       SELECT 1 FROM characters c WHERE c.server_key=p.server_key AND c.identity_key=p.observed_name_key
     ) OR EXISTS (
       SELECT 1 FROM jsonb_each(COALESCE(pe.field_evidence, '{}'::jsonb)) ev
       WHERE (ev.value->>'source_identity_kind'='character_name'
              AND lower(btrim(ev.value->>'source_name'))=p.observed_name_key)
          OR (ev.key IN ('name','gear','model_ref_id') AND ev.value->>'reason'='verified'
              AND NOT (ev.value ? 'source_identity_kind') AND NOT (ev.value ? 'association'))
     ) THEN p.observed_name END) AS normal_name,
   COALESCE(p.job_name, CASE WHEN p.is_jobbing IS TRUE OR EXISTS (
     SELECT 1 FROM player_observations o WHERE o.player_id=p.id AND o.source='thief_sighting'
   ) THEN p.observed_name END) AS alias_name
 FROM players p LEFT JOIN player_enrichments pe ON pe.player_id=p.id
), identity_links AS MATERIALIZED (
 SELECT b.id AS owner_id, b.server_key, b.normal_name, link->>'alias' AS alias,
   lower(btrim(link->>'alias')) AS alias_key, (link->>'last_matched')::timestamptz AS matched_at
 FROM identity_base b CROSS JOIN LATERAL jsonb_array_elements(
   CASE WHEN jsonb_typeof(b.enrichment_extended->'identity_links')='array'
     THEN b.enrichment_extended->'identity_links' ELSE '[]'::jsonb END
 ) link
 WHERE b.normal_name IS NOT NULL AND lower(btrim(b.normal_name))=b.observed_name_key
   AND link->>'status'='inferred' AND link->>'reason'='job_item_transition'
), identity_link_owners AS (
 SELECT server_key, alias_key, COUNT(DISTINCT owner_id) AS owners
 FROM identity_links GROUP BY server_key, alias_key
), identity_safe_links AS MATERIALIZED (
 SELECT l.* FROM identity_links l
 JOIN identity_link_owners counts USING(server_key, alias_key)
 LEFT JOIN identity_base target ON target.server_key=l.server_key AND target.observed_name_key=l.alias_key
 WHERE counts.owners=1 AND l.alias_key <> lower(btrim(l.normal_name))
   AND (target.normal_name IS NULL OR lower(btrim(target.normal_name))=lower(btrim(l.normal_name)))
   AND (target.alias_name IS NULL OR lower(btrim(target.alias_name))=l.alias_key)
), identity_conflicts AS (
 SELECT l.* FROM identity_links l WHERE NOT EXISTS (
   SELECT 1 FROM identity_safe_links s WHERE s.owner_id=l.owner_id AND s.alias_key=l.alias_key
 )
), identity_members AS MATERIALIZED (
 SELECT b.id, COALESCE(l.owner_id,b.id) AS canonical_id
 FROM identity_base b LEFT JOIN identity_safe_links l
 ON l.server_key=b.server_key AND l.alias_key=b.observed_name_key
), identity_groups AS (
 SELECT m.canonical_id, MIN(b.first_seen_at) AS first_seen_at, MAX(b.last_seen_at) AS last_seen_at,
   array_agg(b.id::text ORDER BY b.id) AS related_ids,
   array_agg(b.observed_name) AS observed_names
 FROM identity_members m JOIN identity_base b ON b.id=m.id GROUP BY m.canonical_id
), identity_players AS (
 SELECT b.*,
   b.normal_name AS display_player_name,
   COALESCE(b.alias_name, a.latest_alias) AS display_job_alias,
   g.first_seen_at AS display_first_seen_at, g.last_seen_at AS display_last_seen_at,
   g.observed_names || COALESCE(a.aliases, ARRAY[]::text[]) AS search_names,
   jsonb_build_object(
     'canonical_id', b.id::text, 'player_name', b.normal_name,
     'job_alias', COALESCE(b.alias_name,a.latest_alias),
     'aliases', CASE WHEN b.alias_name IS NOT NULL AND NOT (b.alias_name=ANY(COALESCE(a.aliases,ARRAY[]::text[])))
       THEN ARRAY[b.alias_name] || COALESCE(a.aliases,ARRAY[]::text[])
       ELSE COALESCE(a.aliases,ARRAY[]::text[]) END,
     'related_record_ids', g.related_ids,
     'source', CASE WHEN a.latest_alias IS NOT NULL OR EXISTS (
       SELECT 1 FROM identity_conflicts c WHERE c.owner_id=b.id OR
         (c.server_key=b.server_key AND c.alias_key=b.observed_name_key)
     ) THEN 'ph-proxy' ELSE 'observation' END,
     'status', CASE WHEN EXISTS (
        SELECT 1 FROM identity_conflicts c WHERE c.owner_id=b.id OR
          (c.server_key=b.server_key AND c.alias_key=b.observed_name_key)
       ) THEN 'conflict' WHEN a.latest_alias IS NOT NULL THEN 'inferred'
       WHEN b.normal_name IS NOT NULL OR b.alias_name IS NOT NULL THEN 'observed' ELSE 'unknown' END,
     'first_seen_at',g.first_seen_at,'last_seen_at',g.last_seen_at
   ) AS display_identity
 FROM identity_base b JOIN identity_groups g ON g.canonical_id=b.id
 LEFT JOIN LATERAL (
   SELECT (array_agg(alias ORDER BY matched_at DESC,alias_key ASC))[1] AS latest_alias,
     array_agg(alias ORDER BY matched_at DESC,alias_key ASC) AS aliases
   FROM identity_safe_links l WHERE l.owner_id=b.id
 ) a ON TRUE
)
`

func (s *Registry) loadDisplayIdentity(ctx context.Context, id string) (*DisplayIdentity, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, identityCTE+`SELECT display_identity FROM identity_players
   WHERE id=(SELECT canonical_id FROM identity_members WHERE id=$1::uuid)`, id).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var identity DisplayIdentity
	if err := json.Unmarshal(raw, &identity); err != nil {
		return nil, err
	}
	return &identity, nil
}

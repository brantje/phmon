package players

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Nearby names do not distinguish normal character names from job aliases.
// Keep those sightings in LiveStore, but do not accumulate registry/history rows
// until native classification or accepted proxy evidence identifies the name.
// Apply remains available for explicitly attributed tracking sources/imports.
func (s *Registry) admitLiveObservations(ctx context.Context, observations []Observation) ([]Observation, error) {
	keys := make([]string, 0, len(observations))
	for _, obs := range observations {
		if err := obs.validate(); err != nil {
			return nil, err
		}
		if obs.IsJobbing == nil {
			keys = append(keys, nameKey(obs.ObservedName))
		}
	}
	if len(keys) == 0 {
		return observations, nil
	}
	server := serverKey(observations[0].Server)
	// Limit the materialized identity projection to candidate names and owners
	// of candidate aliases. Include *all* candidate alias owners so the
	// existing unique-owner/conflict checks remain authoritative.
	rows, err := s.pool.Query(ctx, identityQuery(`WHERE p.server_key=$1 AND (
   p.observed_name_key=ANY($2::text[])
   OR EXISTS (
     SELECT 1 FROM player_enrichments owners
     CROSS JOIN LATERAL jsonb_array_elements(
       CASE WHEN jsonb_typeof(owners.extended->'identity_links')='array'
         THEN owners.extended->'identity_links' ELSE '[]'::jsonb END
     ) link
     WHERE owners.player_id=p.id
       AND lower(btrim(link->>'alias'))=ANY($2::text[])
       AND link->>'status'='inferred'
       AND link->>'reason'='job_item_transition'
   )
 )`)+`
 SELECT b.observed_name_key FROM identity_base b
 JOIN identity_members m ON m.id=b.id
 JOIN identity_players p ON p.id=m.canonical_id
 WHERE b.observed_name_key=ANY($2::text[])
   AND (p.display_player_name IS NOT NULL OR p.display_job_alias IS NOT NULL)
 UNION
 SELECT alias_key FROM identity_safe_links WHERE alias_key=ANY($2::text[])
 UNION
 SELECT identity_key FROM characters WHERE server_key=$1 AND identity_key=ANY($2::text[])
 UNION
 SELECT pending.name_key FROM player_enrichment_pending pending
 WHERE $3 AND pending.server_key=$1 AND pending.name_key=ANY($2::text[])
   AND pending.expires_at>=now()
   AND EXISTS (
     SELECT 1 FROM jsonb_each(pending.field_evidence) ev
     WHERE (ev.value->>'source_identity_kind'='character_name'
            AND lower(btrim(ev.value->>'source_name'))=pending.name_key)
        OR (ev.key IN ('name','gear','model_ref_id') AND ev.value->>'reason'='verified'
            AND NOT (ev.value ? 'source_identity_kind') AND NOT (ev.value ? 'association'))
   )`, server, keys, s.promoter != nil)
	if err != nil {
		return nil, err
	}
	allowed, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		known[key] = true
	}
	admitted := make([]Observation, 0, len(observations))
	for _, obs := range observations {
		if obs.IsJobbing != nil || known[nameKey(obs.ObservedName)] {
			admitted = append(admitted, obs)
		}
	}
	return admitted, nil
}

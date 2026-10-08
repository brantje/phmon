package players

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// ImportThiefSightings references the existing source ID. It never copies the
// subsystem's reporting/transport logic. The immutable evidence survives its
// shorter retention, and observer positions are not attributed to the thief.
func (s *Store) ImportThiefSightings(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT t.sighting_id::text,t.server_name,t.thief_name,t.region,t.x,t.y,t.z,t.position_source,t.observed_at,to_jsonb(t) FROM thief_sightings t WHERE NOT EXISTS(SELECT 1 FROM player_observations o WHERE o.server_key=lower(trim(t.server_name)) AND o.source='thief_sighting' AND o.source_ref=t.sighting_id::text) ORDER BY t.received_at,t.sighting_id LIMIT 128`)
	if err != nil {
		return 0, err
	}
	observations := []Observation{}
	for rows.Next() {
		var id, server, name, positionSource string
		var region *int
		var x, y, z *float64
		var observed time.Time
		var evidence json.RawMessage
		if err = rows.Scan(&id, &server, &name, &region, &x, &y, &z, &positionSource, &observed, &evidence); err != nil {
			rows.Close()
			return 0, err
		}
		job := "thief"
		o := Observation{Server: server, Name: name, NameType: "job", Job: &job, Source: "thief_sighting", SourceRef: id, ObservedAt: observed, Evidence: evidence, Pinned: true}
		if positionSource == "thief" && region != nil && x != nil && y != nil {
			o.Location = &Location{Region: *region, X: *x, Y: *y, Z: z}
		}
		observations = append(observations, o)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	if len(observations) == 0 {
		return 0, nil
	}
	return len(observations), s.ApplyObservations(ctx, observations)
}
func (s *Store) RunThiefEvidence(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	lastLog := time.Time{}
	for {
		run, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, err := s.ImportThiefSightings(run)
		cancel()
		if err != nil && time.Since(lastLog) > 30*time.Second {
			lastLog = time.Now()
			slog.Warn("player thief evidence import retrying")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

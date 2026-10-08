package players

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/database"
)

func registryFixture(t *testing.T) (context.Context, *Store, string) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for player registry integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	server := "Fixture Registry " + newID()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM player_identity_links WHERE server_key=$1`, ServerKey(server))
		_, _ = pool.Exec(context.Background(), `DELETE FROM players WHERE server_key=$1`, ServerKey(server))
	})
	return ctx, NewStore(pool), server
}
func fixtureObservation(server, name string, at time.Time) Observation {
	return Observation{Server: server, Name: name, NameType: "unknown", SessionID: "fixture-observer-session", AgentID: "fixture-agent", CharacterID: "fixture-character", RuntimeID: "7", Epoch: "1", Level: intRef(100), Guild: textRef("Fixture Guild"), Location: &Location{Region: 25000, X: 10, Y: 20}, Source: "map.players", ObservedAt: at}
}
func registryList(t *testing.T, ctx context.Context, s *Store, server string) []Record {
	t.Helper()
	page, err := s.List(ctx, Filter{Server: server})
	if err != nil {
		t.Fatal(err)
	}
	return page.Players
}
func TestRegistryPersistenceObserversOutOfOrderAndFilters(t *testing.T) {
	ctx, s, server := registryFixture(t)
	base := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	first := fixtureObservation(server, "Observed", base)
	if err := s.ApplyObservations(ctx, []Observation{first}); err != nil {
		t.Fatal(err)
	}
	rows := registryList(t, ctx, s, server)
	if len(rows) != 1 || rows[0].Name != nil || rows[0].Job != nil || rows[0].Resolved {
		t.Fatalf("unclassified name became normal %+v", rows)
	}
	id := rows[0].ID
	for i := 0; i < 100; i++ {
		o := fixtureObservation(server, "Observed", base.Add(time.Duration(i)*time.Second))
		if err := s.ApplyObservations(ctx, []Observation{o}); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM player_observations WHERE player_id=$1::uuid`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count > 4 {
		t.Fatalf("unchanged snapshots generated %d rows", count)
	}
	other := fixtureObservation(server, "observed", base.Add(2*time.Minute))
	other.SessionID = "other-observer"
	other.RuntimeID = "900"
	other.Level = intRef(110)
	other.Guild = textRef("New Guild")
	if err := s.ApplyObservations(ctx, []Observation{other}); err != nil {
		t.Fatal(err)
	}
	late := fixtureObservation(server, "Observed", base.Add(-time.Minute))
	late.Level = intRef(80)
	late.Guild = nil
	if err := s.ApplyObservations(ctx, []Observation{late}); err != nil {
		t.Fatal(err)
	}
	record, err := NewStore(s.pool).Get(ctx, id)
	if err != nil || *record.Level != 110 || *record.Guild != "New Guild" || !record.FirstSeen.Equal(late.ObservedAt) || !record.LastSeen.Equal(other.ObservedAt) {
		t.Fatalf("restart/ordering %+v %v", record, err)
	}
	for _, f := range []Filter{{Name: "OBS", MinLevel: intRef(105), MaxLevel: intRef(120), Guild: "new", Job: "unknown", Equipment: "unavailable", Identity: "unresolved"}, {Seen: "24h"}, {Sort: "name", Direction: "asc"}, {Sort: "level", Direction: "desc"}} {
		f.Server = server
		p, e := s.List(ctx, f)
		if e != nil || p.Total != 1 {
			t.Fatalf("filter %+v -> %+v %v", f, p, e)
		}
	}
	different := first
	different.Server = server + " other"
	if err := s.ApplyObservations(ctx, []Observation{different}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(context.Background(), `DELETE FROM players WHERE server_key=$1`, ServerKey(different.Server))
	})
	if registryList(t, ctx, s, different.Server)[0].ID == id {
		t.Fatal("cross-server contamination")
	}
	missing := other
	missing.ObservedAt = base.Add(3 * time.Minute)
	missing.Level = nil
	missing.Guild = nil
	if err := s.ApplyObservations(ctx, []Observation{missing}); err != nil {
		t.Fatal(err)
	}
	record, _ = s.Get(ctx, id)
	if *record.Level != 110 || *record.Guild != "New Guild" {
		t.Fatal("missing erased known")
	}
}
func TestRegistryEquipmentIntervalsAndManualLinkReversal(t *testing.T) {
	ctx, s, server := registryFixture(t)
	base := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	for i := 0; i < 4; i++ {
		o := fixtureObservation(server, "Normal", base.Add(time.Duration(i)*time.Minute))
		o.Equipment = fixtureEquipment(o.ObservedAt)
		if i == 2 {
			o.Equipment.Slots[0].Plus = intRef(8)
		}
		if err := s.ApplyObservations(ctx, []Observation{o}); err != nil {
			t.Fatal(err)
		}
	}
	normal := registryList(t, ctx, s, server)[0]
	history, err := s.EquipmentHistory(ctx, normal.ID, 25, "")
	if err != nil || len(history.Items) != 3 {
		t.Fatalf("A/B/A intervals %+v %v", history, err)
	}
	if err = s.Classify(ctx, normal.ID, Classification{Name: "Normal", Type: "normal", Reason: "synthetic fixture", Confirmed: true, Revision: normal.Revision}, "fixture_operator"); err != nil {
		t.Fatal(err)
	}
	normal, _ = s.Get(ctx, normal.ID)
	alias := fixtureObservation(server, "SecretHunter", base.Add(5*time.Minute))
	alias.SessionID = "second-session"
	alias.RuntimeID = "22"
	job := "hunter"
	alias.NameType = "job"
	alias.Job = &job
	alias.Equipment = fixtureEquipment(alias.ObservedAt)
	if err = s.ApplyObservations(ctx, []Observation{alias}); err != nil {
		t.Fatal(err)
	}
	rows := registryList(t, ctx, s, server)
	var target Record
	for _, r := range rows {
		if r.ID != normal.ID {
			target = r
		}
	}
	if target.Name != nil || target.Resolved {
		t.Fatal("job-only identity considered normal/resolved")
	}
	request := LinkRequest{CanonicalID: normal.ID, LinkedID: target.ID, Action: "confirm", Reason: "fixture manual evidence", Confirmed: true, CanonicalRevision: normal.Revision, LinkedRevision: target.Revision}
	linkID, err := s.Decide(ctx, request, "fixture_operator")
	if err != nil {
		t.Fatal(err)
	}
	merged, err := s.Get(ctx, target.ID)
	if err != nil || merged.ID != normal.ID || len(merged.SourceIDs) != 2 || len(registryList(t, ctx, s, server)) != 1 {
		t.Fatalf("canonical projection %+v %v", merged, err)
	}
	source, err := s.GetSource(ctx, target.ID)
	if err != nil || source.ID != target.ID || source.Name != nil {
		t.Fatal("source record lost")
	}
	if _, err = s.Decide(ctx, request, "fixture_operator"); !errors.Is(err, ErrConflict) {
		t.Fatal("stale concurrent decision accepted")
	}
	links, _ := s.Links(ctx, server, "", "confirmed", 25, "")
	if len(links.Items) != 1 {
		t.Fatal("confirmed link unavailable")
	}
	if err = s.Unlink(ctx, linkID, links.Items[0].Revision, "fixture correction", "fixture_operator"); err != nil {
		t.Fatal(err)
	}
	restored, _ := s.Get(ctx, target.ID)
	if restored.ID != target.ID || len(registryList(t, ctx, s, server)) != 2 {
		t.Fatal("revocation failed")
	}
	var pinned int
	s.pool.QueryRow(ctx, `SELECT count(*) FROM player_observations WHERE server_key=$1 AND pinned`, ServerKey(server)).Scan(&pinned)
	if pinned < 2 {
		t.Fatal("decision evidence not preserved")
	}
}

func TestRegistryDecisionPinsOnlyReferencedEvidence(t *testing.T) {
	ctx, s, server := registryFixture(t)
	base := time.Now().Add(-100 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	for _, name := range []string{"RetentionNormal", "RetentionJob"} {
		for i := 0; i < 8; i++ {
			o := fixtureObservation(server, name, base.Add(time.Duration(i)*time.Minute))
			o.SessionID = name + "-observer"
			if err := s.ApplyObservations(ctx, []Observation{o}); err != nil {
				t.Fatal(err)
			}
		}
	}
	players := registryList(t, ctx, s, server)
	if len(players) != 2 {
		t.Fatal("missing fixture players")
	}
	r := LinkRequest{CanonicalID: players[0].ID, LinkedID: players[1].ID, Action: "confirm", Reason: "fixture reviewed association", Confirmed: true, CanonicalRevision: players[0].Revision, LinkedRevision: players[1].Revision}
	if _, err := s.Decide(ctx, r, "fixture_operator"); err != nil {
		t.Fatal(err)
	}
	var pinned int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM player_observations WHERE server_key=$1 AND pinned`, ServerKey(server)).Scan(&pinned); err != nil || pinned != 2 {
		t.Fatalf("decision pinned unrelated routine sightings: %d %v", pinned, err)
	}
	links, err := s.Links(ctx, server, "", "confirmed", 25, "")
	if err != nil || len(links.Items) != 1 {
		t.Fatal("missing decision audit", err)
	}
	var evidence struct {
		Audit []struct {
			IDs []string `json:"evidence_observation_ids"`
		} `json:"audit"`
	}
	if err = decodeJSON(links.Items[0].Evidence, &evidence); err != nil || len(evidence.Audit) != 1 || len(evidence.Audit[0].IDs) != 2 {
		t.Fatal("decision evidence references missing", err)
	}
	if _, err = s.pool.Exec(ctx, `UPDATE player_observations SET received_at=now()-interval '100 days' WHERE server_key=$1`, ServerKey(server)); err != nil {
		t.Fatal(err)
	}
	if deleted, err := s.Prune(ctx, 90); err != nil || deleted != 14 {
		t.Fatalf("routine retention: removed %d, %v", deleted, err)
	}
	var retained int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM player_observations WHERE id=ANY($1::uuid[])`, evidence.Audit[0].IDs).Scan(&retained); err != nil || retained != 2 {
		t.Fatal("decision supporting evidence was removed", err)
	}
}

func TestRegistryDelayedEquipmentSplitsIntervalsAndKeepsEvidence(t *testing.T) {
	ctx, s, server := registryFixture(t)
	base := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	for _, seconds := range []int{0, 30, 10} {
		o := fixtureObservation(server, "DelayedGear", base.Add(time.Duration(seconds)*time.Second))
		o.Equipment = fixtureEquipment(o.ObservedAt)
		if seconds == 10 {
			o.Equipment.Slots = o.Equipment.Slots[:1]
			o.Equipment.Slots[0].Plus = intRef(8)
		}
		if err := s.ApplyObservations(ctx, []Observation{o}); err != nil {
			t.Fatal(err)
		}
		if seconds == 30 {
			// Routine rows may already have expired when delayed equipment arrives.
			if _, err := s.pool.Exec(ctx, `DELETE FROM player_observations WHERE player_id=(SELECT player_id FROM player_aliases WHERE server_key=$1 AND alias_name='DelayedGear') AND NOT pinned`, ServerKey(server)); err != nil {
				t.Fatal(err)
			}
		}
	}
	record := registryList(t, ctx, s, server)[0]
	history, err := s.EquipmentHistory(ctx, record.ID, 25, "")
	if err != nil || len(history.Items) != 3 {
		t.Fatalf("delayed A/B/A history %+v %v", history, err)
	}
	for i, seconds := range []int{30, 10, 0} {
		h := history.Items[i]
		if !h.FirstSeen.Equal(base.Add(time.Duration(seconds)*time.Second)) || !h.LastSeen.Equal(h.FirstSeen) || h.LastEvidence.ID == "" || len(h.Equipment.Slots) != 6 {
			t.Fatalf("incorrect interval or lost partial knowledge %+v", h)
		}
	}
	if history.Items[0].GearHash != history.Items[2].GearHash || history.Items[0].GearHash == history.Items[1].GearHash || gearHash(record.Gear) != history.Items[0].GearHash {
		t.Fatal("late change replaced newer facts or lost a reversal")
	}
	// Endpoint evidence is part of meaningful history, independent of routine
	// sighting retention. A still earlier change can split that retained interval.
	_, err = s.pool.Exec(ctx, `DELETE FROM player_observations WHERE player_id=$1::uuid AND NOT pinned`, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	late := fixtureObservation(server, "DelayedGear", base.Add(20*time.Second))
	late.Equipment = fixtureEquipment(late.ObservedAt)
	late.Equipment.Slots[0].Plus = intRef(9)
	if err = s.ApplyObservations(ctx, []Observation{late}); err != nil {
		t.Fatal(err)
	}
	history, err = s.EquipmentHistory(ctx, record.ID, 25, "")
	if err != nil || len(history.Items) != 4 || !history.Items[0].FirstSeen.Equal(base.Add(30*time.Second)) || !history.Items[1].FirstSeen.Equal(late.ObservedAt) {
		t.Fatalf("retained endpoint replay %+v %v", history, err)
	}
}
func TestRegistryCursorPaginationConcurrentAdmissionAndRetention(t *testing.T) {
	ctx, s, server := registryFixture(t)
	base := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Microsecond)
	o := fixtureObservation(server, "Shared", base)
	var wg sync.WaitGroup
	errorsChan := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v := o
			v.SessionID = fmt.Sprintf("session-%d", i)
			errorsChan <- s.ApplyObservations(ctx, []Observation{v})
		}(i)
	}
	wg.Wait()
	close(errorsChan)
	for err := range errorsChan {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(registryList(t, ctx, s, server)) != 1 {
		t.Fatal("concurrent duplicate players")
	}
	batch := []Observation{}
	for i := 0; i < 30; i++ {
		v := fixtureObservation(server, fmt.Sprintf("Player%02d", i), base.Add(time.Duration(i)*time.Second))
		v.RuntimeID = fmt.Sprint(i + 100)
		batch = append(batch, v)
	}
	if err := s.ApplyObservations(ctx, batch); err != nil {
		t.Fatal(err)
	}
	for _, sortKey := range []string{"last_seen", "name", "level", "guild", "job"} {
		f := Filter{Server: server, Limit: 7, Sort: sortKey, Direction: "asc"}
		firstPage, _ := s.List(ctx, f)
		f.Cursor = firstPage.NextCursor
		secondPage, err := s.List(ctx, f)
		if err != nil || secondPage.PreviousCursor == "" {
			t.Fatal("previous cursor unavailable", err)
		}
		f.Cursor = secondPage.PreviousCursor
		back, err := s.List(ctx, f)
		if err != nil || len(back.Players) != len(firstPage.Players) || back.PreviousCursor != "" {
			t.Fatal("previous cursor did not reach first page", err)
		}
		for i := range back.Players {
			if back.Players[i].ID != firstPage.Players[i].ID {
				t.Fatal("reverse pagination reordered rows")
			}
		}
		f.Cursor = ""
		seen := map[string]bool{}
		for {
			page, err := s.List(ctx, f)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range page.Players {
				if seen[r.ID] {
					t.Fatal("duplicate paginated player")
				}
				seen[r.ID] = true
			}
			if page.NextCursor == "" {
				break
			}
			f.Cursor = page.NextCursor
		}
		if len(seen) != 31 {
			t.Fatalf("%s pagination missed rows: %d", sortKey, len(seen))
		}
	}
	_, err := s.pool.Exec(ctx, `UPDATE player_observations SET received_at=now()-interval '100 days' WHERE server_key=$1`, ServerKey(server))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Prune(ctx, 90); err != nil {
		t.Fatal(err)
	}
	page, err := s.List(ctx, Filter{Server: server, Limit: 100})
	if err != nil || page.Total != 31 {
		t.Fatal("retention deleted registry")
	}
}

func TestRegistryAliasConflictCorrectionConcurrentDecisionsAndThiefEvidence(t *testing.T) {
	ctx, s, server := registryFixture(t)
	base := time.Now().Add(-time.Minute).UTC().Truncate(time.Microsecond)
	first := fixtureObservation(server, "SharedAlias", base)
	first.Model = modelRef(1907)
	second := first
	second.ObservedAt = base.Add(time.Second)
	second.Model = modelRef(1908)
	if err := s.ApplyObservations(ctx, []Observation{first, second}); err != nil {
		t.Fatal(err)
	}
	rows := registryList(t, ctx, s, server)
	if len(rows) != 2 {
		t.Fatal("contradictory model was merged by name")
	}
	links, err := s.Links(ctx, server, "", "pending", 25, "")
	if err != nil || len(links.Items) != 1 {
		t.Fatal("alias conflict missing review candidate", err)
	}
	resolved, _ := s.ResolveNames(ctx, server, []string{"SharedAlias"})
	if len(resolved) != 0 {
		t.Fatal("ambiguous map alias resolved")
	}
	// Both submissions use the same revisions. Exactly one operator can decide.
	r := LinkRequest{CandidateID: links.Items[0].ID, CanonicalID: links.Items[0].CanonicalID, LinkedID: links.Items[0].LinkedID, Action: "reject", Reason: "fixture model conflict", Confirmed: true, Revision: links.Items[0].Revision}
	for _, row := range rows {
		if row.ID == r.CanonicalID {
			r.CanonicalRevision = row.Revision
		}
		if row.ID == r.LinkedID {
			r.LinkedRevision = row.Revision
		}
	}
	errorsChan := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := s.Decide(ctx, r, "fixture_operator"); errorsChan <- err }()
	}
	a, b := <-errorsChan, <-errorsChan
	if (a == nil) == (b == nil) {
		t.Fatalf("concurrent decisions %v %v", a, b)
	}
	id := rows[0].ID
	if err = s.Classify(ctx, id, Classification{Name: "SharedAlias", Type: "normal", Confirmed: true, Reason: "fixture classify", Revision: rows[0].Revision + 1}, "fixture_operator"); err != nil {
		t.Fatal(err)
	}
	record, _ := s.GetSource(ctx, id)
	if err = s.Classify(ctx, id, Classification{Name: "SharedAlias", Type: "job", Job: "hunter", Confirmed: true, Reason: "fixture correction", Revision: record.Revision}, "fixture_operator"); err != nil {
		t.Fatal(err)
	}
	record, _ = s.GetSource(ctx, id)
	if record.Name != nil || record.Resolved || record.Job == nil || *record.Job != "hunter" {
		t.Fatal("correction retained resolved normal name")
	}
	// Historical thief reports keep the original source ID; observer coordinates
	// are never promoted to the thief's position.
	sighting := newID()
	_, err = s.pool.Exec(ctx, `INSERT INTO thief_sightings(sighting_id,server_name,thief_name,region,x,y,position_source,origin,observed_at) VALUES($1::uuid,$2,'FixtureThief',25000,100,200,'observer','external',$3)`, sighting, server, base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(context.Background(), `DELETE FROM thief_sightings WHERE sighting_id=$1::uuid`, sighting)
	})
	if _, err = s.ImportThiefSightings(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportThiefSightings(ctx); err != nil {
		t.Fatal(err)
	}
	page, _ := s.List(ctx, Filter{Server: server, Name: "FixtureThief"})
	if len(page.Players) != 1 || page.Players[0].Location != nil || page.Players[0].Name != nil {
		t.Fatal("external report invented identity/location")
	}
	var count int
	s.pool.QueryRow(ctx, `SELECT count(*) FROM player_observations WHERE source='thief_sighting' AND source_ref=$1 AND pinned`, sighting).Scan(&count)
	if count != 1 {
		t.Fatal("thief replay/evidence pin failed")
	}
}

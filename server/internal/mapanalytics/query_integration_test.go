package mapanalytics

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
	"phmon/server/internal/mapprofile"
)

func TestHeatmapLayersFiltersAndResetPreserveCanonicalSources(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run map analytics integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	agentStore := agents.NewStore(pool)
	if err := agentStore.CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	server := "heat-" + credential.AgentID
	characterStore := characters.NewStore(pool)
	first, err := characterStore.Resolve(ctx, characters.Identity{Server: server, Name: "HeatA"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := characterStore.Resolve(ctx, characters.Identity{Server: server, Name: "HeatB"})
	if err != nil {
		t.Fatal(err)
	}
	firstSession, err := characterStore.ClaimSessionID(ctx, credential.AgentID, first, 1)
	if err != nil {
		t.Fatal(err)
	}
	secondSession, err := characterStore.ClaimSessionID(ctx, credential.AgentID, second, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM map_heatmap_resets WHERE lower(server_name)=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_position_samples WHERE lower(server_name)=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM mob_observation_samples WHERE lower(server_name)=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM activity_events WHERE lower(server_name)=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	var dbNow time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&dbNow); err != nil {
		t.Fatal(err)
	}
	now := dbNow.UTC().Truncate(time.Microsecond)
	region := 25273
	insertEvent := func(id, kind, category, characterID, sessionID string, x, y *float64, at time.Time) {
		t.Helper()
		_, err := pool.Exec(ctx, `INSERT INTO activity_events
			(event_id,schema_version,kind,category,agent_id,character_id,session_id,server_name,occurred_at,source,source_ref,region,x,y,payload)
			VALUES($1,1,$2,$3,$4,$5,$6,$7,$8,'phbot.callback','fixture',$9,$10,$11,'{}'::jsonb)`,
			id, kind, category, credential.AgentID, characterID, sessionID, server, at, region, x, y)
		if err != nil {
			t.Fatal(err)
		}
	}
	x10, y20 := 10.0, 20.0
	x120, y20b := 120.0, 20.0
	insertEvent("00000000-0000-4000-8000-000000009001", "character.died", "character", first, firstSession, &x10, &y20, now.Add(-20*time.Minute))
	insertEvent("00000000-0000-4000-8000-000000009002", "drop.item", "drop", first, firstSession, &x120, &y20b, now.Add(-15*time.Minute))
	insertEvent("00000000-0000-4000-8000-000000009003", "world.unique_spawned", "world", second, secondSession, &x120, &y20b, now.Add(-10*time.Minute))
	insertEvent("00000000-0000-4000-8000-000000009004", "item.acquired", "item", first, firstSession, &x120, &y20b, now.Add(-5*time.Minute))
	insertEvent("00000000-0000-4000-8000-000000009005", "character.died", "character", first, firstSession, nil, nil, now.Add(-4*time.Minute))

	_, err = pool.Exec(ctx, `INSERT INTO character_position_samples
		(agent_id,character_id,session_id,server_name,dataset_id,sampled_at,region,x,y)
		VALUES($1,$2,$3,$4,$5,$6,$7,10,20),($1,$2,$3,$4,$5,$8,$7,80,20),($1,$9,$10,$4,$5,$8,$7,400,20)`,
		credential.AgentID, first, firstSession, server, mapprofile.GreatestDatasetID, now.Add(-30*time.Minute), region,
		now.Add(-25*time.Minute), second, secondSession)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO mob_observation_samples
		(sample_id,agent_id,character_id,session_id,server_name,dataset_id,area_id,floor_id,region,sampled_at,sample_hash,sample_minute,observer_x,observer_y,observer_cell_x,observer_cell_y)
		VALUES
		('00000000-0000-4000-8000-000000009101',$1,$2,$3,$4,$5,'region:25273','unmapped',$6,$7,decode(repeat('01',32),'hex'),date_trunc('minute',$7::timestamptz),10,20,0,0),
		('00000000-0000-4000-8000-000000009102',$1,$8,$9,$4,$5,'region:25273','unmapped',$6,$10,decode(repeat('02',32),'hex'),date_trunc('minute',$10::timestamptz),12,18,0,0)`,
		credential.AgentID, first, firstSession, server, mapprofile.GreatestDatasetID, region, now.Add(-12*time.Minute), second, secondSession, now.Add(-11*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO mob_observations(sample_id,ordinal,monster_id,model_id,monster_type,region,x,y)
		VALUES
		('00000000-0000-4000-8000-000000009101',0,'m1',500,'General',$1,400,20),
		('00000000-0000-4000-8000-000000009102',0,'m2',501,'Party General',$1,410,25)`, region)
	if err != nil {
		t.Fatal(err)
	}

	store := NewStore(pool)
	base := Filter{Server: server, DatasetID: mapprofile.GreatestDatasetID, AreaID: "world", FloorID: "world",
		From: now.Add(-time.Hour), To: now.Add(time.Minute), Limit: 100}
	checkOne := func(layer, metric string) Result {
		t.Helper()
		filter := base
		filter.Layer = layer
		result, err := store.Heatmap(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != StatusAvailable || result.Metric != metric || len(result.Points) != 1 {
			t.Fatalf("layer %s result=%+v", layer, result)
		}
		return result
	}
	death := checkOne(LayerDeaths, "death_occurrences")
	checkOne(LayerDrops, "drop_occurrences")
	checkOne(LayerUniqueSightings, "unique_spawn_occurrences")

	movementFilter := base
	movementFilter.Layer = LayerPlayerMovement
	movementFilter.CharacterID = first
	movement, err := store.Heatmap(ctx, movementFilter)
	if err != nil || len(movement.Points) != 1 || movement.SourceRows != 2 {
		t.Fatalf("movement result=%+v err=%v", movement, err)
	}

	mobFilter := base
	mobFilter.Layer = LayerMobTypes
	mobFilter.MonsterType = "General"
	mob, err := store.Heatmap(ctx, mobFilter)
	if err != nil || len(mob.Points) != 1 || mob.Points[0].X < 350 {
		t.Fatalf("mob sighting should use monster coordinates: result=%+v err=%v", mob, err)
	}

	avgFilter := base
	avgFilter.Layer = LayerMobObserverAvg
	avg, err := store.Heatmap(ctx, avgFilter)
	if err != nil || avg.Status != StatusLimited || len(avg.Points) != 1 || avg.Points[0].Denominator != 2 || avg.Points[0].Numerator != 2 || avg.Points[0].X > 192 {
		t.Fatalf("observer-local denominator/position incorrect: result=%+v err=%v", avg, err)
	}

	densityFilter := base
	densityFilter.Layer = LayerMobDensity
	density, err := store.Heatmap(ctx, densityFilter)
	if err != nil || density.Status != StatusUnsupported || density.Reason != "observation_coverage_unverified" || len(density.Points) != 0 {
		t.Fatalf("true density should fail closed: result=%+v err=%v", density, err)
	}

	caveFilter := base
	caveFilter.Layer = LayerDeaths
	caveFilter.AreaID = "donwhang-stone-cave"
	caveFilter.FloorID = "1f"
	cave, err := store.Heatmap(ctx, caveFilter)
	if err != nil || cave.Status != StatusUnsupported || len(cave.Points) != 0 {
		t.Fatalf("unvalidated cave should fail closed: result=%+v err=%v", cave, err)
	}

	broadReset := ResetScope{Layer: LayerDeaths, Server: server, DatasetID: mapprofile.GreatestDatasetID,
		AreaID: "world", FloorID: "world", From: base.From, To: base.To}
	if _, err := store.Reset(ctx, broadReset); err == nil {
		t.Fatal("broad reset without explicit confirmation was accepted")
	}
	narrowReset := broadReset
	narrowReset.Region = &region
	narrowReset.CharacterID = first
	reset, err := store.Reset(ctx, narrowReset)
	if err != nil || reset.ResetID == "" || reset.BroadScope {
		t.Fatalf("narrow reset=%+v err=%v", reset, err)
	}
	deathAfter, err := store.Heatmap(ctx, Filter{Layer: LayerDeaths, Server: server, DatasetID: mapprofile.GreatestDatasetID,
		AreaID: "world", FloorID: "world", Region: &region, CharacterID: first, From: base.From, To: base.To, Limit: 100})
	if err != nil || len(deathAfter.Points) != 0 || deathAfter.SuppressedRows != 1 {
		t.Fatalf("narrow reset did not suppress only the matching death: %+v err=%v", deathAfter, err)
	}
	var canonicalDeaths int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM activity_events WHERE event_id='00000000-0000-4000-8000-000000009001'`).Scan(&canonicalDeaths); err != nil {
		t.Fatal(err)
	}
	if canonicalDeaths != 1 || death.SourceRows != 1 {
		t.Fatalf("heatmap reset deleted canonical history: before=%+v canonical=%d", death, canonicalDeaths)
	}
}

func TestNormalizeFilterRejectsInvalidBounds(t *testing.T) {
	now := time.Now().UTC()
	_, err := NormalizeFilter(Filter{Layer: LayerDeaths, Server: "greatest", DatasetID: mapprofile.GreatestDatasetID,
		AreaID: "world", FloorID: "world", From: now.Add(-32 * 24 * time.Hour), To: now, Limit: 1})
	if !errors.Is(err, ErrInvalidFilter) {
		t.Fatalf("oversized query window error=%v", err)
	}
}

func TestNormalizeFilterRejectsMobFiltersOnUnrelatedLayers(t *testing.T) {
	now := time.Now().UTC()
	for _, layer := range []string{LayerDeaths, LayerDrops, LayerUniqueSightings, LayerPlayerMovement} {
		filter := Filter{
			Layer: layer, Server: "greatest", DatasetID: mapprofile.GreatestDatasetID,
			AreaID: "world", FloorID: "world", From: now.Add(-time.Hour), To: now, Limit: 1,
			MonsterType: "General",
		}
		if _, err := NormalizeFilter(filter); !errors.Is(err, ErrInvalidFilter) {
			t.Fatalf("layer %s accepted an ignored mob filter: %v", layer, err)
		}
	}
}

func TestNormalizeFilterKeepsWorldBucketsAlignedToRegionTiles(t *testing.T) {
	now := time.Now().UTC()
	filter, err := NormalizeFilter(Filter{
		Layer: LayerDeaths, Server: "greatest", DatasetID: mapprofile.GreatestDatasetID,
		AreaID: "world", FloorID: "world", From: now.Add(-30 * 24 * time.Hour), To: now, Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filter.Resolution != 192 {
		t.Fatalf("30-day world query used non-tile-aligned resolution %v", filter.Resolution)
	}
	filter.Resolution = 100
	if _, err := NormalizeFilter(filter); !errors.Is(err, ErrInvalidFilter) {
		t.Fatalf("non-aligned world resolution was accepted: %v", err)
	}
}


func TestNormalizeFilterAcceptsSignedCaveRegionButRejectsZero(t *testing.T) {
	now := time.Now().UTC()
	region := -32767
	filter, err := NormalizeFilter(Filter{
		Layer: LayerDeaths, Server: "greatest", DatasetID: mapprofile.GreatestDatasetID,
		AreaID: "donwhang-stone-cave", FloorID: "1F", Region: &region,
		From: now.Add(-time.Hour), To: now, Limit: 10,
	})
	if err != nil {
		t.Fatalf("signed cave region rejected: %v", err)
	}
	result, err := NewStore(nil).Heatmap(context.Background(), filter)
	if err != nil || result.Status != StatusUnsupported || result.Reason != "coordinate_transform_unverified" {
		t.Fatalf("signed cave scope did not fail closed at transform boundary: result=%+v err=%v", result, err)
	}
	zero := 0
	filter.Region = &zero
	if _, err := NormalizeFilter(filter); !errors.Is(err, ErrInvalidFilter) {
		t.Fatalf("region zero accepted: %v", err)
	}
}


func TestMobFacetsHonorMobSightingsResetProjection(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run map analytics integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	if err := agents.NewStore(pool).CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	server := "facet-reset-" + credential.AgentID
	characterStore := characters.NewStore(pool)
	characterID, err := characterStore.Resolve(ctx, characters.Identity{Server: server, Name: "FacetReset"})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := characterStore.ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM map_heatmap_resets WHERE lower(server_name)=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM mob_observation_samples WHERE lower(server_name)=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	var now time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO mob_observation_samples
		(sample_id,agent_id,character_id,session_id,server_name,dataset_id,area_id,floor_id,region,sampled_at,sample_hash,sample_minute,observer_x,observer_y,observer_cell_x,observer_cell_y)
		VALUES
		('00000000-0000-4000-8000-000000009201',$1,$2,$3,$4,$5,'region:25273','unmapped',25273,$6,decode(repeat('11',32),'hex'),date_trunc('minute',$6::timestamptz),10,20,0,0),
		('00000000-0000-4000-8000-000000009202',$1,$2,$3,$4,$5,'region:25273','unmapped',25273,$7,decode(repeat('12',32),'hex'),date_trunc('minute',$7::timestamptz),20,20,1,0)`,
		credential.AgentID, characterID, sessionID, server, mapprofile.GreatestDatasetID,
		now.Add(-10*time.Minute), now.Add(-9*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO mob_observations(sample_id,ordinal,monster_id,model_id,monster_type,region,x,y)
		VALUES
		('00000000-0000-4000-8000-000000009201',0,'g',500,'General',25273,100,100),
		('00000000-0000-4000-8000-000000009202',0,'c',501,'Champion',25273,120,100)`)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	from, to := now.Add(-time.Hour), now.Add(time.Minute)
	before, err := store.MobFacets(ctx, Filter{
		Layer: LayerMobTypes, Server: server, DatasetID: mapprofile.GreatestDatasetID,
		AreaID: "world", FloorID: "world", From: from, To: to, Limit: 100,
	}, 100)
	if err != nil || len(before) != 2 {
		t.Fatalf("facet fixture unavailable before reset: facets=%+v err=%v", before, err)
	}
	region := 25273
	_, err = store.Reset(ctx, ResetScope{
		Layer: LayerMobTypes, Server: server, DatasetID: mapprofile.GreatestDatasetID,
		AreaID: "world", FloorID: "world", Region: &region, MonsterType: "General",
		From: from, To: to,
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.MobFacets(ctx, Filter{
		Layer: LayerMobTypes, Server: server, DatasetID: mapprofile.GreatestDatasetID,
		AreaID: "world", FloorID: "world", From: from, To: to, Limit: 100,
	}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].MonsterType != "Champion" || after[0].Count != 1 {
		t.Fatalf("mob facets ignored reset projection: %+v", after)
	}
}

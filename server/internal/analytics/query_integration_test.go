package analytics_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/analytics"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
)

func TestAnalyticsQueriesUseCanonicalEventsAndStableScope(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run analytics integration tests")
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
	server := "analytics-" + credential.AgentID[:8]
	charactersStore := characters.NewStore(pool)
	aria, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: "Aria"})
	if err != nil {
		t.Fatal(err)
	}
	bard, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: "Bard"})
	if err != nil {
		t.Fatal(err)
	}
	ariaSession, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, aria, 1)
	if err != nil {
		t.Fatal(err)
	}
	bardSession, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, bard, 1)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	// This fixture asserts one daily bucket for three hours of activity. Keep
	// those rows on the same UTC day when the suite runs just after midnight.
	if now.Hour() < 4 {
		now = now.Truncate(24 * time.Hour).Add(-time.Hour)
	}
	level := int(100)
	xp, maxXP, sp, gold := int64(500), int64(1000), int64(12), int64(5000)
	if err := charactersStore.SnapshotSession(ctx, credential.AgentID, aria, 1, ariaSession, characters.State{Level: &level, CurrentEXP: &xp, MaxEXP: &maxXP, SP: &sp, Gold: &gold}); err != nil {
		t.Fatal(err)
	}
	if err := charactersStore.SnapshotSession(ctx, credential.AgentID, bard, 1, bardSession, characters.State{Level: &level, CurrentEXP: &xp, MaxEXP: &maxXP, SP: &sp, Gold: &gold}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, _ = pool.Exec(cleanCtx, `DELETE FROM activity_events WHERE agent_id=$1::uuid`, credential.AgentID)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM characters WHERE server_key=$1`, strings.ToLower(server))
		_, _ = pool.Exec(cleanCtx, `DELETE FROM agents WHERE agent_id=$1::uuid`, credential.AgentID)
	})
	insert := func(characterID, sessionID, kind, category string, at time.Time, zone string, region int, model *int64, code, payload string) {
		t.Helper()
		var id string
		if err := pool.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&id); err != nil {
			t.Fatal(err)
		}
		_, err := pool.Exec(ctx, `INSERT INTO activity_events(event_id,schema_version,kind,category,agent_id,character_id,session_id,server_name,occurred_at,source,source_ref,region,zone_name,item_model,item_code,payload)
VALUES($1::uuid,1,$2,$3,$4::uuid,$5::uuid,$6::uuid,$7,$8,'phmon.test','analytics-fixture',$9,$10,$11,NULLIF($12,''),$13::jsonb)`, id, kind, category, credential.AgentID, characterID, sessionID, server, at, region, zone, model, code, payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	insert(aria, ariaSession, "character.died", "character", now.Add(-3*time.Hour), "Jangan", 1, nil, "", `{"cause":"monster"}`)
	insert(aria, ariaSession, "character.died", "character", now.Add(-2*time.Hour), "Jangan", 1, nil, "", `{"cause":"monster"}`)
	insert(bard, bardSession, "character.died", "character", now.Add(-time.Hour), "Donwhang", 2, nil, "", `{"cause":"unknown"}`)
	model := int64(77)
	modelWithoutDegree := int64(78)
	insert(aria, ariaSession, "drop.rare", "drop", now.Add(-2*time.Hour), "Jangan", 1, &model, "ITEM_RARE", `{"model":77,"item":{"name":"Fixture Blade","plus":3}}`)
	insert(aria, ariaSession, "drop.rare", "drop", now.Add(-90*time.Minute), "Jangan", 1, &modelWithoutDegree, "ITEM_RARE_NO_DEGREE", `{"model":78,"item":{"name":"Fixture Hood","plus":2}}`)
	var ownedGainID, transferID string
	if err := pool.QueryRow(ctx, `SELECT gen_random_uuid()::text,gen_random_uuid()::text`).Scan(&ownedGainID, &transferID); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO activity_events(event_id,schema_version,kind,category,agent_id,character_id,session_id,server_name,occurred_at,source,source_ref,region,zone_name,item_model,item_code,item_drop_class,item_drop_class_version,payload)
VALUES($1::uuid,1,'item.acquired','item',$2::uuid,$3::uuid,$4::uuid,$5,$6,'phbot.state_diff','item_container',1,'Jangan',77,'ITEM_RARE','rare','fixture-v1',
'{"item":{"model":77,"servername":"ITEM_RARE"},"quantity_delta":2,"destination_container":{"type":"inventory","slot":4},"acquisition_method":"unknown"}'::jsonb),
($7::uuid,1,'item.transferred','item',$2::uuid,$3::uuid,$4::uuid,$5,$6+interval '1 second','phbot.state_diff','item_container',1,'Jangan',77,'ITEM_RARE','rare','fixture-v1',
'{"item":{"model":77,"servername":"ITEM_RARE"},"quantity_delta":1,"destination_container":{"type":"pets","id":"17","slot":2}}'::jsonb)`, ownedGainID, credential.AgentID, aria, ariaSession, server, now.Add(-time.Hour), transferID)
	if err != nil {
		t.Fatal(err)
	}
	insert(aria, ariaSession, "academy.member_joined", "academy", now.Add(-time.Hour), "Jangan", 1, nil, "", `{"academy_id":7,"member_id":"peer-1","member":{"name":"Peer"}}`)
	insert(bard, bardSession, "academy.member_left", "academy", now.Add(-30*time.Minute), "Donwhang", 2, nil, "", `{"academy_id":7,"member_id":"peer-2","member":{"name":"Peer 2"}}`)
	insert(aria, ariaSession, "alchemy.attempt", "alchemy", now.Add(-15*time.Minute), "Jangan", 1, nil, "", `{"slot":1,"success":true,"plus":4}`)
	insert(aria, ariaSession, "alchemy.attempt", "alchemy", now.Add(-10*time.Minute), "Jangan", 1, nil, "", `{"slot":1,"success":null,"plus":null}`)
	for index, observer := range []string{aria, bard} {
		session := ariaSession
		if observer == bard {
			session = bardSession
		}
		_, err := pool.Exec(ctx, `INSERT INTO guild_gold_samples(server_key,guild_key,observer_character_id,session_id,resource_revision,sampled_at,gold)
VALUES($1,'shared-guild',$2::uuid,$3::uuid,1,$4,$5)`, strings.ToLower(server), observer, session, now.Add(-2*time.Hour), int64(5000+index*2000))
		if err != nil {
			t.Fatal(err)
		}
	}
	for index, seconds := range []int{110, 90, 70, 50} {
		at := now.Add(-time.Duration(seconds) * time.Second)
		expValue := int64(100 + index*100)
		spValue := int64(10 + index*5)
		goldValue := int64(1000 + []int{0, 10, -10, -20}[index])
		_, err := pool.Exec(ctx, `INSERT INTO character_metric_samples(character_id,session_id,agent_id,sample_schema_version,server_key,sampled_at,level,current_exp,max_exp,sp,gold,region,zone_name,botting,dead)
VALUES($1::uuid,$2::uuid,$3::uuid,1,$4,$5,100,$6,1000,$7,$8,1,'Jangan',true,false)`, aria, ariaSession, credential.AgentID, strings.ToLower(server), at, expValue, spValue, goldValue)
		if err != nil {
			t.Fatal(err)
		}
	}
	from, to := now.Add(-24*time.Hour), now.Add(time.Minute)
	store := analytics.NewStore(pool)
	store.SetItemTaxonomy(
		func(server string, model int64, code string) (string, string, bool) {
			if server == strings.ToLower(server) && model == 77 && code == "ITEM_RARE" {
				return "Accessory", "10", true
			}
			if model == 78 && code == "ITEM_RARE_NO_DEGREE" {
				return "Accessory", "", true
			}
			return "", "", false
		},
		func(server, itemType, degree string) ([]int64, bool) {
			if server != strings.ToLower(server) {
				return nil, false
			}
			if (itemType == "" || itemType == "Accessory") && (degree == "" || degree == "10") {
				return []int64{77}, true
			}
			return []int64{}, true
		},
		func(server string) []analytics.TaxonomyOption {
			return []analytics.TaxonomyOption{{Type: "Accessory", Degree: "10"}}
		},
	)
	store.SetItemDetailsResolver(func(_ string, _ *int64, _ string, item map[string]any) (map[string]any, map[string]any) {
		metadata := map[string]any{"name": "Fixture item", "icon_url": "/game-assets/fixture.png"}
		return metadata, item
	})
	deaths, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewDeaths, From: from, To: to, Timezone: "Europe/Amsterdam", Bucket: "hour", GroupBy: "location", PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if deaths.Total != "3" || deaths.Filter.Bucket != "hour" || len(deaths.TimeSeries) != 3 || len(deaths.Occurrences) != 1 || deaths.NextCursor == "" {
		t.Fatalf("death query totals, buckets, or cursor incorrect: %+v", deaths)
	}
	if deaths.Breakdown[0].Label != "Region 1 · Jangan" || deaths.Breakdown[0].Value != "2" {
		t.Fatalf("location breakdown incorrect: %+v", deaths.Breakdown)
	}
	if deaths.Summary[1].Number == nil || *deaths.Summary[1].Number < 0.75 || *deaths.Summary[1].Number > 1.5 {
		t.Fatalf("average denominator should include both registered characters and local calendar days: %+v", deaths.Summary[1])
	}
	secondPage, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewDeaths, From: from, To: to, Timezone: "Europe/Amsterdam", GroupBy: "character", PageSize: 1, Cursor: deaths.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage.Occurrences) != 1 || secondPage.Total != "3" {
		t.Fatalf("cursor changed totals or did not advance: %+v", secondPage)
	}

	drops, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewRareDrops, From: from, To: to, Timezone: "UTC", GroupBy: "item"})
	if err != nil {
		t.Fatal(err)
	}
	if drops.Total != "2" || len(drops.Breakdown) != 2 || len(drops.Taxonomy) != 1 || drops.Taxonomy[0].Label != "Accessory" || drops.TaxonomyKnown != 2 || drops.TaxonomyUnknown != 0 || drops.TaxonomyDegreeUnknown != 1 {
		t.Fatalf("rare drop query incorrect: %+v", drops)
	}
	if len(drops.Occurrences) != 2 || drops.Occurrences[0].ItemDetails["name"] != "Fixture Hood" || drops.Occurrences[0].ItemMetadata["icon_url"] != "/game-assets/fixture.png" {
		t.Fatalf("drop evidence did not carry shared item detail metadata: %+v", drops.Occurrences)
	}
	owned, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewRareDrops, DropSource: "owned_gains", From: from, To: to, Timezone: "UTC", GroupBy: "item"})
	if err != nil || owned.Total != "1" || len(owned.Occurrences) != 1 || owned.Occurrences[0].ID != ownedGainID {
		t.Fatalf("owned-gain population included world observations or transfers: snapshot=%+v err=%v", owned, err)
	}
	quantitySeen := false
	for _, metric := range owned.Summary {
		if metric.Key == "owned_gain_quantity" {
			quantitySeen = true
			if metric.Value != "2" {
				t.Fatalf("owned-gain quantity should use the accepted delta, got %+v", metric)
			}
		}
	}
	if !quantitySeen {
		t.Fatal("owned-gain quantity denominator was not included in its summary")
	}
	filteredDrops, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewRareDrops, From: from, To: to, Timezone: "UTC", GroupBy: "type", ItemType: "Accessory", ItemDegree: "10"})
	if err != nil || filteredDrops.Total != "1" || len(filteredDrops.Breakdown) != 1 || filteredDrops.Breakdown[0].Label != "Accessory" {
		t.Fatalf("profile-aware item type/degree filtering failed: snapshot=%+v err=%v", filteredDrops, err)
	}
	degreeBreakdown, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewRareDrops, From: from, To: to, Timezone: "UTC", GroupBy: "degree"})
	if err != nil || len(degreeBreakdown.Breakdown) != 2 || degreeBreakdown.Breakdown[0].Label != "10" || degreeBreakdown.Breakdown[1].Label != "Unknown" || degreeBreakdown.Breakdown[1].Value != "1" {
		t.Fatalf("unknown item degree was hidden or assigned a fabricated value: snapshot=%+v err=%v", degreeBreakdown, err)
	}
	academy, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewAcademy, From: from, To: to, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if academy.Total != "2" || academy.Summary[1].Value != "1" || academy.Summary[2].Value != "1" || academy.Summary[3].Value != "1" || academy.Summary[5].Status != "unsupported" {
		t.Fatalf("academy evidence was overstated: %+v", academy)
	}
	academyGroups, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewAcademy, From: from, To: to, Timezone: "UTC", GroupBy: "academy"})
	if err != nil || len(academyGroups.Breakdown) != 1 || academyGroups.Breakdown[0].Label != "Academy 7" {
		t.Fatalf("academy membership context was not grouped by verified ID: snapshot=%+v err=%v", academyGroups, err)
	}
	alchemy, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewAlchemy, From: from, To: to, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if alchemy.Total != "2" || alchemy.Summary[1].Value != "1" || alchemy.Summary[3].Value != "1" || alchemy.Summary[5].Value != "4" {
		t.Fatalf("alchemy outcome query incorrect: %+v", alchemy)
	}
	if len(alchemy.TimeSeries) != 1 || alchemy.TimeSeries[0].Value != "100.00" || !strings.Contains(alchemy.TimeSeries[0].Detail, "1 successes / 1 known outcomes") {
		t.Fatalf("alchemy weekday share omitted its observed denominator: %+v", alchemy.TimeSeries)
	}
	progress, err := store.Query(ctx, analytics.Filter{Server: server, CharacterID: aria, View: analytics.ViewPerformance, From: now.Add(-23 * time.Hour), To: now.Add(time.Minute), Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if progress.Performance == nil || progress.Performance.Character != "Aria" || progress.Performance.Rates["xp"].PerHour < 5900 || progress.Performance.Rates["sp"].PerHour < 290 || progress.Performance.Rates["gold"].PerHour > 0 || progress.Performance.NormalDrops24h != 0 || progress.Performance.RareDrops24h != 2 {
		t.Fatalf("performance aggregation lost signed balance rates or recorded event totals: %+v", progress.Performance)
	}
	if progress.Performance.Training == nil || len(progress.Performance.Training) != 1 || progress.Performance.Training[0].CoveredSecond < 50 || progress.Performance.BottingSeconds < 50 {
		t.Fatalf("performance coverage did not use verified adjacent states: %+v", progress.Performance)
	}
	var daily, weekly bool
	for _, period := range progress.Performance.Periods {
		if period.Granularity == "day" {
			daily = true
			if period.XPGain == nil || *period.XPGain != "300" || period.SPNet == nil || *period.SPNet != "15" || period.GoldNet == nil || *period.GoldNet != "-20" || period.Deaths != 2 || period.RareDrops != 2 {
				t.Fatalf("daily summary lost eligible deltas, denominators or canonical counts: %+v", period)
			}
		}
		if period.Granularity == "week" {
			weekly = true
		}
	}
	if !daily || !weekly || len(progress.Performance.Locations) != 1 || progress.Performance.Locations[0].Region == nil || *progress.Performance.Locations[0].Region != 1 || progress.Performance.Locations[0].XPGain == nil || *progress.Performance.Locations[0].XPGain != "300" {
		t.Fatalf("weekly summary or same-location gain comparison is missing: %+v", progress.Performance)
	}
	characterEconomy, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewEconomy, From: from, To: to, Timezone: "UTC", BalanceScope: "characters"})
	if err != nil || len(characterEconomy.TimeSeries) == 0 || characterEconomy.Summary[0].Status != "available" {
		t.Fatalf("character balance history or signed daily delta was not available: snapshot=%+v err=%v", characterEconomy, err)
	}
	guildEconomy, err := store.Query(ctx, analytics.Filter{Server: server, View: analytics.ViewEconomy, From: from, To: to, Timezone: "UTC", BalanceScope: "guild_storage"})
	if err != nil || len(guildEconomy.TimeSeries) != 1 || guildEconomy.TimeSeries[0].Value == "12000" || guildEconomy.Summary[0].Status != "available" {
		t.Fatalf("guild gold observers were summed or omitted: snapshot=%+v err=%v", guildEconomy, err)
	}
	if _, err := store.ResetRateWindow(ctx, aria, "aria", "operator", "analytics-rate-reset-0001"); !errors.Is(err, analytics.ErrRateResetNameMismatch) {
		t.Fatalf("rate reset accepted a non-exact typed character name: %v", err)
	}
	resetAt, err := store.ResetRateWindow(ctx, aria, "Aria", "operator", "analytics-rate-reset-0001")
	if err != nil {
		t.Fatal(err)
	}
	replayedResetAt, err := store.ResetRateWindow(ctx, aria, "Aria", "operator", "analytics-rate-reset-0001")
	if err != nil || !resetAt.Equal(replayedResetAt) {
		t.Fatalf("rate reset retry was not idempotent: first=%s retry=%s error=%v", resetAt, replayedResetAt, err)
	}
}

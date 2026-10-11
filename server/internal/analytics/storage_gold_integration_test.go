package analytics_test

import (
	"context"
	"encoding/json"
	"errors"
	"phmon/server/internal/analytics"
	"phmon/server/internal/characters"
	"phmon/server/internal/resources"
	"testing"
	"time"
)

func TestStorageGoldPersistsWithPairedSamplesAndFreshness(t *testing.T) {
	f := newReviewFixture(t, 1)
	ctx := context.Background()
	carried, bank := int64(1000), int64(4000)
	chars := characters.NewStore(f.pool)
	state := characters.State{Gold: &carried, StorageGold: &bank}
	if err := chars.SnapshotSession(ctx, f.agent, f.chars[0], 1, f.sessions[0], state); err != nil {
		t.Fatal(err)
	}
	var savedBank *int64
	if err := f.pool.QueryRow(ctx, `SELECT storage_gold FROM character_metric_samples WHERE character_id=$1`, f.chars[0]).Scan(&savedBank); err != nil || savedBank == nil || *savedBank != 4000 {
		t.Fatalf("paired bank not persisted: %v %v", savedBank, err)
	}
	state.StorageGold = nil
	if sampled, err := chars.UpdateSessionWithAnalyticsSample(ctx, f.agent, f.chars[0], 1, f.sessions[0], state); err != nil || !sampled {
		t.Fatalf("unknown bank must break history immediately: %v %v", sampled, err)
	}
	state.StorageGold = &bank
	if sampled, err := chars.UpdateSessionWithAnalyticsSample(ctx, f.agent, f.chars[0], 1, f.sessions[0], state); err != nil || !sampled {
		t.Fatalf("first bank must establish a new baseline: %v %v", sampled, err)
	}
	if _, err := chars.UpdateSessionWithAnalyticsSample(ctx, f.agent, f.chars[0], 2, f.sessions[0], state); !errors.Is(err, characters.ErrNotFound) {
		t.Fatalf("stale generation admitted: %v", err)
	}
	store := resources.NewStore(f.pool)
	snap := resources.Snapshot{Revision: 1, Full: true, Resources: map[string]json.RawMessage{"storage": json.RawMessage(`{"availability":"observed","gold":4000,"capacity":0,"used_slots":0,"slots":[]}`)}}
	if err := store.Apply(ctx, f.agent, f.chars[0], 1, 1, f.sessions[0], snap); err != nil {
		t.Fatal(err)
	}
	view, err := chars.Get(ctx, f.chars[0])
	if err != nil {
		t.Fatal(err)
	}
	if view.StorageGold == nil || *view.StorageGold != 4000 || view.StorageGoldObservedAt == nil || view.StorageGoldAvailability == nil || *view.StorageGoldAvailability != "observed" {
		t.Fatalf("bank view: %+v", view)
	}
	snap = resources.Snapshot{Revision: 2, BaseRevision: 1, Resources: map[string]json.RawMessage{"storage": json.RawMessage(`{"availability":"not_observed"}`)}}
	if err := store.Apply(ctx, f.agent, f.chars[0], 1, 2, f.sessions[0], snap); err != nil {
		t.Fatal(err)
	}
	view, err = chars.Get(ctx, f.chars[0])
	if err != nil {
		t.Fatal(err)
	}
	if view.StorageGold == nil || *view.StorageGold != 4000 || *view.StorageGoldAvailability != "stale" {
		t.Fatalf("last bank lost: %+v", view)
	}
	if err := chars.EndSession(ctx, f.agent, f.chars[0], 1, f.sessions[0], "left"); err != nil {
		t.Fatal(err)
	}
	view, err = chars.Get(ctx, f.chars[0])
	if err != nil || view.StorageGold == nil || *view.StorageGoldAvailability != "stale" {
		t.Fatalf("offline bank lost: %+v %v", view, err)
	}
}

func TestCombinedBankAnalyticsAndTransfers(t *testing.T) {
	f := newReviewFixture(t, 2)
	ctx := context.Background()
	start := f.now.Add(-2 * time.Minute)
	for i := 0; i < 3; i++ {
		carried, bank := int64(1000), int64(4000)
		if i > 0 {
			carried, bank = 500, 4500
		}
		_, err := f.pool.Exec(ctx, `INSERT INTO character_metric_samples(character_id,session_id,agent_id,server_key,sampled_at,gold,storage_gold,region,zone_name) VALUES($1,$2,$3,$4,$5,$6,$7,25000,'Bank fixture')`, f.chars[0], f.sessions[0], f.agent, f.server, start.Add(time.Duration(i)*30*time.Second), carried, bank)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := f.pool.Exec(ctx, `INSERT INTO character_metric_samples(character_id,session_id,agent_id,server_key,sampled_at,gold) VALUES($1,$2,$3,$4,$5,$6)`, f.chars[1], f.sessions[1], f.agent, f.server, start.Add(time.Duration(i)*30*time.Second), 9000+i*1000); err != nil {
			t.Fatal(err)
		}
	}
	economy := f.query(t, analytics.ViewEconomy)
	if len(economy.Breakdown) != 1 || economy.Breakdown[0].Value != "5000" || economy.Breakdown[0].CharacterID != f.chars[0] {
		t.Fatalf("balance scope: %+v", economy)
	}
	performance := f.query(t, analytics.ViewPerformance)
	rate := performance.Performance.Rates["gold"]
	if !rate.HasRate || rate.DeltaExact != "0" || rate.PerHour != 0 {
		t.Fatalf("bank transfer changed wealth: %+v", rate)
	}
	if len(performance.Performance.Sessions) != 1 || performance.Performance.Sessions[0].GoldNet == nil || *performance.Performance.Sessions[0].GoldNet != "0" || performance.Performance.Sessions[0].GoldSeconds != 60 {
		t.Fatalf("session net counted transfer: %+v", performance.Performance.Sessions)
	}
	for _, period := range performance.Performance.Periods {
		if period.GoldNet != nil && *period.GoldNet != "0" {
			t.Fatalf("period counted transfer: %+v", period)
		}
	}
	for _, location := range performance.Performance.Locations {
		if location.GoldNet != nil && *location.GoldNet != "0" {
			t.Fatalf("location counted transfer: %+v", location)
		}
	}
	query := analytics.Filter{View: analytics.ViewEconomy, Server: f.server, CharacterID: f.chars[1], From: start.Add(-time.Second), To: f.now, Timezone: "UTC"}
	unknown, err := analytics.NewStore(f.pool).Query(ctx, query)
	if err != nil || len(unknown.Breakdown) != 0 || unknown.Status != "empty" {
		t.Fatalf("legacy bank fabricated: %+v %v", unknown, err)
	}
	query.View = analytics.ViewPerformance
	legacy, err := analytics.NewStore(f.pool).Query(ctx, query)
	if err != nil || legacy.Performance == nil || legacy.Performance.Rates["gold"].HasRate || len(legacy.Performance.Sessions) != 1 || legacy.Performance.Sessions[0].GoldNet != nil || legacy.Performance.Sessions[0].GoldSeconds != 0 {
		t.Fatalf("legacy session bank income fabricated: %+v %v", legacy.Performance, err)
	}
	query.CharacterID = f.chars[0]
	query.Server = "other-server"
	if _, err := analytics.NewStore(f.pool).Query(ctx, query); !errors.Is(err, analytics.ErrCharacterScope) {
		t.Fatalf("cross-server bank leaked: %v", err)
	}
}

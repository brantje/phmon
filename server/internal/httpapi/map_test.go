package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/events"
	"phmon/server/internal/mapprofile"
	"phmon/server/internal/mobs"
	"phmon/server/internal/resources"
)

func TestMapProfileAPIReturnsTileReferencesAndHonestValidation(t *testing.T) {
	metadata := &resources.ItemMetadata{Servers: map[string]string{"greatest": mapprofile.GreatestDatasetID}}
	resourceStore := resources.NewStore(nil)
	resourceStore.SetItemMetadata(metadata)
	registry := agentdomain.NewRegistry()
	handler := New(Dependencies{Agents: newFakeAgentStore(), Registry: registry, Resources: resourceStore})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/map/profile?server=greatest", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("profile status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var profile map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	if profile["dataset_id"] != mapprofile.GreatestDatasetID || profile["coordinate_transform_status"] != "outdoor-region-grid" || profile["command_z_evidence_status"] != "unverified" {
		t.Fatalf("profile omits selected dataset or overstates placement evidence: %v", profile)
	}
	if mappings, ok := profile["region_mappings"].([]any); !ok || len(mappings) != 4 {
		t.Fatalf("documented outdoor region mappings missing: %v", profile["region_mappings"])
	}
	tiles, ok := profile["tiles"].(map[string]any)
	if !ok || tiles["tile_url_format"] != "/game-assets/minimap/{x}x{y}.png" || tiles["tile_count"] != float64(5118) {
		t.Fatalf("tile catalog reference missing: %v", profile["tiles"])
	}
}

func TestPackagedGreatestDatasetEnablesMapProfile(t *testing.T) {
	metadata, err := resources.LoadItemMetadata("../../game-data")
	if err != nil {
		t.Fatal(err)
	}
	dataset, ok := metadata.DatasetForServer("greatest")
	if !ok || dataset != mapprofile.GreatestDatasetID {
		t.Fatalf("packaged Greatest dataset=%q, map profile expects %q", dataset, mapprofile.GreatestDatasetID)
	}
	profile, err := mapprofile.ForServer("greatest", dataset)
	if err != nil || profile.TileCatalog.Status != "available-for-inspection" || len(profile.CaveFloors) != 17 {
		t.Fatalf("packaged dataset did not enable all cave floors: status=%q floors=%d err=%v", profile.ProfileStatus, len(profile.CaveFloors), err)
	}
}

func TestMapLiveSubscriptionRequiresScopedServerAndFloor(t *testing.T) {
	base := liveClientMessage{ProtocolVersion: liveProtocolVersion, SubscriptionID: "map", Revision: 1, Stream: "map",
		Filter: liveFilter{Server: "greatest", Area: "world", Floor: "world", Region: 25273}}
	if _, ok := validateLiveSubscription(base); !ok {
		t.Fatal("valid world map subscription rejected")
	}
	base.Filter.EventID = "00000000-0000-4000-8000-000000000001"
	if _, ok := validateLiveSubscription(base); !ok {
		t.Fatal("valid linked event ID rejected")
	}
	base.Filter.EventID = "not-an-event-id"
	if _, ok := validateLiveSubscription(base); ok {
		t.Fatal("invalid linked event ID accepted")
	}
	base.Filter.EventID = ""
	base.Filter.Server = ""
	if _, ok := validateLiveSubscription(base); ok {
		t.Fatal("unscoped map subscription accepted")
	}
	base.Filter.Server = "greatest"
	base.Filter.Area = "job-temple"
	base.Filter.Floor = "1F"
	base.Filter.Region = -32752
	if _, ok := validateLiveSubscription(base); !ok {
		t.Fatal("signed cave region subscription rejected")
	}
}

func TestCaveMonsterSnapshotsUseObserverZAndRetainObservedEmpty(t *testing.T) {
	profile, err := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	z := -9.0
	wrongFloorZ := 100.0
	snapshots := []mobs.LiveSnapshot{
		{Region: -32767, ObserverZ: &z, Status: "observed", Monsters: []mobs.Monster{
			{ID: "same-floor", Region: 32767, X: -24300, Y: 20},
			{ID: "other-floor", Region: 32767, X: -24300, Y: 20, Z: &wrongFloorZ},
		}},
		{Region: -32767, ObserverZ: &z, Status: "observed", Monsters: []mobs.Monster{}},
	}
	got := filterCaveMonsterSnapshots(profile, snapshots, "donwhang-stone-cave", "1F")
	if len(got) != 2 || len(got[0].Monsters) != 1 || got[0].Monsters[0].ID != "same-floor" || len(got[1].Monsters) != 0 {
		t.Fatalf("cave floor feed lost observer-scoped sightings or empty status: %+v", got)
	}
}

type mapEventListerStub struct {
	events  []events.Event
	filters []events.Filter
}

func (stub *mapEventListerStub) List(_ context.Context, filter events.Filter) (events.Page, error) {
	stub.filters = append(stub.filters, filter)
	page := events.Page{Events: make([]events.Event, 0)}
	for _, event := range stub.events {
		if filter.Server != "" && event.Server != filter.Server || filter.EventID != "" && event.ID != filter.EventID ||
			filter.Kind != "" && event.Kind != filter.Kind || filter.Category != "" && event.Category != filter.Category ||
			filter.Region != nil && (event.Region == nil || *event.Region != *filter.Region) ||
			filter.RequireMapPosition && (event.Region == nil || event.X == nil || event.Y == nil) ||
			filter.From != nil && event.OccurredAt.Before(*filter.From) || filter.To != nil && !event.OccurredAt.Before(*filter.To) {
			continue
		}
		page.Events = append(page.Events, event)
	}
	if len(page.Events) > filter.Limit {
		page.Events = page.Events[:filter.Limit]
	}
	return page, nil
}

func TestMapActivityFiltersDeathAndDropBeforeCombinedLimitAndResolvesLinkedEvent(t *testing.T) {
	baseTime := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	region := 25273
	all := make([]events.Event, 0, 272)
	for index := 0; index < 150; index++ {
		all = append(all, events.Event{ID: testUUID(index + 1), Server: "greatest", Kind: "character.level_up",
			Category: "character", Region: &region, OccurredAt: baseTime.Add(time.Duration(index) * time.Second)})
	}
	for index := 0; index < 60; index++ {
		x, y := float64(index), float64(index+1)
		all = append(all, events.Event{ID: testUUID(index + 1001), Server: "greatest", Kind: events.DeathKind,
			Category: events.DeathCategory, Region: &region, X: &x, Y: &y, OccurredAt: baseTime.Add(-time.Duration(index) * time.Second)})
		all = append(all, events.Event{ID: testUUID(index + 2001), Server: "greatest", Kind: "drop.item",
			Category: "drop", Region: &region, X: &x, Y: &y, OccurredAt: baseTime.Add(-time.Duration(index+60) * time.Second)})
	}
	linkedEventID := "00000000-0000-4000-8000-000000000099"
	all = append(all, events.Event{ID: linkedEventID, Server: "greatest", Kind: "character.level_up",
		Category: "character", Region: &region, OccurredAt: baseTime.Add(-48 * time.Hour)})
	store := &mapEventListerStub{events: all}
	from, to := baseTime.Add(-time.Hour), baseTime.Add(time.Minute)
	activity, err := collectMapActivity(context.Background(), store, "greatest", from, to, region, linkedEventID)
	if err != nil {
		t.Fatal(err)
	}
	if len(activity) != 100 {
		t.Fatalf("expected a 100-event map snapshot with the linked event reserved, got %d", len(activity))
	}
	for _, event := range activity[:99] {
		if event.Kind != events.DeathKind && event.Category != "drop" {
			t.Fatalf("unrelated activity escaped map event filters: %+v", event)
		}
	}
	if activity[99].ID != linkedEventID {
		t.Fatalf("exact linked event was not retained outside the rolling range: %+v", activity[99])
	}
	if len(store.filters) != 3 || store.filters[0].Server != "greatest" || store.filters[0].Kind != events.DeathKind ||
		store.filters[1].Server != "greatest" || store.filters[1].Category != "drop" ||
		store.filters[2].Server != "greatest" || store.filters[2].EventID != linkedEventID {
		t.Fatalf("map queried without death/drop/exact-ID filters: %+v", store.filters)
	}
	if store.filters[0].Region == nil || *store.filters[0].Region != region || !store.filters[0].RequireMapPosition ||
		store.filters[1].Region == nil || *store.filters[1].Region != region || !store.filters[1].RequireMapPosition {
		t.Fatalf("map region/position constraints were applied after the event limit: %+v", store.filters)
	}
}

func testUUID(value int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", value)
}

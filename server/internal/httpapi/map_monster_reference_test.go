package httpapi

import (
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"

	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/mapprofile"
	"phmon/server/internal/resources"
)

func TestMonsterReferenceAPIIsDatasetScopedFilteredAndPaginated(t *testing.T) {
	level := 20
	metadata := &resources.ItemMetadata{
		Servers: map[string]string{"greatest": mapprofile.GreatestDatasetID, "other": "gamedata-other"},
		MonsterReferences: map[string]*resources.MonsterReference{mapprofile.GreatestDatasetID: {
			DatasetID: mapprofile.GreatestDatasetID, Status: "partial",
			Definitions: []resources.MonsterDefinition{{ModelID: 1954, Code: "MOB_CH_TIGERWOMAN", Name: "Tiger Girl", Level: &level, Enabled: true}},
			Areas:       []resources.MonsterGuideArea{{ModelID: 1954, Code: "MOB_CH_TIGERWOMAN", Group: "field", Precision: "guide-grid-cell", Cells: []resources.MonsterGuideCell{{X: 168, Y: 96, Width: 4, Height: 4}}}},
			Points: []resources.MonsterReferencePoint{
				{ModelID: 1954, Code: "MOB_CH_TIGERWOMAN", Region: 24744, RawX: 100, RawY: 100},
				{ModelID: 1954, Code: "MOB_CH_TIGERWOMAN", Region: 24744, RawX: 200, RawY: 200},
			},
		}},
	}
	resourceStore := resources.NewStore(nil)
	resourceStore.SetItemMetadata(metadata)
	handler := New(Dependencies{Agents: newFakeAgentStore(), Registry: agentdomain.NewRegistry(), Resources: resourceStore})
	for _, example := range []struct {
		path    string
		status  int
		count   int
		dataset string
	}{
		{"/api/map/monster-reference/search?server=greatest&q=tiger&min_level=20&max_level=20", 200, 1, mapprofile.GreatestDatasetID},
		{"/api/map/monster-reference/search?server=greatest&q=tiger&min_level=21", 200, 0, mapprofile.GreatestDatasetID},
		{"/api/map/monster-reference/search?server=greatest&min_level=70&max_level=20", 400, 0, ""},
		{"/api/map/monster-reference/search?server=other", 200, 0, "gamedata-other"},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest("GET", example.path, nil))
		if recorder.Code != example.status {
			t.Fatalf("%s status=%d body=%s", example.path, recorder.Code, recorder.Body.String())
		}
		if example.status != 200 {
			continue
		}
		var body struct {
			DatasetID string `json:"dataset_id"`
			Total     int    `json:"total"`
			Status    string `json:"status"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.DatasetID != example.dataset || body.Total != example.count || example.dataset == "gamedata-other" && body.Status != "unavailable" {
			t.Fatalf("%s response=%+v", example.path, body)
		}
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/map/monster-reference/overlay?server=greatest&area=world&floor=world&min_x=168&max_x=168&min_y=96&max_y=96&areas=1&points=1&limit=1", nil))
	if recorder.Code != 200 {
		t.Fatalf("overlay status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var overlay struct {
		Areas      []monsterRefArea  `json:"areas"`
		Points     []monsterRefPoint `json:"points"`
		PointTotal int               `json:"point_total"`
		NextOffset *int              `json:"next_offset"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &overlay); err != nil {
		t.Fatal(err)
	}
	if len(overlay.Areas) != 1 || len(overlay.Points) != 1 || overlay.PointTotal != 2 || overlay.NextOffset == nil || *overlay.NextOffset != 1 {
		t.Fatalf("overlay pagination=%+v", overlay)
	}
}

func TestReferencePointProjectionUsesClientLocalUnitsAndDedicatedCaveFloors(t *testing.T) {
	profile, err := mapprofile.ForServer("greatest", mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	area, floor, pos, ok := projectMonsterRefPoint(profile, resources.MonsterReferencePoint{Region: 24744, RawX: 1842.3, RawY: 600.94})
	if !ok || area != "world" || floor != "world" || pos.TileX != 168 || pos.TileY != 96 || math.Abs(pos.PixelX-245.64) > 0.01 {
		t.Fatalf("outdoor reference misplaced: %s %s %+v %v", area, floor, pos, ok)
	}
	area, floor, pos, ok = projectMonsterRefPoint(profile, resources.MonsterReferencePoint{Region: -32767, RawX: -788.6, RawY: -242.09, RawZ: 0})
	if !ok || area != "donwhang-stone-cave" || floor != "1F" || pos.TileX != 128 || pos.TileY != 127 {
		t.Fatalf("Donwhang reference misplaced: %s %s %+v %v", area, floor, pos, ok)
	}
	area, floor, _, ok = projectMonsterRefPoint(profile, resources.MonsterReferencePoint{Region: 32769, RawX: -788.6, RawY: -242.09, RawZ: 0})
	if !ok || area != "donwhang-stone-cave" || floor != "1F" {
		t.Fatal("unsigned interior alias failed")
	}
	area, floor, _, ok = projectMonsterRefPoint(profile, resources.MonsterReferencePoint{Region: -32761, RawX: -10.05, RawY: -1696.3, RawZ: -77.6})
	if !ok || area != "tomb-of-qin-shi" || floor != "B1" {
		t.Fatal("Tomb reference did not use B1 floor")
	}
	if _, _, _, ok = projectMonsterRefPoint(profile, resources.MonsterReferencePoint{Region: -32752, RawX: 0, RawY: 0, RawZ: 0}); ok {
		t.Fatal("shared Job Temple region was assigned to a guessed floor")
	}
	if _, _, _, ok = projectMonsterRefPoint(profile, resources.MonsterReferencePoint{Region: 24744, RawX: 2500, RawY: 10}); ok {
		t.Fatal("out-of-range outdoor offset was projected")
	}
}

func TestGuideGroupsMapOnlyToKnownFloors(t *testing.T) {
	for _, example := range []struct{ group, area, floor string }{
		{"field", "world", "world"}, {"dunhuang4", "donwhang-stone-cave", "4F"}, {"jinsi6", "tomb-of-qin-shi", "B6"}, {"temple", "job-temple", "1F"},
	} {
		a, f, ok := monsterRefAreaFloor(example.group)
		if !ok || a != example.area || f != example.floor {
			t.Fatalf("%s => %s/%s", example.group, a, f)
		}
	}
	if _, _, ok := monsterRefAreaFloor("unknown"); ok {
		t.Fatal("unknown guide group mapped")
	}
}

func TestPackagedMonsterReferenceCoverageStaysOnItsMapProfile(t *testing.T) {
	metadata, err := resources.LoadItemMetadata("../../game-data")
	if err != nil {
		t.Fatal(err)
	}
	catalog := metadata.MonsterReferenceForDataset(mapprofile.GreatestDatasetID)
	if catalog == nil {
		t.Fatal("active dataset has no monster reference catalog")
	}
	profile, err := mapprofile.ForServer("greatest", mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	projected, excluded := 0, 0
	byArea := map[string]int{}
	excludedRegions := map[int]int{}
	for _, point := range catalog.Points {
		area, _, _, ok := projectMonsterRefPoint(profile, point)
		if !ok {
			excluded++
			excludedRegions[point.Region]++
			continue
		}
		projected++
		byArea[area]++
	}
	if projected == 0 || byArea["world"] == 0 || byArea["donwhang-stone-cave"] == 0 || excluded == 0 {
		t.Fatalf("incomplete packaged projection: projected=%d excluded=%d byArea=%v", projected, excluded, byArea)
	}
	t.Logf("catalog points=%d projected=%d excluded=%d byArea=%v excludedRegions=%v", len(catalog.Points), projected, excluded, byArea, excludedRegions)
}

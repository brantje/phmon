package mapprofile

import "testing"

func TestRequiredMapFamiliesHaveIndependentFloorAndTransformStatus(t *testing.T) {
	profile, err := ForServer("greatest", GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.CoordinateTransform != "outdoor-region-grid" || profile.CommandZEvidence != "unverified" || profile.RegionMappingsStatus != "outdoor-region-grid" {
		t.Fatalf("map evidence was overstated: %+v", profile)
	}
	if len(profile.QuickDestinations) != 0 {
		t.Fatalf("unvalidated map entries were advertised: %+v", profile)
	}
	if len(profile.RegionMappings) != 4 || len(profile.CoordinateTransforms) != 25 {
		t.Fatalf("expected four observed outdoor examples: %+v", profile.RegionMappings)
	}
	for _, mapping := range profile.RegionMappings {
		if mapping.Status != "validated" || mapping.AreaID != "world" || mapping.FloorID != "world" {
			t.Fatalf("unexpected region mapping: %+v", mapping)
		}
	}
	if profile.RegionMappings[2].Region != 25735 || profile.RegionMappings[2].TileX != 135 || profile.RegionMappings[2].TileY != 100 {
		t.Fatalf("live Greatest region has wrong root tile: %+v", profile.RegionMappings[2])
	}
	for index, transform := range profile.CoordinateTransforms[:4] {
		mapping := profile.RegionMappings[index]
		if transform.Region != mapping.Region || transform.WorldOriginX != float64(mapping.TileX-135)*192 || transform.WorldOriginY != float64(mapping.TileY-92)*192 || transform.UnitsPerTileX != 192 || transform.UnitsPerTileY != 192 || transform.CommandZ != nil {
			t.Fatalf("outdoor transform or command Z was misrepresented: %+v", transform)
		}
	}
	if len(profile.ViewPresets) != 18 || profile.ViewPresets[0].Status != "raster-reference-only" {
		t.Fatalf("expected world and 17 cave floor view presets: %+v", profile.ViewPresets)
	}
	want := map[string]int{"tomb-of-qin-shi": 6, "donwhang-stone-cave": 4, "job-temple": 7}
	tileCount := 0
	assetFormats := map[string]bool{}
	for _, area := range profile.Areas {
		if count, ok := want[area.ID]; ok {
			if len(area.Floors) != count {
				t.Errorf("%s floors=%d want %d", area.ID, len(area.Floors), count)
			}
			for _, floor := range area.Floors {
				if floor.ImageStatus != "available" || floor.TransformStatus != "reference-observed" || floor.Tiles == nil || floor.Tiles.TileCount == 0 {
					t.Errorf("cave floor mapping unavailable: %+v", floor)
				}
				if floor.Tiles != nil {
					tileCount += floor.Tiles.TileCount
					assetFormats[floor.Tiles.TileURLFormat] = true
				}
			}
			delete(want, area.ID)
		}
	}
	if len(want) > 0 {
		t.Fatalf("missing cave families: %v", want)
	}
	if tileCount != 1891 {
		t.Fatalf("expected 1891 cave tiles across 17 floors, got %d", tileCount)
	}
	if len(assetFormats) != 17 {
		t.Fatalf("expected one local tile format for each of 17 floors, got %d", len(assetFormats))
	}
}

func TestCaveFloorClassification(t *testing.T) {
	profile, _ := ForServer("greatest", GreatestDatasetID)
	for _, test := range []struct {
		region      int
		z           float64
		area, floor string
	}{
		{-32761, 0, "tomb-of-qin-shi", "B1"}, {-32766, 0, "tomb-of-qin-shi", "B6"},
		{-32767, -50, "donwhang-stone-cave", "1F"}, {32767, 70, "donwhang-stone-cave", "1F"},
		{-32767, 71, "donwhang-stone-cave", "2F"}, {-32767, 211, "donwhang-stone-cave", "3F"},
		{-32767, 490, "donwhang-stone-cave", "4F"}, {-32752, 100, "job-temple", "1F"},
	} {
		area, floor, ok := ClassifyCave(profile, &test.region, &test.z)
		if !ok || area != test.area || floor != test.floor {
			t.Errorf("region %d z %v => %s/%s", test.region, test.z, area, floor)
		}
	}
	region, z := -32767, 70.5
	if _, _, ok := ClassifyCave(profile, &region, &z); ok {
		t.Fatal("Donwhang Z gap must remain unclassified")
	}
	if _, _, ok := ClassifyCave(profile, &region, nil); ok {
		t.Fatal("Donwhang without Z is ambiguous")
	}
	job := profile.Areas[3]
	for _, floor := range job.Floors[1:] {
		if floor.AutoDetect {
			t.Errorf("Job Temple %s must require manual floor selection", floor.ID)
		}
	}
}

func TestUnknownDatasetDoesNotAdvertiseLocalTiles(t *testing.T) {
	profile, err := ForServer("other", "gamedata-other")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ProfileStatus != "unsupported" || profile.TileCatalog.Status != "unavailable-for-dataset" || profile.TileCatalog.TileCount != 0 || len(profile.ViewPresets) != 0 || len(profile.RegionMappings) != 0 {
		t.Fatalf("unsupported profile advertises map tiles: %+v", profile)
	}
}

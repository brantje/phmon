package mapprofile

import "testing"

func TestRequiredMapFamiliesHaveIndependentFloorAndTransformStatus(t *testing.T) {
	profile, err := ForServer("greatest", GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.CoordinateTransform != "partial-validated-outdoor" || profile.CommandZEvidence != "unverified" || profile.RegionMappingsStatus != "partial-validated" {
		t.Fatalf("map evidence was overstated: %+v", profile)
	}
	if len(profile.QuickDestinations) != 0 {
		t.Fatalf("unvalidated map entries were advertised: %+v", profile)
	}
	if len(profile.RegionMappings) != 4 || len(profile.CoordinateTransforms) != 4 {
		t.Fatalf("expected only the four documented outdoor joins: %+v", profile.RegionMappings)
	}
	for _, mapping := range profile.RegionMappings {
		if mapping.Status != "validated" || mapping.AreaID != "world" || mapping.FloorID != "world" {
			t.Fatalf("unexpected region mapping: %+v", mapping)
		}
	}
	if profile.RegionMappings[2].Region != 25735 || profile.RegionMappings[2].TileX != 135 || profile.RegionMappings[2].TileY != 100 {
		t.Fatalf("live Greatest region has wrong root tile: %+v", profile.RegionMappings[2])
	}
	for index, transform := range profile.CoordinateTransforms {
		mapping := profile.RegionMappings[index]
		if transform.Region != mapping.Region || transform.WorldOriginX != float64(mapping.TileX-135)*192 || transform.WorldOriginY != float64(mapping.TileY-92)*192 || transform.UnitsPerTileX != 192 || transform.UnitsPerTileY != 192 || transform.CommandZ != nil {
			t.Fatalf("outdoor transform or command Z was misrepresented: %+v", transform)
		}
	}
	if len(profile.ViewPresets) != 1 || profile.ViewPresets[0].Status != "raster-reference-only" {
		t.Fatalf("expected only the labelled raster reference view preset: %+v", profile.ViewPresets)
	}
	want := map[string]int{"tomb-of-qin-shi": 6, "donwhang-stone-cave": 4, "job-temple": 7}
	for _, area := range profile.Areas {
		if count, ok := want[area.ID]; ok {
			if len(area.Floors) != count {
				t.Errorf("%s floors=%d want %d", area.ID, len(area.Floors), count)
			}
			for _, floor := range area.Floors {
				if floor.ImageStatus != "missing" || floor.TransformStatus != "unvalidated" {
					t.Errorf("cave floor unexpectedly enabled: %+v", floor)
				}
			}
			delete(want, area.ID)
		}
	}
	if len(want) > 0 {
		t.Fatalf("missing cave families: %v", want)
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

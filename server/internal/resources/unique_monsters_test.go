package resources

import "testing"

func TestUniqueCatalogResolvesTigerGirlPortrait(t *testing.T) {
	metadata, err := LoadItemMetadata("../../game-data")
	if err != nil {
		t.Fatal(err)
	}
	info := metadata.UniqueInfo("Greatest", []byte(`{"model":1954,"notice":"spawn"}`))
	if info == nil || info.Name != "Tiger Girl" || info.ImageURL != "/game-assets/monsters/tigerwoman.png" || info.Level == nil || *info.Level != 20 {
		t.Fatalf("model lookup = %#v", info)
	}
	named := metadata.UniqueInfo("greatest", []byte(`{"value":"Tiger Girl"}`))
	if named == nil || named.ImageURL != "/game-assets/monsters/tigerwoman.png" || named.Level == nil || *named.Level != 20 {
		t.Fatalf("name lookup = %#v", named)
	}
	if metadata.UniqueInfo("missing", []byte(`{"model":1954,"notice":"spawn"}`)) != nil {
		t.Fatal("unmapped server used the Greatest unique catalog")
	}
}

package resources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReverseReturnNamesStayInSelectedProfile(t *testing.T) {
	m, err := LoadItemMetadata("../../game-data")
	if err != nil {
		t.Fatal(err)
	}
	names := m.ReverseReturnLocations(" Greatest ")
	if len(names) != 36 {
		t.Fatalf("names=%v", names)
	}
	found := false
	for _, name := range names {
		if name == "Jangan" {
			found = true
		}
	}
	if !found || len(m.ReverseReturnLocations("Zerkroad")) != 0 {
		t.Fatal("cross-profile or missing names")
	}
	names[0] = "modified"
	if m.ReverseReturnLocations("Greatest")[0] == "modified" {
		t.Fatal("caller changed shared catalog")
	}
}

func TestInvalidNamedLocationCatalogRejected(t *testing.T) {
	for _, raw := range []string{`{"gamedata-a":[""]}`, `{"gamedata-a":[" Hotan"]}`, `{"gamedata-a":["Hotan","hotan"]}`, `{"gamedata-a":["bad\n"]}`} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "servers.json"), []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "reverse-return-locations.json"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadItemMetadata(dir); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

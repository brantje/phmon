package resources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMonsterLevelsStayWithinDatasetAndRequireExactIdentity(t *testing.T) {
	directory := t.TempDir()
	first := `{"catalogVersion":"1.2.3","datasetId":"gamedata-first","family":"monsterReference","status":"partial","records":[{"model_id":1954,"code":"MOB_CH_TIGERWOMAN","name":"Tiger Girl","level":20,"enabled":true},{"model_id":3796,"code":"MOB_TK_EDIMMU_CLON","name":"Shakram","level":61,"enabled":true},{"model_id":3798,"code":"MOB_TK_EDIMMU","name":"Edimmu","level":63,"enabled":true}],"areas":[],"points":[],"coverage":{}}`
	second := `{"catalogVersion":"1.2.3","datasetId":"gamedata-second","family":"monsterReference","status":"partial","records":[{"model_id":9000,"code":"MOB_CH_TIGERWOMAN","name":"Tiger Girl","level":20,"enabled":true}],"areas":[],"points":[],"coverage":{}}`
	for dataset, payload := range map[string]string{"gamedata-first": first, "gamedata-second": second} {
		if err := os.WriteFile(filepath.Join(directory, dataset+"-monster-reference.json"), []byte(payload), 0600); err != nil {
			t.Fatal(err)
		}
	}
	firstCatalog, err := loadMonsterReference(filepath.Join(directory, "gamedata-first-monster-reference.json"), "gamedata-first")
	if err != nil {
		t.Fatal(err)
	}
	secondCatalog, err := loadMonsterReference(filepath.Join(directory, "gamedata-second-monster-reference.json"), "gamedata-second")
	if err != nil {
		t.Fatal(err)
	}
	metadata := &ItemMetadata{MonsterReferences: map[string]*MonsterReference{"gamedata-first": firstCatalog, "gamedata-second": secondCatalog}}
	for _, example := range []struct {
		model int64
		code  string
		want  int
	}{
		{1954, "MOB_CH_TIGERWOMAN", 20}, {3796, "MOB_TK_EDIMMU_CLON", 61}, {3798, "MOB_TK_EDIMMU", 63},
	} {
		got, ok := metadata.MonsterLevel("gamedata-first", &example.model, example.code)
		if !ok || got != example.want {
			t.Fatalf("%s level got %d, %v", example.code, got, ok)
		}
	}
	model := int64(1954)
	if _, ok := metadata.MonsterLevel("gamedata-second", &model, "MOB_CH_TIGERWOMAN"); ok {
		t.Fatal("cross-dataset model matched")
	}
	if _, ok := metadata.MonsterLevel("gamedata-first", &model, "MOB_TK_EDIMMU"); ok {
		t.Fatal("mismatched code matched")
	}
	if got, ok := metadata.MonsterLevel("gamedata-second", nil, "MOB_CH_TIGERWOMAN"); !ok || got != 20 {
		t.Fatal("unique code-only lookup failed")
	}
}

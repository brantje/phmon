package resources

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestPetReferenceAddsCatalogMaximaWithoutOverridingRuntimeValues(t *testing.T) {
	reference := &PetReference{
		pets: map[int64]petReferenceRecord{6142: {Level: 37, MaxHP: 6022}},
		exp:  map[int64]int64{37: 3239210},
	}
	num := func(v int64) json.Number { return json.Number(strconv.FormatInt(v, 10)) }
	pet := map[string]any{"model": num(6142), "hp": num(6018), "level": num(37)}
	reference.enrichPet(pet)
	if pet["hp_max"] != int64(6022) || pet["max_exp"] != int64(3239210) {
		t.Fatalf("catalog maxima missing: %#v", pet)
	}
	reported := map[string]any{"model": num(6142), "hp": num(10), "hp_max": num(99), "level": num(37), "max_exp": num(5)}
	reference.enrichPet(reported)
	if reported["hp_max"] != num(99) || reported["max_exp"] != num(5) {
		t.Fatalf("runtime values overridden: %#v", reported)
	}
	mismatch := map[string]any{"model": num(6142), "hp": num(7000)}
	reference.enrichPet(mismatch)
	if _, ok := mismatch["hp_max"]; ok {
		t.Fatalf("hp above catalog max must not be paired: %#v", mismatch)
	}
	pick := map[string]any{"model": num(6142), "hp": num(5), "level": num(0)}
	reference.enrichPet(pick)
	if _, ok := pick["max_exp"]; ok {
		t.Fatalf("level 0 must not get an EXP maximum: %#v", pick)
	}
}

func TestPackagedPetReferenceLoads(t *testing.T) {
	reference, err := loadPetReference("../../game-data/pet-reference.json")
	if err != nil || reference == nil {
		t.Fatalf("load: %v", err)
	}
	if reference.pets[6142].MaxHP != 6022 || reference.exp[37] != 3239210 {
		t.Fatalf("unexpected values: %#v %d", reference.pets[6142], reference.exp[37])
	}
}

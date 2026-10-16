package resources

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// PetReference holds exported static pet facts for one dataset: each pet
// model's level and maximum HP, and the EXP needed per level. phBot's
// get_pets() reports neither maximum, so these are catalog values, not
// observations.
type PetReference struct {
	DatasetID string
	pets      map[int64]petReferenceRecord
	exp       map[int64]int64
}

type petReferenceRecord struct {
	Level int64 `json:"level"`
	MaxHP int64 `json:"max_hp"`
}

type petReferenceFile struct {
	DatasetID string                        `json:"dataset_id"`
	Pets      map[string]petReferenceRecord `json:"pets"`
	EXP       map[string]int64              `json:"exp_to_next_level"`
}

func loadPetReference(path string) (*PetReference, error) {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if stat.Size() > 2<<20 {
		return nil, fmt.Errorf("pet reference exceeds size limit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file petReferenceFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if !datasetName.MatchString(file.DatasetID) || len(file.Pets) == 0 || len(file.Pets) > 20000 || len(file.EXP) > 1000 {
		return nil, fmt.Errorf("invalid pet reference identity or size")
	}
	reference := &PetReference{
		DatasetID: file.DatasetID,
		pets:      make(map[int64]petReferenceRecord, len(file.Pets)),
		exp:       make(map[int64]int64, len(file.EXP)),
	}
	for key, record := range file.Pets {
		model, err := strconv.ParseInt(key, 10, 64)
		if err != nil || model < 1 || model > 4294967295 || record.MaxHP < 1 || record.MaxHP > 4294967295 {
			return nil, fmt.Errorf("invalid pet reference record")
		}
		reference.pets[model] = record
	}
	for key, needed := range file.EXP {
		level, err := strconv.ParseInt(key, 10, 64)
		if err != nil || level < 1 || needed < 1 {
			return nil, fmt.Errorf("invalid pet reference EXP level")
		}
		reference.exp[level] = needed
	}
	return reference, nil
}

// enrichPet adds catalog maxima without overriding anything the runtime
// reported. Max HP is skipped when the observed HP already exceeds it, which
// would mean the model belongs to another dataset or a different pet variant.
func (r *PetReference) enrichPet(pet map[string]any) {
	if r == nil {
		return
	}
	hp, hasHP := jsonInt(pet["hp"])
	if model, ok := jsonInt(pet["model"]); ok {
		if record, found := r.pets[model]; found {
			if _, present := pet["hp_max"]; !present && (!hasHP || hp <= record.MaxHP) {
				pet["hp_max"] = record.MaxHP
				pet["hp_max_source"] = "catalog"
			}
		}
	}
	if level, ok := jsonInt(pet["level"]); ok && level > 0 {
		if needed, found := r.exp[level]; found {
			if _, present := pet["max_exp"]; !present {
				pet["max_exp"] = needed
				pet["max_exp_source"] = "catalog"
			}
		}
	}
}

func jsonInt(value any) (int64, bool) {
	switch number := value.(type) {
	case json.Number:
		parsed, err := number.Int64()
		return parsed, err == nil
	case float64:
		return int64(number), number == float64(int64(number))
	}
	return 0, false
}

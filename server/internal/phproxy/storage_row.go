package phproxy

import (
	"encoding/json"
	"time"
)

const enrichmentSelectColumns = `model_ref_id, model_ref_observed_at, gear, gear_observed_at, field_evidence,
	extended, proxy_level, proxy_job, proxy_job_level, proxy_guild, proxy_race`

type rowScanner interface {
	Scan(dest ...any) error
}

func loadStoredFieldsFromRow(row rowScanner) (*storedFields, error) {
	var modelID *int
	var modelAt, gearAt *time.Time
	var gearJSON, evidenceJSON, extendedJSON []byte
	var proxyLevel, proxyJobLevel *int
	var proxyJob, proxyGuild, proxyRace *string
	if err := row.Scan(
		&modelID, &modelAt, &gearJSON, &gearAt, &evidenceJSON,
		&extendedJSON, &proxyLevel, &proxyJob, &proxyJobLevel, &proxyGuild, &proxyRace,
	); err != nil {
		return nil, err
	}
	st := &storedFields{
		ModelRefID:         modelID,
		ModelRefObservedAt: modelAt,
		GearObservedAt:     gearAt,
		FieldEvidence:      map[string]FieldEvidence{},
		ProxyLevel:         proxyLevel,
		ProxyJob:           proxyJob,
		ProxyJobLevel:      proxyJobLevel,
		ProxyGuild:         proxyGuild,
		ProxyRace:          proxyRace,
	}
	if len(gearJSON) > 0 {
		if err := json.Unmarshal(gearJSON, &st.Gear); err != nil {
			return nil, err
		}
	}
	if len(evidenceJSON) > 0 {
		if err := json.Unmarshal(evidenceJSON, &st.FieldEvidence); err != nil {
			return nil, err
		}
	}
	if len(extendedJSON) > 0 {
		if err := json.Unmarshal(extendedJSON, &st.Extended); err != nil {
			return nil, err
		}
	}
	return st, nil
}

func marshalStoredRow(st *storedFields) (gearJSON, evJSON, extJSON []byte, err error) {
	gearJSON, err = gearToJSON(st.Gear)
	if err != nil {
		return nil, nil, nil, err
	}
	evJSON, err = evidenceToJSON(st.FieldEvidence)
	if err != nil {
		return nil, nil, nil, err
	}
	extJSON, err = json.Marshal(st.Extended)
	if err != nil {
		return nil, nil, nil, err
	}
	return gearJSON, evJSON, extJSON, nil
}

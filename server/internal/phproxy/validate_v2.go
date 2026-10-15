package phproxy

import (
	"encoding/json"
	"math"
	"strings"

	agentdomain "phmon/server/internal/agents"
)

type wirePlayerV2 struct {
	Name          string                     `json:"name"`
	IdentityKind  string                     `json:"identity_kind"`
	ModelRefID    *int                       `json:"model_ref_id"`
	Gear          *[]GearEntry               `json:"gear"`
	AvatarGear    *[]GearEntry               `json:"avatar_gear"`
	Level         *int                       `json:"level"`
	Job           *int                       `json:"job"`
	JobLevel      *int                       `json:"job_level"`
	Race          *string                    `json:"race"`
	Guild         *string                    `json:"guild"`
	Position      *WirePosition              `json:"position"`
	JobItemIDs    *[]int                     `json:"job_item_ids"`
	IdentityLinks *[]IdentityLink            `json:"identity_links"`
	FieldEvidence map[string]json.RawMessage `json:"field_evidence"`
}

type wireRequestV2 struct {
	SchemaVersion  int             `json:"schema_version"`
	RequestID      string          `json:"request_id"`
	AgentID        string          `json:"agent_id"`
	Server         string          `json:"server"`
	Players        []wirePlayerV2  `json:"players"`
	ServerMetadata *ServerMetadata `json:"server_metadata"`
	Coverage       *Coverage       `json:"coverage"`
}

func isSpawnOpcode(op int) bool {
	return op == OpcodeSpawnEquipment || op == OpcodeSpawnEquipment2
}

func isMetadataOpcode(op int) bool {
	return op == OpcodeMetadataB007 || op == OpcodeMetadata3065 || op == OpcodeMetadata3864
}

func validateEvidenceV1(ev FieldEvidence) error {
	if ev.Profile != ProfileName {
		return ErrInvalid
	}
	if ev.Reason != EvidenceReasonVerified {
		return ErrInvalid
	}
	if !isSpawnOpcode(ev.SourceOpcode) {
		return ErrInvalid
	}
	if ev.ObservedAt.IsZero() {
		return ErrInvalid
	}
	if ev.SourceIdentityKind != "" || ev.SourceName != "" || ev.Association != nil {
		return ErrInvalid
	}
	return nil
}

func validateEvidenceV2(field string, ev FieldEvidence, playerName string) error {
	if ev.Profile != ProfileName || ev.ObservedAt.IsZero() {
		return ErrInvalid
	}
	directOnly := field == "name" || field == "level" || field == "identity_links"
	if ev.SourceIdentityKind == IdentityKindCharacterName {
		if !validPlayerNameV2(ev.SourceName) || !namesEqual(ev.SourceName, playerName) || ev.Association != nil {
			return ErrInvalid
		}
	} else if ev.SourceIdentityKind == IdentityKindJobAlias && !directOnly {
		a := ev.Association
		if a == nil || !validPlayerNameV2(ev.SourceName) || !validPlayerNameV2(a.Alias) ||
			!namesEqual(a.Alias, ev.SourceName) || a.Status != IdentityLinkStatusInferred ||
			a.Reason != IdentityLinkReasonJobTransition || a.ObservedAt.IsZero() {
			return ErrInvalid
		}
	} else {
		return ErrInvalid
	}
	switch field {
	case "name":
		if ev.Reason != EvidenceReasonVerified || (!isSpawnOpcode(ev.SourceOpcode) && !isMetadataOpcode(ev.SourceOpcode)) {
			return ErrInvalid
		}
	case "model_ref_id":
		if (ev.Reason != EvidenceReasonVerified && ev.Reason != EvidenceReasonUnverifiedReference) || (!isSpawnOpcode(ev.SourceOpcode) && !isMetadataOpcode(ev.SourceOpcode)) {
			return ErrInvalid
		}
	case "gear":
		if ev.Reason != EvidenceReasonVerified || !isSpawnOpcode(ev.SourceOpcode) {
			return ErrInvalid
		}
	case "level":
		if (ev.Reason != EvidenceReasonVerified && ev.Reason != EvidenceReasonUnverifiedReference) || !isMetadataOpcode(ev.SourceOpcode) {
			return ErrInvalid
		}
	case "avatar_gear", "job", "job_level", "race", "guild", "position", "job_item_ids":
		if (ev.Reason != EvidenceReasonVerified && ev.Reason != EvidenceReasonUnverifiedReference) || !isSpawnOpcode(ev.SourceOpcode) {
			return ErrInvalid
		}
	case "identity_links":
		if ev.Reason != EvidenceReasonInferredJobTransition || !isSpawnOpcode(ev.SourceOpcode) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func namesEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func parseFieldEvidenceV2(raw map[string]json.RawMessage, playerName string) (map[string]FieldEvidence, error) {
	if raw == nil {
		return nil, ErrInvalid
	}
	allowed := map[string]bool{
		"name": true, "model_ref_id": true, "gear": true, "avatar_gear": true, "level": true,
		"job": true, "job_level": true, "race": true, "guild": true, "position": true,
		"job_item_ids": true, "identity_links": true,
	}
	out := make(map[string]FieldEvidence, len(raw))
	for key, value := range raw {
		if !allowed[key] {
			return nil, validationError("unsupported_field", "field_evidence", -1)
		}
		var ev FieldEvidence
		decoder := json.NewDecoder(strings.NewReader(string(value)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&ev); err != nil {
			return nil, validationError("invalid_field_evidence", key, -1)
		}
		if err := validateEvidenceV2(key, ev, playerName); err != nil {
			return nil, validationError("invalid_field_evidence", key, -1)
		}
		out[key] = ev
	}
	return out, nil
}

func validLevel(v int) bool {
	return v >= 1 && v <= 255
}

func validJobCode(v int) bool {
	return v >= 0 && v <= 255
}

func validGuild(guild string) bool {
	return guild == strings.TrimSpace(guild) && len(guild) <= MaxGuildBytes
}

func validRace(race string) bool {
	return race != "" && race == strings.TrimSpace(race) && len(race) <= MaxRaceBytes
}

func validPosition(pos *WirePosition) bool {
	if pos == nil {
		return false
	}
	if !validFiniteFloat32(pos.X) || !validFiniteFloat32(pos.Y) || !validFiniteFloat32(pos.Z) {
		return false
	}
	return true
}

func validFiniteFloat32(v float32) bool {
	if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return false
	}
	return true
}

func validateGearEntries(entries []GearEntry) error {
	if len(entries) > MaxGearPerPlayer {
		return ErrInvalid
	}
	for _, g := range entries {
		if !validRefIDV2(g.RefItemID) || !validOptOrSlot(g.OptLevel) || !validOptOrSlot(g.Slot) {
			return ErrInvalid
		}
	}
	return nil
}

func validateJobItemIDs(ids []int) error {
	if len(ids) > MaxGearPerPlayer {
		return ErrInvalid
	}
	for _, id := range ids {
		if !validRefIDV2(id) {
			return ErrInvalid
		}
	}
	return nil
}

func validateIdentityLinks(links []IdentityLink) error {
	if len(links) > MaxIdentityLinksPerPlayer {
		return ErrInvalid
	}
	seen := make(map[string]struct{}, len(links))
	for _, link := range links {
		alias := strings.TrimSpace(link.Alias)
		if alias == "" || len(alias) > MaxPlayerNameBytesV2 || alias != link.Alias {
			return ErrInvalid
		}
		if _, dup := seen[strings.ToLower(alias)]; dup {
			return ErrInvalid
		}
		seen[strings.ToLower(alias)] = struct{}{}
		if link.Status != IdentityLinkStatusInferred || link.Reason != IdentityLinkReasonJobTransition {
			return ErrInvalid
		}
		if link.Profile != ProfileName || !isSpawnOpcode(link.SourceOpcode) {
			return ErrInvalid
		}
		if link.Matches < 1 || link.FirstMatched.IsZero() || link.LastMatched.IsZero() {
			return ErrInvalid
		}
		if link.FirstMatched.After(link.LastMatched) {
			return ErrInvalid
		}
	}
	return nil
}

func validateSubmitV2(body []byte, authenticatedAgentID string) (validatedRequest, error) {
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	var wire wireRequestV2
	if err := dec.Decode(&wire); err != nil {
		return validatedRequest{}, ErrInvalid
	}
	if dec.More() {
		return validatedRequest{}, ErrInvalid
	}
	if wire.SchemaVersion != SchemaVersionV2 {
		return validatedRequest{}, ErrInvalid
	}
	if !lowercaseUUID.MatchString(wire.RequestID) || !agentdomain.ValidAgentID(wire.RequestID) {
		return validatedRequest{}, ErrInvalid
	}
	if !lowercaseUUID.MatchString(wire.AgentID) {
		return validatedRequest{}, ErrInvalid
	}
	if wire.AgentID != authenticatedAgentID {
		return validatedRequest{}, AgentIDMismatch{}
	}
	if !validServerName(wire.Server) {
		return validatedRequest{}, ErrInvalid
	}
	if len(wire.Players) > MaxPlayersPerBatch {
		return validatedRequest{}, ErrInvalid
	}
	seen := make(map[string]struct{}, len(wire.Players))
	players := make([]validatedPlayer, 0, len(wire.Players))
	for index, p := range wire.Players {
		if p.IdentityKind != IdentityKindCharacterName {
			return validatedRequest{}, ErrInvalid
		}
		if !validPlayerNameV2(p.Name) {
			return validatedRequest{}, ErrInvalid
		}
		sk := serverKey(wire.Server)
		nk := nameKey(p.Name)
		key := sk + "\x00" + nk
		if _, dup := seen[key]; dup {
			return validatedRequest{}, ErrInvalid
		}
		seen[key] = struct{}{}
		evidence, err := parseFieldEvidenceV2(p.FieldEvidence, p.Name)
		if err != nil {
			return validatedRequest{}, withPlayerIndex(err, index)
		}
		if _, ok := evidence["name"]; !ok {
			return validatedRequest{}, ErrInvalid
		}
		vp := validatedPlayer{
			Name:      strings.TrimSpace(p.Name),
			ServerKey: sk,
			NameKey:   nk,
			Evidence:  evidence,
		}
		if p.ModelRefID != nil {
			if !validRefIDV2(*p.ModelRefID) {
				return validatedRequest{}, validationError("invalid_field", "model_ref_id", index)
			}
			if _, ok := evidence["model_ref_id"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "model_ref_id", index)
			}
			vp.ModelRefID = p.ModelRefID
			vp.HasModel = true
		}
		if p.Gear != nil {
			if _, ok := evidence["gear"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "gear", index)
			}
			if err := validateGearEntries(*p.Gear); err != nil {
				return validatedRequest{}, validationError("invalid_field", "gear", index)
			}
			vp.Gear = *p.Gear
			vp.HasGear = true
			vp.GearEmpty = len(*p.Gear) == 0
		}
		if p.AvatarGear != nil {
			if _, ok := evidence["avatar_gear"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "avatar_gear", index)
			}
			if err := validateGearEntries(*p.AvatarGear); err != nil {
				return validatedRequest{}, validationError("invalid_field", "avatar_gear", index)
			}
			vp.AvatarGear = *p.AvatarGear
			vp.HasAvatarGear = true
			vp.AvatarGearEmpty = len(*p.AvatarGear) == 0
		}
		if p.Level != nil {
			if !validLevel(*p.Level) {
				return validatedRequest{}, validationError("invalid_field", "level", index)
			}
			if _, ok := evidence["level"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "level", index)
			}
			vp.Level = p.Level
			vp.HasLevel = true
		}
		if p.Job != nil {
			if !validJobCode(*p.Job) {
				return validatedRequest{}, validationError("invalid_field", "job", index)
			}
			if _, ok := evidence["job"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "job", index)
			}
			vp.Job = p.Job
			vp.HasJob = true
		}
		if p.JobLevel != nil {
			if !validJobCode(*p.JobLevel) {
				return validatedRequest{}, validationError("invalid_field", "job_level", index)
			}
			if _, ok := evidence["job_level"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "job_level", index)
			}
			vp.JobLevel = p.JobLevel
			vp.HasJobLevel = true
		}
		if p.Race != nil {
			if !validRace(*p.Race) {
				return validatedRequest{}, validationError("invalid_field", "race", index)
			}
			if _, ok := evidence["race"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "race", index)
			}
			race := strings.TrimSpace(*p.Race)
			vp.Race = &race
			vp.HasRace = true
		}
		if p.Guild != nil {
			if !validGuild(*p.Guild) {
				return validatedRequest{}, validationError("invalid_field", "guild", index)
			}
			if _, ok := evidence["guild"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "guild", index)
			}
			guild := *p.Guild
			vp.Guild = &guild
			vp.HasGuild = true
		}
		if p.Position != nil {
			if !validPosition(p.Position) {
				return validatedRequest{}, validationError("invalid_field", "position", index)
			}
			if _, ok := evidence["position"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "position", index)
			}
			vp.Position = p.Position
			vp.HasPosition = true
		}
		if p.JobItemIDs != nil {
			if _, ok := evidence["job_item_ids"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "job_item_ids", index)
			}
			if err := validateJobItemIDs(*p.JobItemIDs); err != nil {
				return validatedRequest{}, validationError("invalid_field", "job_item_ids", index)
			}
			vp.JobItemIDs = *p.JobItemIDs
			vp.HasJobItemIDs = true
			vp.JobItemIDsEmpty = len(*p.JobItemIDs) == 0
		}
		if p.IdentityLinks != nil {
			if _, ok := evidence["identity_links"]; !ok {
				return validatedRequest{}, validationError("invalid_field", "identity_links", index)
			}
			if err := validateIdentityLinks(*p.IdentityLinks); err != nil {
				return validatedRequest{}, validationError("invalid_field", "identity_links", index)
			}
			vp.IdentityLinks = *p.IdentityLinks
			vp.HasIdentityLinks = true
			vp.IdentityLinksEmpty = len(*p.IdentityLinks) == 0
		}
		for field := range evidence {
			present := field == "name"
			switch field {
			case "model_ref_id":
				present = p.ModelRefID != nil
			case "gear":
				present = p.Gear != nil
			case "avatar_gear":
				present = p.AvatarGear != nil
			case "level":
				present = p.Level != nil
			case "job":
				present = p.Job != nil
			case "job_level":
				present = p.JobLevel != nil
			case "race":
				present = p.Race != nil
			case "guild":
				present = p.Guild != nil
			case "position":
				present = p.Position != nil
			case "job_item_ids":
				present = p.JobItemIDs != nil
			case "identity_links":
				present = p.IdentityLinks != nil
			}
			if !present {
				return validatedRequest{}, validationError("evidence_without_value", field, index)
			}
		}
		players = append(players, vp)
	}
	if wire.ServerMetadata != nil {
		if strings.TrimSpace(wire.ServerMetadata.GatewayName) == "" ||
			len(wire.ServerMetadata.GatewayName) > 128 ||
			!validGatewayCount(wire.ServerMetadata.Online) ||
			!validGatewayCount(wire.ServerMetadata.Max) ||
			wire.ServerMetadata.ObservedAt.IsZero() {
			return validatedRequest{}, ErrInvalid
		}
		if wire.ServerMetadata.Online != nil && wire.ServerMetadata.Max != nil &&
			*wire.ServerMetadata.Online > *wire.ServerMetadata.Max {
			return validatedRequest{}, ErrInvalid
		}
	}
	if wire.Coverage != nil {
		if wire.Coverage.Profile != ProfileName ||
			wire.Coverage.DroppedObservations < 0 || wire.Coverage.DroppedObservations > 1_000_000_000 {
			return validatedRequest{}, ErrInvalid
		}
	}
	canonical, err := json.Marshal(wire)
	if err != nil {
		return validatedRequest{}, ErrInvalid
	}
	return validatedRequest{
		SchemaVersion:  SchemaVersionV2,
		RequestID:      wire.RequestID,
		AgentID:        wire.AgentID,
		Server:         strings.TrimSpace(wire.Server),
		ServerKey:      serverKey(wire.Server),
		Players:        players,
		ServerMetadata: wire.ServerMetadata,
		Coverage:       wire.Coverage,
		CanonicalJSON:  canonical,
	}, nil
}

func validRefIDV2(id int) bool { return id >= 1 && uint64(id) <= 4294967295 }

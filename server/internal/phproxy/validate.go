package phproxy

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	agentdomain "phmon/server/internal/agents"
)

var lowercaseUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type validatedPlayer struct {
	Name               string
	ServerKey          string
	NameKey            string
	ModelRefID         *int
	Gear               []GearEntry
	AvatarGear         []GearEntry
	Level              *int
	Job                *int
	JobLevel           *int
	Race               *string
	Guild              *string
	Position           *WirePosition
	JobItemIDs         []int
	IdentityLinks      []IdentityLink
	Evidence           map[string]FieldEvidence
	HasModel           bool
	HasGear            bool
	GearEmpty          bool
	HasAvatarGear      bool
	AvatarGearEmpty    bool
	HasLevel           bool
	HasJob             bool
	HasJobLevel        bool
	HasRace            bool
	HasGuild           bool
	HasPosition        bool
	HasJobItemIDs      bool
	JobItemIDsEmpty    bool
	HasIdentityLinks   bool
	IdentityLinksEmpty bool
}

type validatedRequest struct {
	SchemaVersion  int
	RequestID      string
	AgentID        string
	Server         string
	ServerKey      string
	Players        []validatedPlayer
	ServerMetadata *ServerMetadata
	Coverage       *Coverage
	CanonicalJSON  []byte
}

func (vp validatedPlayer) hasStorableFields() bool {
	return vp.HasModel || vp.HasGear || vp.HasAvatarGear || vp.HasLevel || vp.HasJob ||
		vp.HasJobLevel || vp.HasRace || vp.HasGuild || vp.HasPosition || vp.HasJobItemIDs ||
		vp.HasIdentityLinks
}

func parseFieldEvidence(raw map[string]json.RawMessage) (map[string]FieldEvidence, error) {
	if raw == nil {
		return nil, ErrInvalid
	}
	allowed := map[string]bool{"name": true, "model_ref_id": true, "gear": true}
	out := make(map[string]FieldEvidence, len(raw))
	for key, value := range raw {
		if !allowed[key] {
			return nil, ErrInvalid
		}
		var ev FieldEvidence
		if err := json.Unmarshal(value, &ev); err != nil {
			return nil, ErrInvalid
		}
		if err := validateEvidenceV1(ev); err != nil {
			return nil, err
		}
		out[key] = ev
	}
	return out, nil
}

type wirePlayer struct {
	Name          string                     `json:"name"`
	IdentityKind  string                     `json:"identity_kind"`
	ModelRefID    *int                       `json:"model_ref_id"`
	Gear          *[]GearEntry               `json:"gear"`
	FieldEvidence map[string]json.RawMessage `json:"field_evidence"`
}

type wireRequest struct {
	SchemaVersion  int             `json:"schema_version"`
	RequestID      string          `json:"request_id"`
	AgentID        string          `json:"agent_id"`
	Server         string          `json:"server"`
	Players        []wirePlayer    `json:"players"`
	ServerMetadata *ServerMetadata `json:"server_metadata"`
	Coverage       *Coverage       `json:"coverage"`
}

func validServerName(server string) bool {
	server = strings.TrimSpace(server)
	return server != "" && len(server) <= 100 && !strings.ContainsRune(server, 0)
}

func validPlayerNameV1(name string) bool {
	if name == "" || name != strings.TrimSpace(name) || strings.ContainsRune(name, 0) {
		return false
	}
	return len(name) <= MaxPlayerNameBytesV1
}

func validPlayerNameV2(name string) bool {
	if name == "" || name != strings.TrimSpace(name) || strings.ContainsRune(name, 0) {
		return false
	}
	return len(name) <= MaxPlayerNameBytesV2
}

func validRefID(id int) bool {
	return id >= 1 && id <= 2147483647
}

func validGatewayCount(v *int) bool {
	if v == nil {
		return true
	}
	return *v >= 0 && *v <= 1_000_000
}

func validOptOrSlot(v *int) bool {
	if v == nil {
		return true
	}
	return *v >= 0 && *v <= 255
}

func serverKey(server string) string {
	return strings.ToLower(strings.TrimSpace(server))
}

func nameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func ValidateSubmitBody(body []byte, authenticatedAgentID string) (validatedRequest, error) {
	if len(body) == 0 || len(body) > MaxBodyBytes {
		return validatedRequest{}, ErrInvalid
	}
	var peek struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		return validatedRequest{}, ErrInvalid
	}
	switch peek.SchemaVersion {
	case SchemaVersionV1:
		return validateSubmitV1(body, authenticatedAgentID)
	case SchemaVersionV2:
		return validateSubmitV2(body, authenticatedAgentID)
	default:
		return validatedRequest{}, ErrInvalid
	}
}

func validateSubmitV1(body []byte, authenticatedAgentID string) (validatedRequest, error) {
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	var wire wireRequest
	if err := dec.Decode(&wire); err != nil {
		return validatedRequest{}, ErrInvalid
	}
	if dec.More() {
		return validatedRequest{}, ErrInvalid
	}
	if wire.SchemaVersion != SchemaVersionV1 {
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
	for _, p := range wire.Players {
		if p.IdentityKind != "character_name" {
			return validatedRequest{}, ErrInvalid
		}
		if !validPlayerNameV1(p.Name) {
			return validatedRequest{}, ErrInvalid
		}
		sk := serverKey(wire.Server)
		nk := nameKey(p.Name)
		key := sk + "\x00" + nk
		if _, dup := seen[key]; dup {
			return validatedRequest{}, ErrInvalid
		}
		seen[key] = struct{}{}
		evidence, err := parseFieldEvidence(p.FieldEvidence)
		if err != nil {
			return validatedRequest{}, err
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
			if !validRefID(*p.ModelRefID) {
				return validatedRequest{}, ErrInvalid
			}
			if _, ok := evidence["model_ref_id"]; !ok {
				return validatedRequest{}, ErrInvalid
			}
			vp.ModelRefID = p.ModelRefID
			vp.HasModel = true
		}
		if p.Gear != nil {
			if _, ok := evidence["gear"]; !ok {
				return validatedRequest{}, ErrInvalid
			}
			if len(*p.Gear) > MaxGearPerPlayer {
				return validatedRequest{}, ErrInvalid
			}
			for _, g := range *p.Gear {
				if !validRefID(g.RefItemID) || !validOptOrSlot(g.OptLevel) || !validOptOrSlot(g.Slot) {
					return validatedRequest{}, ErrInvalid
				}
			}
			vp.Gear = *p.Gear
			vp.HasGear = true
			vp.GearEmpty = len(*p.Gear) == 0
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
		SchemaVersion:  SchemaVersionV1,
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

// AgentIDMismatch is returned when body agent_id does not match the token.
type AgentIDMismatch struct{}

func (AgentIDMismatch) Error() string { return "agent id mismatch" }

func ValidateAgentIDField(body []byte, authenticatedAgentID string) error {
	var partial struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(body, &partial); err != nil {
		return ErrInvalid
	}
	if partial.AgentID != authenticatedAgentID {
		return AgentIDMismatch{}
	}
	return nil
}

func advisoryLockKey(agentID, requestID string) (int64, error) {
	var a, b int64
	parts := strings.Split(agentID, "-")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid agent id")
	}
	for i, hex := range parts[:2] {
		var v int64
		for _, ch := range hex {
			v = v * 16
			switch {
			case ch >= '0' && ch <= '9':
				v += int64(ch - '0')
			case ch >= 'a' && ch <= 'f':
				v += int64(ch - 'a' + 10)
			default:
				return 0, fmt.Errorf("invalid agent id")
			}
		}
		if i == 0 {
			a = v
		} else {
			b = v
		}
	}
	var c int64
	for _, ch := range requestID[:8] {
		c = c * 16
		switch {
		case ch >= '0' && ch <= '9':
			c += int64(ch - '0')
		case ch >= 'a' && ch <= 'f':
			c += int64(ch - 'a' + 10)
		default:
			return 0, fmt.Errorf("invalid request id")
		}
	}
	return a ^ (b << 16) ^ c, nil
}

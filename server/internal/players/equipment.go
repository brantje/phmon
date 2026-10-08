package players

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

var EquipmentSlots = []string{"head", "chest", "shoulder", "hands", "legs", "feet", "weapon", "shield", "earring", "necklace", "ring_left", "ring_right", "job", "avatar_head", "avatar_body", "avatar_attachment", "avatar_flag"}

type EquipmentSlot struct {
	Slot         string               `json:"slot"`
	State        string               `json:"state"`
	ModelID      *int64               `json:"model_id,omitempty"`
	Plus         *int                 `json:"plus,omitempty"`
	Variance     *string              `json:"variance,omitempty"`
	Durability   *uint32              `json:"durability,omitempty"`
	MagicOptions map[string]string    `json:"magic_options,omitempty"`
	Item         map[string]any       `json:"item,omitempty"`
	ObservedAt   time.Time            `json:"observed_at"`
	FieldTimes   map[string]time.Time `json:"field_times,omitempty"`
}

func slotTimes(s EquipmentSlot, fallback time.Time) EquipmentSlot {
	times := map[string]time.Time{}
	for k, v := range s.FieldTimes {
		times[k] = v
	}
	stamp := s.ObservedAt
	if stamp.IsZero() {
		stamp = fallback
	}
	for key, present := range map[string]bool{"state": true, "configuration": true, "model": s.ModelID != nil, "plus": s.Plus != nil, "variance": s.Variance != nil, "durability": s.Durability != nil, "magic_options": s.MagicOptions != nil, "item": s.Item != nil} {
		if present && times[key].IsZero() {
			times[key] = stamp
		}
	}
	s.FieldTimes = times
	return s
}

type Equipment struct {
	Availability     string          `json:"availability"`
	LastAvailability string          `json:"last_availability,omitempty"`
	LastAttemptAt    time.Time       `json:"last_attempt_at,omitempty"`
	Source           string          `json:"source"`
	Profile          string          `json:"profile"`
	Slots            []EquipmentSlot `json:"slots"`
	ObservedAt       time.Time       `json:"observed_at"`
	// This is a backend evidence gate, not a claim accepted from an agent.
	IdentityVerified bool   `json:"identity_verified"`
	CharacterModel   *int64 `json:"character_model_id,omitempty"`
}

func (e Equipment) Validate() error {
	if e.Availability != "observed_complete" && e.Availability != "observed_partial" && e.Availability != "unavailable" || len(e.Slots) > len(EquipmentSlots) || !validText(e.Source, 64) || !validText(e.Profile, 64) {
		return ErrInvalid
	}
	if e.Availability == "unavailable" && len(e.Slots) != 0 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, s := range e.Slots {
		valid := false
		for _, name := range EquipmentSlots {
			if name == s.Slot {
				valid = true
			}
		}
		if !valid || seen[s.Slot] || s.State != "unknown" && s.State != "empty" && s.State != "occupied" {
			return ErrInvalid
		}
		seen[s.Slot] = true
		if s.State == "occupied" && (s.ModelID == nil || *s.ModelID < 1 || *s.ModelID > 4294967295) || s.State != "occupied" && (s.ModelID != nil || s.Plus != nil || s.Variance != nil || s.Item != nil || s.MagicOptions != nil || s.Durability != nil) || s.Plus != nil && (*s.Plus < 0 || *s.Plus > 255) {
			return ErrInvalid
		}
		if s.Variance != nil {
			if _, err := strconv.ParseUint(*s.Variance, 10, 64); err != nil {
				return ErrInvalid
			}
		}
		if len(s.MagicOptions) > 32 {
			return ErrInvalid
		}
		for k, v := range s.MagicOptions {
			if _, err := strconv.ParseUint(k, 10, 32); err != nil {
				return ErrInvalid
			}
			if _, err := strconv.ParseUint(v, 10, 64); err != nil {
				return ErrInvalid
			}
		}
	}
	if e.Availability == "observed_complete" {
		for _, name := range EquipmentSlots[:12] {
			known := false
			for _, s := range e.Slots {
				if s.Slot == name && s.State != "unknown" {
					known = true
				}
			}
			if !known {
				return ErrInvalid
			}
		}
	}
	return nil
}

func gearHash(e *Equipment) string {
	if e == nil || e.Availability == "unavailable" {
		return ""
	}
	type persistentSlot struct {
		Slot, State string
		Model       *int64
		Plus        *int
		Variance    *string
		Options     map[string]string
	}
	slots := []persistentSlot{}
	for _, s := range e.Slots {
		if s.State == "unknown" {
			continue
		}
		// Item is an enriched presentation adapter. Only normalized observed
		// instance fields enter a fingerprint; catalog/reference keys never do.
		slots = append(slots, persistentSlot{s.Slot, s.State, s.ModelID, s.Plus, s.Variance, s.MagicOptions})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].Slot < slots[j].Slot })
	b, _ := json.Marshal(slots)
	sum := sha256.Sum256(b)
	return "gear-v1:" + hex.EncodeToString(sum[:])
}

func identityHash(e *Equipment) string {
	if e == nil || !e.IdentityVerified || e.CharacterModel == nil || e.Profile == "" {
		return ""
	}
	type slot struct {
		Slot, State string
		Model       *int64
		Plus        *int
	}
	slots := []slot{}
	for _, s := range e.Slots {
		if normalSlot(s.Slot) && s.State != "unknown" {
			slots = append(slots, slot{s.Slot, s.State, s.ModelID, s.Plus})
		}
	}
	if len(slots) < 4 {
		return ""
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].Slot < slots[j].Slot })
	b, _ := json.Marshal(struct {
		Version string
		Profile string
		Model   int64
		Slots   []slot
	}{"identity-v1", e.Profile, *e.CharacterModel, slots})
	sum := sha256.Sum256(b)
	return "identity-v1:" + hex.EncodeToString(sum[:])
}
func normalSlot(s string) bool {
	for _, n := range EquipmentSlots[:12] {
		if n == s {
			return true
		}
	}
	return false
}

// mergeEquipment keeps absent fields, but never transfers stats to a replacement
// item. Slot timestamps keep historical knowledge visible as historical knowledge.
func mergeEquipment(old, incoming *Equipment) *Equipment {
	if incoming == nil {
		return old
	}
	if incoming.Availability == "unavailable" && old != nil {
		copy := *old
		if !incoming.ObservedAt.Before(old.LastAttemptAt) {
			copy.LastAvailability = "unavailable"
			copy.LastAttemptAt = incoming.ObservedAt
		}
		return &copy
	}
	if old == nil {
		copy := *incoming
		copy.LastAvailability = incoming.Availability
		copy.LastAttemptAt = incoming.ObservedAt
		copy.Slots = append([]EquipmentSlot{}, incoming.Slots...)
		for i := range copy.Slots {
			if copy.Slots[i].ObservedAt.IsZero() {
				copy.Slots[i].ObservedAt = incoming.ObservedAt
			}
			copy.Slots[i] = slotTimes(copy.Slots[i], incoming.ObservedAt)
		}
		return &copy
	}
	result := *old
	if !incoming.ObservedAt.Before(old.LastAttemptAt) {
		result.LastAvailability = incoming.Availability
		result.LastAttemptAt = incoming.ObservedAt
	}
	bySlot := map[string]EquipmentSlot{}
	for _, s := range old.Slots {
		bySlot[s.Slot] = slotTimes(s, old.ObservedAt)
	}
	for _, s := range incoming.Slots {
		if s.State == "unknown" {
			continue
		}
		if s.ObservedAt.IsZero() {
			s.ObservedAt = incoming.ObservedAt
		}
		s = slotTimes(s, incoming.ObservedAt)
		previous, ok := bySlot[s.Slot]
		stable := ok && s.State == "occupied" && previous.State == "occupied" && *s.ModelID == *previous.ModelID && (s.Plus == nil || previous.Plus == nil || *s.Plus == *previous.Plus)
		if ok && previous.ObservedAt.After(s.ObservedAt) {
			// Late attributes may fill earlier unknown fields on the same observed
			// configuration. Never attach them across a known replacement/reversal.
			if !stable || s.ObservedAt.Before(previous.FieldTimes["configuration"]) {
				continue
			}
			s.State = previous.State
			s.ModelID = previous.ModelID
			s.FieldTimes["state"] = previous.FieldTimes["state"]
			s.FieldTimes["model"] = previous.FieldTimes["model"]
			s.ObservedAt = previous.ObservedAt
		}
		if stable {
			s.FieldTimes["configuration"] = previous.FieldTimes["configuration"]
			keep := func(key string, missing bool) bool {
				return missing || s.FieldTimes[key].Before(previous.FieldTimes[key])
			}
			if keep("plus", s.Plus == nil) {
				s.Plus = previous.Plus
				s.FieldTimes["plus"] = previous.FieldTimes["plus"]
			}
			if keep("variance", s.Variance == nil) {
				s.Variance = previous.Variance
				s.FieldTimes["variance"] = previous.FieldTimes["variance"]
			}
			if keep("magic_options", s.MagicOptions == nil) {
				s.MagicOptions = previous.MagicOptions
				s.FieldTimes["magic_options"] = previous.FieldTimes["magic_options"]
			}
			if keep("durability", s.Durability == nil) {
				s.Durability = previous.Durability
				s.FieldTimes["durability"] = previous.FieldTimes["durability"]
			}
			if keep("item", s.Item == nil) {
				s.Item = previous.Item
				s.FieldTimes["item"] = previous.FieldTimes["item"]
			}
		}
		bySlot[s.Slot] = s
	}
	result.Slots = []EquipmentSlot{}
	for _, s := range bySlot {
		result.Slots = append(result.Slots, s)
	}
	sort.Slice(result.Slots, func(i, j int) bool { return result.Slots[i].Slot < result.Slots[j].Slot })
	if !incoming.ObservedAt.Before(old.ObservedAt) {
		result.Source = incoming.Source
		result.Profile = incoming.Profile
		result.ObservedAt = incoming.ObservedAt
		result.IdentityVerified = incoming.IdentityVerified
		result.CharacterModel = incoming.CharacterModel
	}
	result.Availability = "observed_partial"
	complete := true
	for _, s := range EquipmentSlots[:12] {
		v, ok := bySlot[s]
		if !ok || v.State == "unknown" {
			complete = false
		}
	}
	if complete {
		result.Availability = "observed_complete"
	}
	return &result
}

// ComparableEquipment does not equate incomplete sets. It reports overlapping
// verified occupied normal slots and conflicts, never a merge decision.
func ComparableEquipment(a, b *Equipment) (matching, comparable int, conflict bool) {
	if a == nil || b == nil || !a.IdentityVerified || !b.IdentityVerified || a.CharacterModel == nil || b.CharacterModel == nil || a.Profile == "" || a.Profile != b.Profile {
		return
	}
	if *a.CharacterModel != *b.CharacterModel {
		return 0, 0, true
	}
	bySlot := map[string]EquipmentSlot{}
	for _, s := range a.Slots {
		bySlot[s.Slot] = s
	}
	for _, s := range b.Slots {
		p, ok := bySlot[s.Slot]
		if !ok || !normalSlot(s.Slot) || p.State == "unknown" || s.State == "unknown" {
			continue
		}
		if p.State != s.State {
			conflict = true
			continue
		}
		if s.State != "occupied" || p.Plus == nil || s.Plus == nil {
			continue
		}
		comparable++
		if *p.ModelID == *s.ModelID && *p.Plus == *s.Plus {
			matching++
		} else {
			conflict = true
		}
	}
	return
}

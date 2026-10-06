package tradenexus

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"phmon/server/internal/events"
)

// SightingFromEvent maps a newly stored PhMon thief callback into a relay
// sighting. Duplicate or replayed events must not call this.
func SightingFromEvent(event events.Event) (Sighting, bool) {
	if event.Kind != "job.thief_seen" {
		return Sighting{}, false
	}
	var payload struct {
		Value string `json:"value"`
		Thief *struct {
			Name   string   `json:"name"`
			Region *int     `json:"region"`
			X      *float64 `json:"x"`
			Y      *float64 `json:"y"`
		} `json:"thief"`
		PositionSource string `json:"position_source"`
		PluginVersion  string `json:"plugin_version"`
	}
	if len(event.Payload) > 0 && json.Unmarshal(event.Payload, &payload) != nil {
		return Sighting{}, false
	}
	name := strings.TrimSpace(payload.Value)
	if payload.Thief != nil && strings.TrimSpace(payload.Thief.Name) != "" {
		name = strings.TrimSpace(payload.Thief.Name)
	}
	name = boundBytes(name, MaxNameBytes)
	server, ok := validServer(event.Server)
	if !ok || name == "" {
		return Sighting{}, false
	}
	version, _ := optionalText(payload.PluginVersion, MaxNameBytes)
	reporter, _ := optionalText(event.Character, MaxNameBytes)
	sighting := Sighting{
		Server: server, ThiefName: name, PositionSource: payload.PositionSource,
		Reporter:   Reporter{Name: reporter, App: "PhMon", Version: version},
		Origin:     OriginPhMon,
		EventID:    event.ID,
		ObservedAt: event.OccurredAt.UTC(),
	}
	if payload.Thief != nil && payload.Thief.Region != nil && payload.Thief.X != nil && payload.Thief.Y != nil &&
		payload.PositionSource == SourceThief && validRegion(*payload.Thief.Region) &&
		finiteCoord(*payload.Thief.X) && finiteCoord(*payload.Thief.Y) {
		sighting.Position = &Position{Region: *payload.Thief.Region, X: *payload.Thief.X, Y: *payload.Thief.Y}
		sighting.PositionSource = SourceThief
		return sighting, true
	}
	if event.Region != nil && event.X != nil && event.Y != nil && validRegion(*event.Region) && finiteCoord(*event.X) && finiteCoord(*event.Y) {
		z := event.Z
		if z != nil && !finiteCoord(*z) {
			z = nil
		}
		sighting.Position = &Position{Region: *event.Region, X: *event.X, Y: *event.Y, Z: z}
		sighting.PositionSource = SourceObserver
		return sighting, true
	}
	sighting.PositionSource = SourceUnknown
	return sighting, true
}

func boundBytes(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	for len(value) > maxBytes {
		_, size := utf8.DecodeLastRuneInString(value)
		if size <= 0 {
			break
		}
		value = value[:len(value)-size]
	}
	return strings.TrimSpace(value)
}

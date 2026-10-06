package tradenexus

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	ProtocolVersion    = 1
	MaxFrameBytes      = 4096
	MaxServers         = 25
	MaxServerRunes     = 100
	MaxNameBytes       = 64
	MaxRefBytes        = 64
	ReportsPerWindow   = 100
	ReportWindow       = 5 * time.Second
	MaxConnections     = 5000
	MaxConnectionsIP   = 25
	SendQueue          = 64
	MaxInvalidFrames   = 5
	ReadLimitBytes     = 8192
	ObservedAtSkew     = 5 * time.Minute
	MarkerTTL          = 5 * time.Minute
	MaxRecentSightings = 256
	MaxPageSize        = 100

	OriginPhMon    = "phmon"
	OriginExternal = "external"

	SourceThief    = "thief"
	SourceObserver = "observer"
	SourceUnknown  = "unknown"
)

var errInvalidSighting = errors.New("invalid tradenexus sighting")

type Position struct {
	Region int      `json:"region"`
	X      float64  `json:"x"`
	Y      float64  `json:"y"`
	Z      *float64 `json:"z,omitempty"`
}

type Reporter struct {
	Name    string `json:"name,omitempty"`
	App     string `json:"app,omitempty"`
	Version string `json:"version,omitempty"`
}

type Sighting struct {
	ID             string    `json:"sighting_id"`
	Server         string    `json:"server"`
	ThiefName      string    `json:"thief_name"`
	Position       *Position `json:"position,omitempty"`
	PositionSource string    `json:"position_source"`
	Reporter       Reporter  `json:"reporter"`
	Origin         string    `json:"origin"`
	EventID        string    `json:"event_id,omitempty"`
	ObservedAt     time.Time `json:"observed_at"`
	ReceivedAt     time.Time `json:"received_at"`
}

type clientFrame struct {
	V       int      `json:"v"`
	Type    string   `json:"type"`
	Ref     string   `json:"ref"`
	Servers []string `json:"servers"`
	Server  string   `json:"server"`
	Thief   struct {
		Name string `json:"name"`
	} `json:"thief"`
	Position *struct {
		Region float64  `json:"region"`
		X      float64  `json:"x"`
		Y      float64  `json:"y"`
		Z      *float64 `json:"z"`
	} `json:"position"`
	PositionSource string `json:"position_source"`
	Reporter       struct {
		Name    string `json:"name"`
		App     string `json:"app"`
		Version string `json:"version"`
	} `json:"reporter"`
	ObservedAt string `json:"observed_at"`
}

type parseError struct {
	Code    string
	Message string
	Ref     string
}

func (e parseError) Error() string { return e.Message }

func newSightingID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(buf)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func validRegion(region int) bool {
	return region != 0 && region >= -32768 && region <= 65535
}

func finiteCoord(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -1000000 && value <= 1000000
}

func optionalText(value string, maxBytes int) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) > maxBytes || strings.ContainsFunc(value, unicode.IsControl) {
		return "", false
	}
	return value, true
}

func requiredText(value string, maxBytes int) (string, bool) {
	value, ok := optionalText(value, maxBytes)
	return value, ok && value != ""
}

func validServer(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > MaxServerRunes || strings.ContainsFunc(value, unicode.IsControl) {
		return "", false
	}
	return value, true
}

func parseClientFrame(payload []byte) (clientFrame, parseError) {
	if len(payload) > MaxFrameBytes {
		return clientFrame{}, parseError{Code: "too_large", Message: "frame exceeds 4096 bytes"}
	}
	var frame clientFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return clientFrame{}, parseError{Code: "invalid_message", Message: "frame must be a JSON object"}
	}
	if frame.V != ProtocolVersion {
		return clientFrame{}, parseError{Code: "unsupported_version", Message: "unsupported protocol version", Ref: safeRef(frame.Ref)}
	}
	ref, refOK := optionalText(frame.Ref, MaxRefBytes)
	if !refOK {
		return clientFrame{}, parseError{Code: "invalid_message", Message: "ref must be at most 64 bytes"}
	}
	frame.Ref = ref
	switch frame.Type {
	case "subscribe", "thief.report":
		return frame, parseError{}
	default:
		return clientFrame{}, parseError{Code: "unsupported_type", Message: "unsupported message type", Ref: frame.Ref}
	}
}

func safeRef(value string) string {
	value, ok := optionalText(value, MaxRefBytes)
	if !ok {
		return ""
	}
	return value
}

func serverLimitMessage() string {
	return fmt.Sprintf("servers must contain 1 to %d names", MaxServers)
}

func subscribeServers(frame clientFrame) ([]string, parseError) {
	if len(frame.Servers) == 0 || len(frame.Servers) > MaxServers {
		return nil, parseError{Code: "invalid_message", Message: serverLimitMessage(), Ref: frame.Ref}
	}
	servers := make([]string, 0, len(frame.Servers))
	for _, raw := range frame.Servers {
		name, ok := validServer(raw)
		if !ok {
			return nil, parseError{Code: "invalid_message", Message: "server name is empty or too long", Ref: frame.Ref}
		}
		duplicate := false
		for _, existing := range servers {
			if strings.EqualFold(existing, name) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			servers = append(servers, name)
		}
	}
	if len(servers) == 0 {
		return nil, parseError{Code: "invalid_message", Message: serverLimitMessage(), Ref: frame.Ref}
	}
	return servers, parseError{}
}

func reportSighting(frame clientFrame, now time.Time) (Sighting, parseError) {
	server, ok := validServer(frame.Server)
	if !ok {
		return Sighting{}, parseError{Code: "invalid_message", Message: "server is required", Ref: frame.Ref}
	}
	name, ok := requiredText(frame.Thief.Name, MaxNameBytes)
	if !ok {
		return Sighting{}, parseError{Code: "invalid_message", Message: "thief.name is required", Ref: frame.Ref}
	}
	reporterName, ok := optionalText(frame.Reporter.Name, MaxNameBytes)
	if !ok {
		return Sighting{}, parseError{Code: "invalid_message", Message: "reporter.name is too long", Ref: frame.Ref}
	}
	reporterApp, ok := optionalText(frame.Reporter.App, MaxNameBytes)
	if !ok {
		return Sighting{}, parseError{Code: "invalid_message", Message: "reporter.app is too long", Ref: frame.Ref}
	}
	reporterVersion, ok := optionalText(frame.Reporter.Version, MaxNameBytes)
	if !ok {
		return Sighting{}, parseError{Code: "invalid_message", Message: "reporter.version is too long", Ref: frame.Ref}
	}
	source := frame.PositionSource
	if source == "" {
		source = SourceUnknown
	}
	if source != SourceThief && source != SourceObserver && source != SourceUnknown {
		return Sighting{}, parseError{Code: "invalid_message", Message: "position_source is invalid", Ref: frame.Ref}
	}
	var position *Position
	if frame.Position != nil {
		region := frame.Position.Region
		if region != math.Trunc(region) || !validRegion(int(region)) || !finiteCoord(frame.Position.X) || !finiteCoord(frame.Position.Y) {
			return Sighting{}, parseError{Code: "invalid_message", Message: "position is invalid", Ref: frame.Ref}
		}
		if frame.Position.Z != nil && !finiteCoord(*frame.Position.Z) {
			return Sighting{}, parseError{Code: "invalid_message", Message: "position is invalid", Ref: frame.Ref}
		}
		position = &Position{Region: int(region), X: frame.Position.X, Y: frame.Position.Y, Z: frame.Position.Z}
	}
	if source == SourceThief && position == nil {
		return Sighting{}, parseError{Code: "invalid_message", Message: "thief position is required", Ref: frame.Ref}
	}
	if position == nil {
		source = SourceUnknown
	}
	received := now.UTC()
	observed := received
	if frame.ObservedAt != "" {
		parsed, err := time.Parse(time.RFC3339, frame.ObservedAt)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, frame.ObservedAt)
		}
		if err != nil {
			return Sighting{}, parseError{Code: "invalid_message", Message: "observed_at is invalid", Ref: frame.Ref}
		}
		if parsed.After(received.Add(ObservedAtSkew)) || parsed.Before(received.Add(-ObservedAtSkew)) {
			observed = received
		} else {
			observed = parsed.UTC()
		}
	}
	return Sighting{
		Server: server, ThiefName: name, Position: position, PositionSource: source,
		Reporter:   Reporter{Name: reporterName, App: reporterApp, Version: reporterVersion},
		Origin:     OriginExternal,
		ObservedAt: observed, ReceivedAt: received,
	}, parseError{}
}

type wirePosition struct {
	Region int      `json:"region"`
	X      float64  `json:"x"`
	Y      float64  `json:"y"`
	Z      *float64 `json:"z,omitempty"`
}

type wireReporter struct {
	Name    string `json:"name,omitempty"`
	App     string `json:"app,omitempty"`
	Version string `json:"version,omitempty"`
}

type wireSighting struct {
	V          int    `json:"v"`
	Type       string `json:"type"`
	SightingID string `json:"sighting_id"`
	Server     string `json:"server"`
	Thief      struct {
		Name string `json:"name"`
	} `json:"thief"`
	Position       *wirePosition `json:"position,omitempty"`
	PositionSource string        `json:"position_source"`
	Reporter       *wireReporter `json:"reporter,omitempty"`
	Origin         string        `json:"origin"`
	ObservedAt     string        `json:"observed_at"`
	ReceivedAt     string        `json:"received_at"`
}

func marshalSighting(sighting Sighting) ([]byte, error) {
	frame := wireSighting{
		V: ProtocolVersion, Type: "thief.sighting", SightingID: sighting.ID,
		Server: sighting.Server, PositionSource: sighting.PositionSource, Origin: sighting.Origin,
		ObservedAt: sighting.ObservedAt.UTC().Format(time.RFC3339Nano),
		ReceivedAt: sighting.ReceivedAt.UTC().Format(time.RFC3339Nano),
	}
	frame.Thief.Name = sighting.ThiefName
	if sighting.Position != nil {
		frame.Position = &wirePosition{
			Region: sighting.Position.Region, X: sighting.Position.X, Y: sighting.Position.Y, Z: sighting.Position.Z,
		}
	}
	if sighting.Reporter.Name != "" || sighting.Reporter.App != "" || sighting.Reporter.Version != "" {
		frame.Reporter = &wireReporter{Name: sighting.Reporter.Name, App: sighting.Reporter.App, Version: sighting.Reporter.Version}
	}
	return json.Marshal(frame)
}

func errorFrame(ref, code, message string) []byte {
	frame := map[string]any{"v": ProtocolVersion, "type": "error", "code": code, "message": message}
	if ref != "" {
		frame["ref"] = ref
	}
	payload, _ := json.Marshal(frame)
	return payload
}

func ackFrame(ref, sightingID string) []byte {
	frame := map[string]any{"v": ProtocolVersion, "type": "ack", "sighting_id": sightingID}
	if ref != "" {
		frame["ref"] = ref
	}
	payload, _ := json.Marshal(frame)
	return payload
}

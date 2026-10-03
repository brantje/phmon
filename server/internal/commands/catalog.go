package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"unicode"

	"phmon/server/internal/chat"
)

const (
	maxCoordinate = 10_000_000
	maxRadius     = 10_000
)

type noArgs struct{}

type reverseReturnArgs struct {
	Type *int   `json:"type"`
	Name string `json:"name,omitempty"`
}

type traceArgs struct {
	Name string `json:"name"`
}

type radiusArgs struct {
	Radius float64 `json:"radius"`
}

type walkArgs struct {
	Region int     `json:"region"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
}

type navigateStopArgs struct {
	CommandID     string `json:"command_id"`
	RouteSequence uint64 `json:"route_sequence"`
}

type teleportArgs struct {
	Source         string `json:"source"`
	Destination    string `json:"destination"`
	GateServername string `json:"gate_servername"`
}

type trainingAreaArgs struct {
	Mode   string   `json:"mode"`
	Name   *string  `json:"name,omitempty"`
	Region *int     `json:"region,omitempty"`
	X      *float64 `json:"x,omitempty"`
	Y      *float64 `json:"y,omitempty"`
	Z      *float64 `json:"z,omitempty"`
}

func Validate(name string, raw json.RawMessage, confirmation bool) (Validated, error) {
	name = strings.TrimSpace(name)
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	switch name {
	case "bot.start", "bot.stop", "trace.stop", "character.return", "character.disconnect", "client.clientless":
		var args noArgs
		if err := decodeExact(raw, &args); err != nil {
			return Validated{}, ErrInvalid
		}
		if (name == "character.return" || name == "character.disconnect" || name == "client.clientless") && !confirmation {
			return Validated{}, ErrInvalid
		}
		normalized, _ := json.Marshal(args)
		return Validated{Name: name, Args: normalized, Confirmation: confirmation}, nil
	case "character.reverse_return":
		var args reverseReturnArgs
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || bytes.Equal(fields["name"], []byte("null")) {
			return Validated{}, ErrInvalid
		}
		if decodeExact(raw, &args) != nil || args.Type == nil || *args.Type < 0 || *args.Type > 3 || !confirmation {
			return Validated{}, ErrInvalid
		}
		for _, r := range args.Name {
			if unicode.IsControl(r) {
				return Validated{}, ErrInvalid
			}
		}
		if *args.Type < 2 && args.Name != "" {
			return Validated{}, ErrInvalid
		}
		args.Name = strings.TrimSpace(args.Name)
		if len(args.Name) > 100 || (*args.Type < 2 && args.Name != "") || (*args.Type >= 2 && args.Name == "") {
			return Validated{}, ErrInvalid
		}
		normalized, _ := json.Marshal(args)
		return Validated{Name: name, Args: normalized, Confirmation: confirmation}, nil
	case "trace.start":
		var args traceArgs
		if err := decodeExact(raw, &args); err != nil {
			return Validated{}, ErrInvalid
		}
		args.Name = strings.TrimSpace(args.Name)
		if len(args.Name) < 1 || len(args.Name) > 64 || strings.ContainsRune(args.Name, 0) {
			return Validated{}, ErrInvalid
		}
		normalized, _ := json.Marshal(args)
		return Validated{Name: name, Args: normalized, Confirmation: confirmation}, nil
	case "training.radius.set":
		var args radiusArgs
		if err := decodeExact(raw, &args); err != nil || !finite(args.Radius) || args.Radius < 1 || args.Radius > maxRadius {
			return Validated{}, ErrInvalid
		}
		normalized, _ := json.Marshal(args)
		return Validated{Name: name, Args: normalized, Confirmation: confirmation}, nil
	case "character.walk", "character.navigate":
		var args walkArgs
		if err := decodeExact(raw, &args); err != nil || !validRegion(args.Region) || (name == "character.walk" && args.Region < 0) || !coordinate(args.X) || !coordinate(args.Y) || !coordinate(args.Z) {
			return Validated{}, ErrInvalid
		}
		normalized, _ := json.Marshal(args)
		return Validated{Name: name, Args: normalized, Confirmation: confirmation}, nil
	case "character.navigate.stop":
		var args navigateStopArgs
		if err := decodeExact(raw, &args); err != nil {
			return Validated{}, ErrInvalid
		}
		args.CommandID = strings.TrimSpace(args.CommandID)
		if len(args.CommandID) != 40 || !strings.HasPrefix(args.CommandID, "cmd_") || strings.ContainsRune(args.CommandID, 0) || args.RouteSequence == 0 {
			return Validated{}, ErrInvalid
		}
		normalized, _ := json.Marshal(args)
		return Validated{Name: name, Args: normalized, Confirmation: confirmation}, nil
	case "character.teleport":
		var args teleportArgs
		if err := decodeExact(raw, &args); err != nil {
			return Validated{}, ErrInvalid
		}
		args.Source = strings.TrimSpace(args.Source)
		args.Destination = strings.TrimSpace(args.Destination)
		args.GateServername = strings.TrimSpace(args.GateServername)
		if !teleportLabel(args.Source) || !teleportLabel(args.Destination) || !teleportGateServername(args.GateServername) {
			return Validated{}, ErrInvalid
		}
		normalized, _ := json.Marshal(args)
		return Validated{Name: name, Args: normalized, Confirmation: confirmation}, nil
	case "training.area.set":
		var args trainingAreaArgs
		if err := decodeExact(raw, &args); err != nil {
			return Validated{}, ErrInvalid
		}
		args.Mode = strings.TrimSpace(args.Mode)
		switch args.Mode {
		case "current_position":
			if args.Name != nil || args.Region != nil || args.X != nil || args.Y != nil || args.Z != nil {
				return Validated{}, ErrInvalid
			}
		case "position":
			if args.Name != nil || args.Region == nil || !validRegion(*args.Region) || args.X == nil || args.Y == nil || args.Z == nil ||
				!coordinate(*args.X) || !coordinate(*args.Y) || !coordinate(*args.Z) {
				return Validated{}, ErrInvalid
			}
		case "named":
			if args.Name == nil || args.Region != nil || args.X != nil || args.Y != nil || args.Z != nil {
				return Validated{}, ErrInvalid
			}
			value := strings.TrimSpace(*args.Name)
			if len(value) < 1 || len(value) > 100 || strings.ContainsRune(value, 0) {
				return Validated{}, ErrInvalid
			}
			args.Name = &value
		default:
			return Validated{}, ErrInvalid
		}
		normalized, _ := json.Marshal(args)
		return Validated{Name: name, Args: normalized, Confirmation: confirmation}, nil
	case "chat.send":
		var args struct {
			Channel   string  `json:"channel"`
			Text      string  `json:"text"`
			Recipient *string `json:"recipient,omitempty"`
		}
		if err := decodeExact(raw, &args); err != nil {
			return Validated{}, ErrInvalid
		}
		args.Channel = strings.TrimSpace(args.Channel)
		recipient := ""
		if args.Recipient != nil {
			recipient = strings.TrimSpace(*args.Recipient)
			args.Recipient = &recipient
		}
		if chat.ValidateOutbound(args.Channel, args.Text, recipient) != nil ||
			args.Channel == "global" && !confirmation {
			return Validated{}, ErrInvalid
		}
		normalized, _ := json.Marshal(args)
		return Validated{Name: name, Args: normalized, Confirmation: confirmation}, nil
	default:
		return Validated{}, ErrUnsupported
	}
}

func decodeExact(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func coordinate(value float64) bool {
	return finite(value) && math.Abs(value) <= maxCoordinate
}

func validRegion(value int) bool { return value >= -32768 && value <= 65535 && value != 0 }

func teleportLabel(value string) bool {
	if len(value) < 1 || len(value) > 64 || strings.ContainsRune(value, 0) ||
		strings.Contains(value, ",") || strings.Contains(value, "\n") || strings.Contains(value, "\r") {
		return false
	}
	return true
}

func teleportGateServername(value string) bool {
	if len(value) < 6 || len(value) > 64 || strings.ContainsRune(value, 0) {
		return false
	}
	if !strings.HasPrefix(value, "GATE_") {
		return false
	}
	for _, r := range value[5:] {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

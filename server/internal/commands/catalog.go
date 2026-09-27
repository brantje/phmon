package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
)

const (
	maxCoordinate = 10_000_000
	maxRadius     = 10_000
)

type noArgs struct{}

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
	case "character.walk":
		var args walkArgs
		if err := decodeExact(raw, &args); err != nil || args.Region <= 0 || !coordinate(args.X) || !coordinate(args.Y) || !coordinate(args.Z) {
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
			if args.Name != nil || args.Region == nil || *args.Region <= 0 || args.X == nil || args.Y == nil || args.Z == nil ||
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

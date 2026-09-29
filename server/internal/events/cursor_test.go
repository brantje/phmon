package events

import (
	"math"
	"testing"
	"time"
)

func TestEventCursorRoundTripAndRejectsMalformedValues(t *testing.T) {
	want := Cursor{
		OccurredAt: time.Date(2026, time.September, 28, 9, 30, 15, 123456789, time.UTC),
		EventID:    "00000000-0000-4000-8000-000000000001",
	}
	got, err := DecodeCursor(EncodeCursor(want))
	if err != nil || !got.OccurredAt.Equal(want.OccurredAt) || got.EventID != want.EventID {
		t.Fatalf("cursor round trip = %+v, %v", got, err)
	}
	for _, value := range []string{"bad", "eA", ""} {
		if _, err := DecodeCursor(value); value != "" && err == nil {
			t.Errorf("DecodeCursor(%q) accepted malformed cursor", value)
		}
	}
}

func TestValidPositionRejectsNonFiniteAndOutOfRangeCoordinates(t *testing.T) {
	if !validPosition(nil, cursorFloatPointer(0), cursorFloatPointer(-1000000), cursorFloatPointer(1000000)) {
		t.Fatal("valid position rejected")
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1000001} {
		if validPosition(nil, &value) {
			t.Errorf("invalid coordinate %v accepted", value)
		}
	}
}

func TestValidPositionAcceptsSignedCaveRegions(t *testing.T) {
	for _, region := range []int{-32768, -32767, 32767, 65535} {
		if !validPosition(&region) {
			t.Errorf("valid region %d rejected", region)
		}
	}
	for _, region := range []int{-32769, 0, 65536} {
		if validPosition(&region) {
			t.Errorf("invalid region %d accepted", region)
		}
	}
}

func cursorFloatPointer(value float64) *float64 { return &value }

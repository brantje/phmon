package httpapi

import (
	"math"
	"testing"
	"time"

	"phmon/server/internal/commands"
	"phmon/server/internal/mapprofile"
)

func trainingObservation(id, name string, region int, x, y float64, z *float64, radius float64) commands.TrainingAreaObservation {
	observed := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	return commands.TrainingAreaObservation{CharacterID: id, CharacterName: name, State: commands.ControlState{
		SessionID: "session-" + id, TrainingAvailable: true, TrainingRegion: &region,
		TrainingX: &x, TrainingY: &y, TrainingZ: z, TrainingRadius: &radius, ObservedAt: &observed,
	}}
}

func greatestProfile(t *testing.T) mapprofile.Profile {
	t.Helper()
	profile, err := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func TestProjectTrainingAreasSeparatesOutdoorAndCaveFloors(t *testing.T) {
	profile := greatestProfile(t)
	floorOneZ, floorTwoZ := 0.0, 100.0
	rows := []commands.TrainingAreaObservation{
		trainingObservation("outdoor", "Outdoor", 25735, 100, 1559, nil, 20),
		trainingObservation("cave-1f", "CaveOne", -32767, -24272, -93, &floorOneZ, 30),
		trainingObservation("cave-2f", "CaveTwo", 32767, -24272, -93, &floorTwoZ, 30),
		trainingObservation("cave-no-z", "CaveNoZ", -32767, -24272, -93, nil, 30),
	}

	world := projectTrainingAreas(profile, rows, false, "world", "world", 0)
	if world.Status != "observed" || len(world.Areas) != 1 || world.Areas[0].CharacterID != "outdoor" {
		t.Fatalf("world areas = %+v", world)
	}
	if world.Areas[0].SessionID != "session-outdoor" || world.Areas[0].Radius != 20 || world.Areas[0].ObservedAt == "" {
		t.Fatalf("world area fields = %+v", world.Areas[0])
	}

	floorOne := projectTrainingAreas(profile, rows, false, "donwhang-stone-cave", "1F", 0)
	if len(floorOne.Areas) != 1 || floorOne.Areas[0].CharacterID != "cave-1f" {
		t.Fatalf("1F areas = %+v", floorOne.Areas)
	}
	floorTwo := projectTrainingAreas(profile, rows, false, "donwhang-stone-cave", "2F", 0)
	if len(floorTwo.Areas) != 1 || floorTwo.Areas[0].CharacterID != "cave-2f" {
		t.Fatalf("2F areas = %+v", floorTwo.Areas)
	}
	signedAlias := projectTrainingAreas(profile, rows, false, "donwhang-stone-cave", "2F", -32767)
	if len(signedAlias.Areas) != 1 || signedAlias.Areas[0].Region != 32767 {
		t.Fatalf("signed region alias areas = %+v", signedAlias.Areas)
	}
}

func TestProjectTrainingAreasRejectsUnusableRowsAndFiltersRegion(t *testing.T) {
	profile := greatestProfile(t)
	unavailable := trainingObservation("unavailable", "Unavailable", 25735, 1, 2, nil, 20)
	unavailable.State.TrainingAvailable = false
	missingRadius := trainingObservation("missing", "Missing", 25735, 1, 2, nil, 20)
	missingRadius.State.TrainingRadius = nil
	rows := []commands.TrainingAreaObservation{
		unavailable,
		missingRadius,
		trainingObservation("zero-radius", "Zero", 25735, 1, 2, nil, 0),
		trainingObservation("large-radius", "Large", 25735, 1, 2, nil, 10001),
		trainingObservation("nan", "NaN", 25735, math.NaN(), 2, nil, 20),
		trainingObservation("region-zero", "RegionZero", 0, 1, 2, nil, 20),
		trainingObservation("kept", "Kept", 25735, 1, 2, nil, 20),
		trainingObservation("other-region", "Other", 23941, 1, 2, nil, 20),
	}
	got := projectTrainingAreas(profile, rows, false, "world", "world", 25735)
	if len(got.Areas) != 1 || got.Areas[0].CharacterID != "kept" {
		t.Fatalf("areas = %+v", got.Areas)
	}
}

func TestProjectTrainingAreasReportsTruncation(t *testing.T) {
	profile := greatestProfile(t)
	rows := make([]commands.TrainingAreaObservation, 0, maxMapTrainingAreas+1)
	for index := 0; index <= maxMapTrainingAreas; index++ {
		rows = append(rows, trainingObservation("c"+string(rune('a'+index%26))+time.Duration(index).String(), "N", 25735, 1, 2, nil, 20))
	}
	got := projectTrainingAreas(profile, rows, false, "world", "world", 0)
	if !got.Truncated || got.Status != "truncated" || len(got.Areas) != maxMapTrainingAreas {
		t.Fatalf("truncation = %v %s %d", got.Truncated, got.Status, len(got.Areas))
	}
	empty := projectTrainingAreas(profile, nil, true, "world", "world", 0)
	if !empty.Truncated || empty.Areas == nil {
		t.Fatalf("source truncation = %+v", empty)
	}
}

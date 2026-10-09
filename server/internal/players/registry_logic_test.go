package players

import (
	"testing"
	"time"
)

func TestMergePlayerKeepsNamesSeparateAndDoesNotRegress(t *testing.T) {
	start := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	later := start.Add(48 * time.Hour)
	hunter := "hunter"
	level7 := 7
	level101 := 101
	level102 := 102
	jobbing := false
	first, history := mergePlayer(nil, Observation{
		Server: "Greatest", ObservedName: "DarkWizard", PlayerName: strPtr("DarkWizard"),
		Level: &level101, Job: &hunter, JobLevel: &level7, IsJobbing: &jobbing,
		Source: SourceMapPlayers, ObservedAt: start,
	})
	if !history || first.Level == nil || *first.Level != 101 || first.JobName != nil {
		t.Fatalf("first sighting = %+v history %v", first, history)
	}
	second, history := mergePlayer(&first, Observation{
		Server: "Greatest", ObservedName: "DarkWizard", Level: &level102,
		Source: SourceMapPlayers, ObservedAt: later,
	})
	if !history || second.Level == nil || *second.Level != 102 || second.Job == nil || *second.Job != "hunter" || second.JobLevel == nil || *second.JobLevel != 7 {
		t.Fatalf("job information was cleared: %+v", second)
	}
	if second.PlayerName == nil || *second.PlayerName != "DarkWizard" {
		t.Fatalf("player name was cleared")
	}
	delayedLevel := 100
	delayed, history := mergePlayer(&second, Observation{
		Server: "Greatest", ObservedName: "DarkWizard", Level: &delayedLevel,
		Source: SourceMapPlayers, ObservedAt: start.Add(time.Hour),
	})
	if delayed.Level == nil || *delayed.Level != 102 {
		t.Fatalf("delayed observation regressed level to %v", delayed.Level)
	}
	if !history {
		t.Fatal("expected a historical sighting for the delayed level")
	}
	if !delayed.FirstSeenAt.Equal(start) {
		t.Fatalf("first seen = %s", delayed.FirstSeenAt)
	}
}

func TestMergeSnapshotDoesNotDuplicateOrInventLevels(t *testing.T) {
	start := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	level := 101
	hunter := "hunter"
	jobLevel := 4
	first, ok := mergeSnapshot(nil, Observation{ObservedName: "DarkWizard", Level: &level, Job: &hunter, JobLevel: &jobLevel, ObservedAt: start, Source: SourceMapPlayers})
	if !ok || first.Level != 101 || first.JobLevel == nil || *first.JobLevel != 4 {
		t.Fatalf("snapshot = %+v ok %v", first, ok)
	}
	repeat, changed := mergeSnapshot(&first, Observation{ObservedName: "DarkWizard", Level: &level, Job: &hunter, JobLevel: &jobLevel, ObservedAt: start.Add(time.Second), Source: SourceMapPlayers})
	if changed || !repeat.LastSeenAt.Equal(start) {
		t.Fatalf("unchanged repeat wrote a snapshot update: %+v changed %v", repeat, changed)
	}
	laterJob := 5
	updated, changed := mergeSnapshot(&first, Observation{ObservedName: "DarkWizard", Level: &level, Job: &hunter, JobLevel: &laterJob, ObservedAt: start.Add(time.Hour), Source: SourceMapPlayers})
	if !changed || updated.JobLevel == nil || *updated.JobLevel != 5 || !updated.FirstSeenAt.Equal(start) {
		t.Fatalf("same level did not keep one snapshot: %+v", updated)
	}
	if _, ok := mergeSnapshot(&first, Observation{ObservedName: "DarkWizard", ObservedAt: start.Add(time.Hour), Source: SourceMapPlayers}); ok {
		t.Fatal("missing level created a snapshot")
	}
}

func TestLivePlayerRejectsUnknownJob(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	jobLevel := 7
	model := int64(1907)
	jobbing := false
	if err := ValidateLiveSnapshot("observed", 25000, []Player{{
		PlayerID: "7", Name: "DarkWizard", X: 1, Y: 2, Job: "hunter", JobLevel: &jobLevel, IsJobbing: &jobbing, ModelID: &model,
	}}, now, now); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLiveSnapshot("observed", 25000, []Player{{
		PlayerID: "7", Name: "DarkWizard", X: 1, Y: 2, Job: "Warrior",
	}}, now, now); err == nil {
		t.Fatal("expected unknown job to be rejected")
	}
}

func strPtr(value string) *string { return &value }

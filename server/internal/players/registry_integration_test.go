package players

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/database"
)

func TestPlayerRegistryPersistsLevelsAndRestarts(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run player registry integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	serverName := "PlayerRegistry-" + time.Now().UTC().Format("150405.000")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM players WHERE server_key=$1`, serverKey(serverName))
		_, _ = pool.Exec(context.Background(), `DELETE FROM players WHERE server_key=$1`, serverKey(serverName+"-other"))
	})
	store := NewRegistry(pool)
	start := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	level101 := 101
	level103 := 103
	level110 := 110
	level108 := 108
	hunter := "hunter"
	job4 := 4
	job5 := 5
	if err := store.Apply(ctx, []Observation{{
		Server: serverName, ObservedName: "DarkWizard", PlayerName: strPtr("DarkWizard"),
		Level: &level101, Job: &hunter, JobLevel: &job4, IsJobbing: boolPtr(false),
		Source: SourceMapPlayers, ObservedAt: start, X: floatPtr(10), Y: floatPtr(20),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(ctx, []Observation{{
		Server: serverName, ObservedName: "DarkWizard", Level: &level101, Job: &hunter, JobLevel: &job4,
		Source: SourceMapPlayers, ObservedAt: start.Add(time.Second), X: floatPtr(11), Y: floatPtr(20),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(ctx, []Observation{{
		Server: serverName, ObservedName: "DarkWizard", Level: &level103, Job: &hunter, JobLevel: &job5,
		Source: SourceMapPlayers, ObservedAt: start.Add(48 * time.Hour), X: floatPtr(30), Y: floatPtr(40),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(ctx, []Observation{{
		Server: serverName, ObservedName: "SecretHunter", JobName: strPtr("SecretHunter"),
		Job: &hunter, IsJobbing: boolPtr(true), Source: SourceMapPlayers, ObservedAt: start,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(ctx, []Observation{{
		Server: serverName + "-other", ObservedName: "DarkWizard", Level: &level110,
		Source: SourceMapPlayers, ObservedAt: start,
	}}); err != nil {
		t.Fatal(err)
	}
	page, err := store.List(ctx, ListFilter{Server: serverName, Sort: "observed_name", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 {
		t.Fatalf("players = %d", page.Total)
	}
	var wizard PlayerRecord
	for _, player := range page.Players {
		if player.ObservedName == "DarkWizard" {
			wizard = player
		}
	}
	if wizard.ID == "" || wizard.Level == nil || *wizard.Level != 103 || wizard.JobLevel == nil || *wizard.JobLevel != 5 {
		t.Fatalf("current wizard = %+v", wizard)
	}
	if wizard.PlayerName == nil || *wizard.PlayerName != "DarkWizard" || wizard.JobName != nil {
		t.Fatalf("names = %+v %+v", wizard.PlayerName, wizard.JobName)
	}
	levels, err := store.Levels(ctx, wizard.ID, 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if levels.Total != 2 || len(levels.Chart) != 2 || levels.Chart[0].Level != 101 || levels.Chart[1].Level != 103 {
		t.Fatalf("levels = %+v", levels.Chart)
	}
	if levels.Progress.DistinctLevelsObserved != 2 || levels.Progress.HighestObservedLevel == nil || *levels.Progress.HighestObservedLevel != 103 {
		t.Fatalf("progress = %+v", levels.Progress)
	}
	if err := store.Apply(ctx, []Observation{{
		Server: serverName, ObservedName: "DarkWizard", Level: &level110,
		Source: SourceMapPlayers, ObservedAt: start.Add(96 * time.Hour),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(ctx, []Observation{{
		Server: serverName, ObservedName: "DarkWizard", Level: &level108, Job: &hunter, JobLevel: &job4,
		Source: SourceMapPlayers, ObservedAt: start.Add(72 * time.Hour),
	}}); err != nil {
		t.Fatal(err)
	}
	reloaded := NewRegistry(pool)
	current, err := reloaded.Get(ctx, wizard.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Level == nil || *current.Level != 110 {
		t.Fatalf("restarted level = %v", current.Level)
	}
	if current.Job == nil || *current.Job != "hunter" {
		t.Fatalf("delayed row cleared job: %+v", current.Job)
	}
	levels, err = reloaded.Levels(ctx, wizard.ID, 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if levels.Total != 4 {
		t.Fatalf("expected 101, 103, 108 and 110, got %d", levels.Total)
	}
	for _, snapshot := range levels.Chart {
		if snapshot.Level == 102 || snapshot.Level == 104 || snapshot.Level == 109 {
			t.Fatalf("invented level %d", snapshot.Level)
		}
	}
	var group sync.WaitGroup
	errs := make(chan error, 8)
	sharedLevel := 90
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			errs <- store.Apply(ctx, []Observation{{
				Server: serverName, ObservedName: "Concurrent", Level: &sharedLevel,
				Source: SourceMapPlayers, ObservedAt: start,
			}})
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	listed, err := store.List(ctx, ListFilter{Server: serverName, Name: "Concurrent", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if listed.Total != 1 {
		t.Fatalf("concurrent players = %d", listed.Total)
	}
	concurrentLevels, err := store.Levels(ctx, listed.Players[0].ID, 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if concurrentLevels.Total != 1 {
		t.Fatalf("concurrent snapshots = %d", concurrentLevels.Total)
	}
}

func boolPtr(value bool) *bool { return &value }

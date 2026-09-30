package navigation

import (
	"encoding/json"
	"math"
	"sync"
	"testing"
	"time"

	"phmon/server/internal/mapprofile"
)

func routeInput(sequence uint64, steps ...Instruction) Input {
	invokedAt := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	z := 0.0
	return Input{SchemaVersion: SchemaVersion, CommandID: "cmd-navigation", CharacterID: "character-one", SessionID: "session-one", Sequence: sequence,
		InvokedAt: invokedAt, Source: &Position{Region: 25000, X: 6410, Y: 1080, Z: &z, At: invokedAt.Add(-time.Second)}, Instructions: steps}
}

func TestValidBoundsAndRejectsUnrecognizedOrUnsafeInstructions(t *testing.T) {
	valid := routeInput(1, Instruction{Index: 0, Kind: "walk", X: 10, Y: -20, Z: 0},
		Instruction{Index: 1, Kind: "wait", DurationMS: 500}, Instruction{Index: 2, Kind: "teleport"})
	if err := Valid(valid); err != nil {
		t.Fatalf("valid route: %v", err)
	}
	for name, bad := range map[string]Input{
		"nan":                      routeInput(1, Instruction{Index: 0, Kind: "walk", X: math.NaN()}),
		"unknown":                  routeInput(1, Instruction{Index: 0, Kind: "script", X: 1}),
		"wait carries coordinates": routeInput(1, Instruction{Index: 0, Kind: "wait", DurationMS: 1, X: 2}),
		"sequence":                 routeInput(0, Instruction{Index: 0, Kind: "teleport"}),
	} {
		t.Run(name, func(t *testing.T) {
			if Valid(bad) == nil {
				t.Fatal("accepted invalid route")
			}
		})
	}
	tooMany := routeInput(1)
	for i := 0; i < MaxInstructions+1; i++ {
		tooMany.Instructions = append(tooMany.Instructions, Instruction{Index: i, Kind: "teleport"})
	}
	if Valid(tooMany) == nil {
		t.Fatal("accepted oversized instruction list")
	}
}

func TestStoreFencesSequenceAndReturnsOnlyRemainingTransientRoute(t *testing.T) {
	store := NewStore()
	profile, err := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	destination := Point{Region: 25000, X: 6430, Y: 1090, Z: 0}
	input := routeInput(1, Instruction{Index: 0, Kind: "walk", X: 6420, Y: 1080, Z: 0}, Instruction{Index: 1, Kind: "walk", X: 6430, Y: 1090, Z: 0})
	if !store.Replace(input, "agent-one", 1, "greatest", profile.DatasetID, destination, input.InvokedAt) {
		t.Fatal("route not stored")
	}
	if store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID, destination, input.InvokedAt) {
		t.Fatal("duplicate sequence replaced route")
	}
	views := store.Snapshot("Greatest", profile, input.InvokedAt.Add(time.Second))
	if len(views) != 1 || views[0].Status != "waiting_for_movement" || len(views[0].Blocks) != 1 || len(views[0].Blocks[0].Points) != 2 {
		t.Fatalf("unexpected initial view: %#v", views)
	}
	if !store.Observe(input.CharacterID, input.SessionID, Position{Region: 25000, X: 6420, Y: 1080, At: input.InvokedAt.Add(2 * time.Second)}) {
		t.Fatal("fresh position did not update route")
	}
	views = store.Snapshot("Greatest", profile, input.InvokedAt.Add(2*time.Second))
	if len(views) != 1 || views[0].Status != "moving" || len(views[0].Blocks[0].Points) != 1 || views[0].Blocks[0].Points[0].X != 6430 {
		t.Fatalf("passed waypoint remained: %#v", views)
	}
	if store.Observe(input.CharacterID, input.SessionID, Position{Region: 25000, X: 6425, Y: 1085, At: input.InvokedAt.Add(1500 * time.Millisecond)}) {
		t.Fatal("out-of-order position advanced route")
	}
	if !store.Observe(input.CharacterID, input.SessionID, Position{Region: 25000, X: 6430, Y: 1090, At: input.InvokedAt.Add(3 * time.Second)}) {
		t.Fatal("arrival observation was ignored")
	}
	views = store.Snapshot("Greatest", profile, input.InvokedAt.Add(3*time.Second))
	if len(views) != 1 || !views[0].Arrived || views[0].Status != "arrived" || len(views[0].Blocks) != 0 {
		t.Fatalf("route did not become terminal: %#v", views)
	}
	if store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID, destination, input.InvokedAt.Add(4*time.Second)) {
		t.Fatal("terminal sequence was resurrected")
	}
	store.RemoveSession(input.SessionID)
	if views = store.Snapshot("Greatest", profile, input.InvokedAt); len(views) != 0 {
		t.Fatalf("session route remained: %#v", views)
	}
	// Re-identification can restart the worker's sequence within the same
	// claimed session. Clearing the old route must allow its next sequence one.
	restarted := routeInput(1, Instruction{Index: 0, Kind: "walk", X: 6440, Y: 1100})
	restarted.CommandID = "cmd-navigation-restarted"
	if !store.Replace(restarted, "agent-one", 1, "Greatest", profile.DatasetID, destination, restarted.InvokedAt) {
		t.Fatal("route after session cleanup was rejected")
	}
}

func TestReplaceIfFencesOwnerAndSerializesSessionCleanup(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	destination := Point{Region: 25000, X: 6430, Y: 1090}
	input := routeInput(1, Instruction{Index: 0, Kind: "walk", X: 6420, Y: 1080})
	if store.ReplaceIf(input, "agent-one", 1, "Greatest", profile.DatasetID, destination,
		input.InvokedAt, func() bool { return false }) {
		t.Fatal("stale owner was allowed to install a route")
	}
	if views := store.Snapshot("Greatest", profile, input.InvokedAt); len(views) != 0 {
		t.Fatalf("stale owner left a route: %#v", views)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	generation := uint64(1)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if !store.ReplaceIf(input, "agent-one", generation, "Greatest", profile.DatasetID, destination,
			input.InvokedAt, func() bool {
				close(entered)
				<-release
				return true // The generation changes after its final check.
			}) {
			t.Error("current owner route was rejected")
		}
	}()
	<-entered
	if sessions := store.sessionsForOwner("agent-one", &generation); len(sessions) != 1 || sessions[0] != input.SessionID {
		t.Fatalf("in-flight route was not visible to generation cleanup: %v", sessions)
	}
	go func() {
		defer wg.Done()
		store.RemoveAgentGeneration("agent-one", generation)
	}()
	close(release)
	wg.Wait()
	if views := store.Snapshot("Greatest", profile, input.InvokedAt); len(views) != 0 {
		t.Fatalf("session cleanup raced with a stale route write: %#v", views)
	}
}

func TestWaitAndTeleportRemainSeparateDrawableBlocks(t *testing.T) {
	input := routeInput(1, Instruction{Index: 0, Kind: "walk", X: 6420, Y: 1080}, Instruction{Index: 1, Kind: "wait", DurationMS: 500}, Instruction{Index: 2, Kind: "walk", X: 6430, Y: 1090}, Instruction{Index: 3, Kind: "teleport"}, Instruction{Index: 4, Kind: "walk", X: 6440, Y: 1100})
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	if !store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID, Point{Region: 25000, X: 6440, Y: 1100}, input.InvokedAt) {
		t.Fatal("route not stored")
	}
	views := store.Snapshot("Greatest", profile, input.InvokedAt.Add(time.Second))
	if len(views) != 1 || len(views[0].Blocks) != 1 || len(views[0].Blocks[0].Points) != 1 {
		t.Fatalf("barriers were joined or future geometry leaked: %#v", views)
	}
	if views[0].Blocks[0].Points[0].X != 6420 {
		t.Fatalf("wrong active block: %#v", views)
	}
}

func TestBarrierDoesNotRevealOverlappingFutureGeometryBeforeMovementEvidence(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	input := routeInput(1,
		Instruction{Index: 0, Kind: "walk", X: 6420, Y: 1080},
		Instruction{Index: 1, Kind: "wait", DurationMS: 500},
		// Deliberately overlaps the first walk. A location match is not evidence
		// that the character has crossed the wait barrier.
		Instruction{Index: 2, Kind: "walk", X: 6420, Y: 1080},
		Instruction{Index: 3, Kind: "walk", X: 6460, Y: 1080},
	)
	input.Source = &Position{Region: 25000, X: 6400, Y: 1080, At: input.InvokedAt.Add(-time.Second)}
	if !store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID,
		Point{Region: 25000, X: 6460, Y: 1080}, input.InvokedAt) {
		t.Fatal("route not stored")
	}
	if !store.Observe(input.CharacterID, input.SessionID, Position{
		Region: 25000, X: 6420, Y: 1080, At: input.InvokedAt.Add(time.Second),
	}) {
		t.Fatal("first walk observation was ignored")
	}
	views := store.Snapshot("Greatest", profile, input.InvokedAt.Add(time.Second))
	if len(views) != 1 || views[0].Status != "transition_awaiting_evidence" || len(views[0].Blocks) != 0 {
		t.Fatalf("future geometry leaked across a wait without movement evidence: %#v", views)
	}
	if !store.Observe(input.CharacterID, input.SessionID, Position{
		Region: 25000, X: 6440, Y: 1080, At: input.InvokedAt.Add(2 * time.Second),
	}) {
		t.Fatal("post-wait movement observation was ignored")
	}
	views = store.Snapshot("Greatest", profile, input.InvokedAt.Add(2*time.Second))
	if len(views) != 1 || views[0].Status != "moving" || len(views[0].Blocks) != 1 {
		t.Fatalf("future block did not activate after movement evidence: %#v", views)
	}
}

func TestSkippedPositionSampleProjectsForwardWithinCurrentBlock(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	input := routeInput(1,
		Instruction{Index: 0, Kind: "walk", X: 6420, Y: 1080},
		Instruction{Index: 1, Kind: "walk", X: 6460, Y: 1080},
		Instruction{Index: 2, Kind: "walk", X: 6500, Y: 1080},
	)
	input.Source = &Position{Region: 25000, X: 6400, Y: 1080, At: input.InvokedAt.Add(-time.Second)}
	if !store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID, Point{Region: 25000, X: 6530, Y: 1080}, input.InvokedAt) {
		t.Fatal("route not stored")
	}
	if !store.Observe(input.CharacterID, input.SessionID, Position{Region: 25000, X: 6480, Y: 1080, At: input.InvokedAt.Add(time.Second)}) {
		t.Fatal("forward sample ignored")
	}
	views := store.Snapshot("Greatest", profile, input.InvokedAt.Add(time.Second))
	if len(views) != 1 || views[0].Status != "moving" || len(views[0].Blocks) != 1 || len(views[0].Blocks[0].Points) != 1 || views[0].Blocks[0].Points[0].X != 6500 {
		t.Fatalf("skipped points were not consumed as an ordered prefix: %#v", views)
	}
}

func TestCrossingObservationDoesNotJumpAcrossAmbiguousLoop(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	input := routeInput(1,
		Instruction{Index: 0, Kind: "walk", X: 6420, Y: 1080},
		Instruction{Index: 1, Kind: "walk", X: 6440, Y: 1080},
		Instruction{Index: 2, Kind: "walk", X: 6420, Y: 1080},
		Instruction{Index: 3, Kind: "walk", X: 6460, Y: 1080},
	)
	input.Source = &Position{Region: 25000, X: 6400, Y: 1080, At: input.InvokedAt.Add(-time.Second)}
	if !store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID, Point{Region: 25000, X: 6460, Y: 1080}, input.InvokedAt) {
		t.Fatal("route not stored")
	}
	if !store.Observe(input.CharacterID, input.SessionID, Position{Region: 25000, X: 6420, Y: 1080, At: input.InvokedAt.Add(time.Second)}) {
		t.Fatal("crossing sample ignored")
	}
	views := store.Snapshot("Greatest", profile, input.InvokedAt.Add(time.Second))
	if len(views) != 1 || len(views[0].Blocks) != 1 || views[0].Blocks[0].Points[0].X != 6440 {
		t.Fatalf("ambiguous crossing lost the earliest remaining path: %#v", views)
	}
}

func TestRouteTypesStayJSONBoundedAndScriptFree(t *testing.T) {
	input := routeInput(1, Instruction{Index: 0, Kind: "teleport"})
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > MaxFrameBytes {
		t.Fatalf("small route exceeds frame budget: %d", len(encoded))
	}
}

func TestNewerSuccessfulRouteReplacesAndInvalidRoutePreservesPriorRoute(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	destination := Point{Region: 25000, X: 6430, Y: 1090}
	first := routeInput(1, Instruction{Index: 0, Kind: "walk", X: 6420, Y: 1080})
	if !store.Replace(first, "agent-one", 1, "Greatest", profile.DatasetID, destination, first.InvokedAt) {
		t.Fatal("first route not stored")
	}
	second := routeInput(2, Instruction{Index: 0, Kind: "walk", X: 6425, Y: 1085})
	second.CommandID = "cmd-navigation-newer"
	if !store.Replace(second, "agent-one", 1, "Greatest", profile.DatasetID, destination, second.InvokedAt.Add(time.Second)) {
		t.Fatal("newer successful invocation did not replace old route")
	}
	invalid := routeInput(3, Instruction{Index: 0, Kind: "exec", X: 1})
	invalid.CommandID = "cmd-navigation-invalid"
	if store.Replace(invalid, "agent-one", 1, "Greatest", profile.DatasetID, destination, invalid.InvokedAt.Add(2*time.Second)) {
		t.Fatal("invalid newer invocation replaced a valid running route")
	}
	views := store.Snapshot("Greatest", profile, second.InvokedAt.Add(time.Second))
	if len(views) != 1 || views[0].CommandID != second.CommandID || views[0].Sequence != 2 ||
		len(views[0].Blocks) != 1 || views[0].Blocks[0].Points[0].X != 6425 {
		t.Fatalf("valid route did not survive the failed replacement: %#v", views)
	}
}

func TestRouteSnapshotBudgetOmitsWholeGeometryButKeepsStatus(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	input := routeInput(1,
		Instruction{Index: 0, Kind: "walk", X: 6420, Y: 1080},
		Instruction{Index: 1, Kind: "walk", X: 6430, Y: 1090},
	)
	if !store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID,
		Point{Region: 25000, X: 6440, Y: 1100}, input.InvokedAt) {
		t.Fatal("route not stored")
	}
	full := store.Snapshot("Greatest", profile, input.InvokedAt)[0]
	fullBytes, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	statusOnly := full
	statusOnly.Blocks = []Block{}
	statusOnly.CurrentAnchor = nil
	statusOnly.GeometryOmitted = true
	statusOnly.Reason = "navigation_geometry_payload_budget"
	statusBytes, err := json.Marshal(statusOnly)
	if err != nil {
		t.Fatal(err)
	}
	if len(statusBytes) >= len(fullBytes) {
		t.Fatal("fixture did not create a meaningful geometry budget difference")
	}
	views, omitted := store.SnapshotBudget("Greatest", profile, input.InvokedAt, len(statusBytes))
	if omitted != 0 || len(views) != 1 || !views[0].GeometryOmitted || len(views[0].Blocks) != 0 ||
		views[0].Reason != "navigation_geometry_payload_budget" {
		t.Fatalf("payload reduction split geometry or lost status: omitted=%d views=%#v", omitted, views)
	}
}

func TestMissingSourceStartsStatusOnlyAndFreshPositionCanResolveOutdoorScope(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	input := routeInput(1,
		Instruction{Index: 0, Kind: "walk", X: 6400, Y: 1080},
		Instruction{Index: 1, Kind: "walk", X: 6460, Y: 1080},
	)
	input.Source = nil
	if !store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID,
		Point{Region: 25000, X: 6500, Y: 1090}, input.InvokedAt) {
		t.Fatal("route not stored")
	}
	views := store.Snapshot("Greatest", profile, input.InvokedAt)
	if len(views) != 1 || len(views[0].Blocks) != 0 || views[0].Reason != "route_geometry_unavailable" {
		t.Fatalf("missing source evidence did not stay status-only: %#v", views)
	}
	if !store.Observe(input.CharacterID, input.SessionID, Position{
		Region: 25000, X: 6430, Y: 1080, At: input.InvokedAt.Add(time.Second),
	}) {
		t.Fatal("fresh position did not resolve route context")
	}
	views = store.Snapshot("Greatest", profile, input.InvokedAt.Add(time.Second))
	if len(views) != 1 || views[0].Status != "moving" || len(views[0].Blocks) != 1 ||
		views[0].Blocks[0].AreaID != "world" || views[0].Blocks[0].FloorID != "world" ||
		len(views[0].Blocks[0].Points) != 1 || views[0].Blocks[0].Points[0].X != 6460 {
		t.Fatalf("fresh position did not resolve only the remaining outdoor geometry: %#v", views)
	}
}

func TestOutdoorScopeUsesTileGridSeamsAndUnsupportedProfilesFailClosed(t *testing.T) {
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	z := 0.0
	current := route{server: "Greatest", datasetID: profile.DatasetID,
		Source: &Position{Region: 25000, X: 6410, Y: 1080, Z: &z, At: time.Now().UTC()}}
	left, _, _, leftOK := scopeStep(current, Instruction{Kind: "walk", X: 6527.99, Y: 1080, Z: 0})
	right, _, _, rightOK := scopeStep(current, Instruction{Kind: "walk", X: 6528, Y: 1080, Z: 0})
	if !leftOK || !rightOK || left.Region != 25000 || right.Region != 25001 {
		t.Fatalf("outdoor seam scope left=%#v/%v right=%#v/%v", left, leftOK, right, rightOK)
	}
	current.datasetID = "unknown-dataset"
	if _, _, _, ok := scopeStep(current, Instruction{Kind: "walk", X: 6528, Y: 1080, Z: 0}); ok {
		t.Fatal("unknown dataset produced route geometry")
	}
}

func TestCaveScopePreservesSignedRegionsAndRefusesUnverifiedJobTempleFloors(t *testing.T) {
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	zero := 0.0
	cave := route{server: "Greatest", datasetID: profile.DatasetID,
		Source: &Position{Region: -32762, X: -100, Y: 50, Z: &zero, At: time.Now().UTC()}}
	point, area, floor, ok := scopeStep(cave, Instruction{Kind: "walk", X: -100, Y: 50, Z: 0})
	if !ok || point.Region != -32762 || area != "tomb-of-qin-shi" || floor != "B2" {
		t.Fatalf("signed Tomb scope = %#v %q/%q %v", point, area, floor, ok)
	}
	donwhangZ := 220.0
	donwhang := route{server: "Greatest", datasetID: profile.DatasetID,
		Source: &Position{Region: -32767, X: -24000, Y: -100, Z: &donwhangZ, At: time.Now().UTC()}}
	point, area, floor, ok = scopeStep(donwhang, Instruction{Kind: "walk", X: -23900, Y: -100, Z: 220})
	if !ok || point.Region != -32767 || area != "donwhang-stone-cave" || floor != "3F" {
		t.Fatalf("signed Donwhang 3F scope = %#v %q/%q %v", point, area, floor, ok)
	}
	// The shared Job Temple region does not establish one of its manually
	// selected upper or annex floors without an observed Z/floor classification.
	temple := route{server: "Greatest", datasetID: profile.DatasetID,
		Source:      &Position{Region: -32752, X: 500, Y: 500, At: time.Now().UTC()},
		destination: Point{Region: -32752, X: 500, Y: 500, Z: 16}}
	if _, area, floor, ok := scopeStep(temple, Instruction{Kind: "walk", X: 500, Y: 500, Z: 16}); ok {
		t.Fatalf("unverified Job Temple floor was projected as %s/%s", area, floor)
	}
}

func TestOutdoorSeamKeepsConnectedGeometryAndAdvancesAcrossSkippedSamples(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	input := routeInput(1,
		Instruction{Index: 0, Kind: "walk", X: 6450, Y: 1080},
		Instruction{Index: 1, Kind: "walk", X: 6500, Y: 1080},
		Instruction{Index: 2, Kind: "walk", X: 6580, Y: 1080},
		Instruction{Index: 3, Kind: "walk", X: 6650, Y: 1080})
	store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID, Point{Region: 25001, X: 6650, Y: 1080}, input.InvokedAt)
	view := store.Snapshot("Greatest", profile, input.InvokedAt)[0]
	if len(view.Blocks) != 1 || len(view.Blocks[0].Points) != 4 || view.Blocks[0].Points[2].Region != 25001 {
		t.Fatalf("outdoor seam split valid walk geometry: %#v", view)
	}
	store.Observe(input.CharacterID, input.SessionID, Position{Region: 25001, X: 6540, Y: 1080, At: input.InvokedAt.Add(time.Second)})
	view = store.Snapshot("Greatest", profile, input.InvokedAt.Add(time.Second))[0]
	if view.Status != "moving" || len(view.Blocks) != 1 || len(view.Blocks[0].Points) != 2 || view.Blocks[0].Points[0].X != 6580 || view.CurrentAnchor == nil || view.CurrentAnchor.Region != 25001 {
		t.Fatalf("cross-seam progress/connector failed: %#v", view)
	}
	store.Observe(input.CharacterID, input.SessionID, Position{Region: 25000, X: 6500, Y: 1080, At: input.InvokedAt.Add(500 * time.Millisecond)})
	view = store.Snapshot("Greatest", profile, input.InvokedAt.Add(time.Second))[0]
	if len(view.Blocks[0].Points) != 2 {
		t.Fatal("older sample rewound progress")
	}
	store.Observe(input.CharacterID, input.SessionID, Position{Region: 25001, X: 6650, Y: 1080, At: input.InvokedAt.Add(2 * time.Second)})
	if !store.Snapshot("Greatest", profile, input.InvokedAt.Add(2*time.Second))[0].Arrived {
		t.Fatal("seam route never arrived")
	}
}

func TestOutdoorSeamRejectsInconsistentObservedRegionAndPreservesWait(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	input := routeInput(1, Instruction{Index: 0, Kind: "walk", X: 6500, Y: 1080},
		Instruction{Index: 1, Kind: "wait", DurationMS: 500}, Instruction{Index: 2, Kind: "walk", X: 6580, Y: 1080})
	store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID, Point{Region: 25001, X: 6700, Y: 1080}, input.InvokedAt)
	view := store.Snapshot("Greatest", profile, input.InvokedAt)[0]
	if len(view.Blocks) != 1 || len(view.Blocks[0].Points) != 1 {
		t.Fatal("geometry crossed the wait")
	}
	store.Observe(input.CharacterID, input.SessionID, Position{Region: 25000, X: 6540, Y: 1080, At: input.InvokedAt.Add(time.Second)})
	view = store.Snapshot("Greatest", profile, input.InvokedAt.Add(time.Second))[0]
	if view.Status != "progress_uncertain" || view.CurrentAnchor != nil || view.Blocks[0].Points[0].X != 6500 {
		t.Fatalf("inconsistent outdoor region advanced progress: %#v", view)
	}
}

func TestDuplicateRouteSkipsOwnerLookupAndDoesNotRefreshTerminalEvidence(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	input := routeInput(1, Instruction{Index: 0, Kind: "walk", X: 6430, Y: 1090})
	destination := Point{Region: 25000, X: 6430, Y: 1090}
	if !store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID, destination, input.InvokedAt) {
		t.Fatal("route not stored")
	}
	store.Observe(input.CharacterID, input.SessionID, Position{Region: 25000, X: 6430, Y: 1090, At: input.InvokedAt.Add(time.Second)})
	lookups := 0
	if store.ReplaceIf(input, "agent-one", 1, "Greatest", profile.DatasetID, destination, input.InvokedAt.Add(time.Hour), func() bool {
		lookups++
		return true
	}) || lookups != 0 {
		t.Fatalf("duplicate called owner lookup or replaced terminal route: %d", lookups)
	}
	if !store.AlreadyApplied(input, "agent-one", 1) || store.AlreadyApplied(input, "another-agent", 1) || store.AlreadyApplied(input, "agent-one", 2) {
		t.Fatal("duplicate precheck did not fence the authenticated owner")
	}
	view := store.Snapshot("Greatest", profile, input.InvokedAt.Add(time.Hour))[0]
	if !view.Arrived || !view.UpdatedAt.Equal(input.InvokedAt.Add(time.Second)) {
		t.Fatalf("duplicate changed terminal evidence: %#v", view)
	}
	newer := input
	newer.Sequence++
	if store.AlreadyApplied(newer, "agent-one", 1) {
		t.Fatal("new sequence was skipped")
	}
}

func TestSnapshotBudgetChoosesGeometryInStableSessionOrder(t *testing.T) {
	store := NewStore()
	profile, _ := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	input := routeInput(1)
	for i := 0; i < MaxInstructions; i++ {
		input.Instructions = append(input.Instructions, Instruction{Index: i, Kind: "walk", X: 6420 + float64(i)/100, Y: 1080})
	}
	for _, session := range []string{"session-z", "session-a"} {
		input.SessionID = session
		input.CharacterID = session
		if !store.Replace(input, "agent-one", 1, "Greatest", profile.DatasetID, Point{Region: 25000, X: 6430, Y: 1090}, input.InvokedAt) {
			t.Fatal("route not stored")
		}
	}
	views := store.Snapshot("Greatest", profile, input.InvokedAt)
	first, _ := json.Marshal(views[0])
	status := views[1]
	status.Blocks = []Block{}
	status.CurrentAnchor = nil
	status.GeometryOmitted = true
	status.Reason = "navigation_geometry_payload_budget"
	second, _ := json.Marshal(status)
	for i := 0; i < 50; i++ {
		views, omitted := store.SnapshotBudget("Greatest", profile, input.InvokedAt, len(first)+len(second))
		if omitted != 0 || len(views) != 2 || views[0].SessionID != "session-a" || views[0].GeometryOmitted || views[1].SessionID != "session-z" || !views[1].GeometryOmitted {
			t.Fatalf("geometry allocation changed: omitted=%d views=%#v", omitted, views)
		}
	}
}

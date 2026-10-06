package httpapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"phmon/server/internal/mapprofile"
	"phmon/server/internal/navigation"
	"phmon/server/internal/positions"
)

func TestPositionsLiveSubscriptionRequiresOnlyServerScope(t *testing.T) {
	base := liveClientMessage{
		ProtocolVersion: liveProtocolVersion,
		SubscriptionID:  "map-positions",
		Revision:        1,
		Stream:          "positions",
		Filter:          liveFilter{Server: "Greatest"},
	}
	if _, ok := validateLiveSubscription(base); !ok {
		t.Fatal("valid positions subscription rejected")
	}
	withRegion := base
	withRegion.Filter.Region = 25000
	if _, ok := validateLiveSubscription(withRegion); ok {
		t.Fatal("positions subscription unexpectedly accepted map filter")
	}
	withoutServer := base
	withoutServer.Filter.Server = ""
	if _, ok := validateLiveSubscription(withoutServer); ok {
		t.Fatal("positions subscription unexpectedly accepted empty server")
	}
}

func TestPositionSnapshotsAreOnlyIncludedWhenExplicitlyRequested(t *testing.T) {
	client := &liveClient{
		subscriptions: map[string]liveSubscription{
			"map": {
				ID: "map", Revision: 1, Stream: "map",
				Filter: liveFilter{Server: "Greatest", Area: "world", Floor: "world"},
			},
			"map-positions": {
				ID: "map-positions", Revision: 3, Stream: "positions",
				Filter: liveFilter{Server: "Greatest"},
			},
		},
		positionSnapshots: map[string]uint64{"map-positions": 3},
	}

	first := client.snapshotSubscriptionsForPass()
	if len(first) != 2 {
		t.Fatalf("initial pass should include map and requested positions: %#v", first)
	}
	second := client.snapshotSubscriptionsForPass()
	if len(second) != 1 || second[0].Stream != "map" {
		t.Fatalf("global invalidation pass should skip positions: %#v", second)
	}

	client.mu.Lock()
	client.positionSnapshots["map-positions"] = 3
	client.mu.Unlock()
	refreshed := client.snapshotSubscriptionsForPass()
	if len(refreshed) != 2 {
		t.Fatalf("explicit position refresh should request a new snapshot: %#v", refreshed)
	}
}

func TestRealtimePositionHotPathFeedsNavigationWithoutSnapshotInvalidation(t *testing.T) {
	now := time.Date(2026, 10, 5, 20, 0, 0, 0, time.UTC)
	characterID := testPositionUUID(1)
	sessionID := testPositionUUID(101)
	agentID := testPositionUUID(201)

	positionStore := positions.NewStore()
	positionStore.Claim("Greatest", agentID, 4, characterID, sessionID)
	navigationStore := navigation.NewStore()
	destination := navigation.Point{Region: 25000, X: 10, Y: 20, Z: 3}
	if !navigationStore.Replace(navigation.Input{
		SchemaVersion: navigation.SchemaVersion,
		CommandID:     testPositionUUID(301),
		CharacterID:   characterID,
		SessionID:     sessionID,
		Sequence:      1,
		InvokedAt:     now.Add(-time.Second),
		Instructions: []navigation.Instruction{{
			Index: 0, Kind: "walk", X: 10, Y: 20, Z: 3,
		}},
	}, agentID, 4, "Greatest", "dataset", destination, now.Add(-time.Second)) {
		t.Fatal("failed to prepare navigation route")
	}

	hub := NewLiveHub(nil, nil, nil)
	hub.SetPositions(positionStore)
	client := &liveClient{
		ctx:           context.Background(),
		outgoing:      make(chan []byte, liveOutgoingQueueSize),
		snapshotWake:  make(chan struct{}, 1),
		subscriptions: map[string]liveSubscription{},
		revisions:     map[string]uint64{},
	}
	hub.register(client)
	defer hub.unregister(client)

	handler := &agentHandler{
		positions:  positionStore,
		live:       hub,
		navigation: navigationStore,
	}
	accepted, ok := handler.applyRealtimePosition(positions.Position{
		AgentID: agentID, Generation: 4, CharacterID: characterID,
		SessionID: sessionID, Sequence: 1, Region: 25000,
		X: 10, Y: 20, Z: positionFloat64(3), ObservedAt: now,
	}, now)
	if !ok || accepted.Server != "Greatest" {
		t.Fatalf("valid realtime position was not accepted: %#v", accepted)
	}
	if len(client.snapshotWake) != 0 {
		t.Fatal("realtime hot path woke full live snapshots")
	}

	views := navigationStore.Snapshot("Greatest", mapprofile.Profile{
		DatasetID: "dataset", DatasetVersion: "test",
	}, now)
	if len(views) != 1 || !views[0].UpdatedAt.Equal(now) {
		t.Fatalf("navigation did not receive realtime observation: %#v", views)
	}
}

func TestPositionSnapshotReadsAfterDeliveryFence(t *testing.T) {
	now := time.Date(2026, 10, 5, 20, 0, 0, 0, time.UTC)
	characterID := testPositionUUID(1)
	agentID := testPositionUUID(201)
	oldSession := testPositionUUID(101)
	newSession := testPositionUUID(102)

	positionStore := positions.NewStore()
	positionStore.Claim("Greatest", agentID, 1, characterID, oldSession)
	if _, ok := positionStore.Apply(positions.Position{
		AgentID: agentID, Generation: 1, CharacterID: characterID,
		SessionID: oldSession, Sequence: 1, Region: 25000,
		X: 1, Y: 2, Z: positionFloat64(3), ObservedAt: now,
	}, now); !ok {
		t.Fatal("failed to prepare old position")
	}

	hub := NewLiveHub(nil, nil, nil)
	hub.SetPositions(positionStore)
	subscription := liveSubscription{
		ID: "map-positions", Revision: 1, Stream: "positions",
		Filter: liveFilter{Server: "Greatest"},
	}
	client := &liveClient{
		hub:           hub,
		ctx:           context.Background(),
		outgoing:      make(chan []byte, liveOutgoingQueueSize),
		subscriptions: map[string]liveSubscription{subscription.ID: subscription},
		revisions:     map[string]uint64{subscription.ID: subscription.Revision},
	}

	hub.positionDeliveryMu.Lock()
	done := make(chan bool, 1)
	go func() {
		done <- client.snapshot(subscription)
	}()

	positionStore.Claim("Greatest", agentID, 2, characterID, newSession)
	if _, ok := positionStore.Apply(positions.Position{
		AgentID: agentID, Generation: 2, CharacterID: characterID,
		SessionID: newSession, Sequence: 1, Region: 25000,
		X: 10, Y: 20, Z: positionFloat64(30), ObservedAt: now.Add(time.Second),
	}, now.Add(time.Second)); !ok {
		hub.positionDeliveryMu.Unlock()
		t.Fatal("failed to prepare replacement position")
	}
	hub.positionDeliveryMu.Unlock()

	select {
	case ok := <-done:
		if !ok {
			t.Fatal("position snapshot was not enqueued")
		}
	case <-time.After(time.Second):
		t.Fatal("position snapshot remained blocked")
	}

	var frame struct {
		Data struct {
			Positions []positions.Position `json:"positions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(<-client.outgoing, &frame); err != nil {
		t.Fatal(err)
	}
	if len(frame.Data.Positions) != 1 || frame.Data.Positions[0].SessionID != newSession {
		t.Fatalf("snapshot escaped delivery fence with stale session: %#v", frame.Data.Positions)
	}
}

func TestPositionBatchKeepsLatestPerCharacterAndDoesNotInvalidateSnapshots(t *testing.T) {
	hub := NewLiveHub(nil, nil, nil)
	client := &liveClient{
		hub:           hub,
		ctx:           context.Background(),
		outgoing:      make(chan []byte, liveOutgoingQueueSize),
		snapshotWake:  make(chan struct{}, 1),
		subscriptions: map[string]liveSubscription{},
		revisions:     map[string]uint64{},
	}
	client.subscriptions["map-positions"] = liveSubscription{
		ID: "map-positions", Revision: 1, Stream: "positions",
		Filter: liveFilter{Server: "Greatest"},
	}
	hub.register(client)
	defer hub.unregister(client)

	now := time.Date(2026, 10, 5, 20, 0, 0, 0, time.UTC)
	for sequence := uint64(1); sequence <= 5; sequence++ {
		for index := 0; index < 50; index++ {
			characterID := testPositionUUID(index)
			sessionID := testPositionUUID(index + 100)
			hub.PublishPosition(positions.Position{
				Server: "Greatest", AgentID: testPositionUUID(index + 200), Generation: 1,
				CharacterID: characterID, SessionID: sessionID, Sequence: sequence,
				Region: 25000, X: float64(sequence*100) + float64(index), Y: float64(index),
				ObservedAt: now.Add(time.Duration(sequence) * 200 * time.Millisecond),
			})
		}
	}

	hub.positionMu.Lock()
	if hub.positionFlush != nil {
		hub.positionFlush.Stop()
		hub.positionFlush = nil
	}
	pending := len(hub.positionPending)
	hub.positionMu.Unlock()
	if pending != 50 {
		t.Fatalf("pending latest map grew with update count: got %d want 50", pending)
	}
	if len(client.snapshotWake) != 0 {
		t.Fatal("realtime positions triggered whole-subscription invalidation")
	}

	hub.flushPositionDeltas()
	if got := len(client.outgoing); got != 1 {
		t.Fatalf("expected one batched browser frame, got %d", got)
	}
	var frame map[string]any
	if err := json.Unmarshal(<-client.outgoing, &frame); err != nil {
		t.Fatal(err)
	}
	if frame["type"] != "delta" || frame["stream"] != "positions" || frame["subscription_id"] != "map-positions" {
		t.Fatalf("unexpected delta envelope: %#v", frame)
	}
	data, ok := frame["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing delta data: %#v", frame)
	}
	rows, ok := data["positions"].([]any)
	if !ok || len(rows) != 50 {
		t.Fatalf("unexpected batched position count: %#v", data["positions"])
	}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok || row["sequence"] != float64(5) {
			t.Fatalf("intermediate position leaked into batch: %#v", raw)
		}
	}
	if livePositionCoalesce != 100*time.Millisecond {
		t.Fatalf("unexpected coalesce interval: %s", livePositionCoalesce)
	}
}

func TestPositionRemovalCarriesSessionAndCannotDeleteNewerPendingPosition(t *testing.T) {
	hub := NewLiveHub(nil, nil, nil)
	characterID := testPositionUUID(1)
	oldSession := testPositionUUID(101)
	newSession := testPositionUUID(102)
	hub.PublishPositionRemoval(positions.Removal{
		Server: "Greatest", CharacterID: characterID, SessionID: oldSession,
	})
	hub.PublishPosition(positions.Position{
		Server: "Greatest", CharacterID: characterID, SessionID: newSession,
		Sequence: 1, Region: 25000, X: 1, Y: 2, ObservedAt: time.Now().UTC(),
	})

	hub.positionMu.Lock()
	defer hub.positionMu.Unlock()
	if current, ok := hub.positionPending[characterID]; !ok || current.SessionID != newSession {
		t.Fatalf("newer session position was removed: %#v", current)
	}
	removal, ok := hub.positionRemoved[characterID]
	if !ok || removal.SessionID != oldSession {
		t.Fatalf("old-session removal was not retained safely: %#v", removal)
	}
	if hub.positionFlush != nil {
		hub.positionFlush.Stop()
		hub.positionFlush = nil
	}
}

func positionFloat64(value float64) *float64 {
	return &value
}

func TestPositionBatchUsesClientVisibleSessionAcrossSnapshotBetweenReplacements(t *testing.T) {
	now := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	characterID := testPositionUUID(1)
	agentID := testPositionUUID(201)
	sessionA := testPositionUUID(101)
	sessionB := testPositionUUID(102)
	sessionC := testPositionUUID(103)

	store := positions.NewStore()
	store.Claim("Greatest", agentID, 1, characterID, sessionA)
	if _, ok := store.Apply(positions.Position{
		AgentID: agentID, Generation: 1, CharacterID: characterID, SessionID: sessionA,
		Sequence: 1, Region: 25000, X: 1, Y: 2, ObservedAt: now,
	}, now); !ok {
		t.Fatal("failed to prepare session A")
	}

	hub := NewLiveHub(nil, nil, nil)
	hub.SetPositions(store)
	subscription := liveSubscription{
		ID: "map-positions", Revision: 1, Stream: "positions",
		Filter: liveFilter{Server: "Greatest"},
	}
	client := &liveClient{
		hub: hub, ctx: context.Background(),
		outgoing: make(chan []byte, liveOutgoingQueueSize),
		subscriptions: map[string]liveSubscription{subscription.ID: subscription},
		revisions: map[string]uint64{subscription.ID: subscription.Revision},
		positionSessions: make(map[string]map[string]string),
	}
	hub.register(client)
	defer hub.unregister(client)

	// Establish browser-visible A through the real snapshot delivery path.
	if !client.snapshot(subscription) {
		t.Fatal("session A snapshot was not delivered")
	}
	<-client.outgoing
	if got := client.positionSessionState(subscription.ID)[characterID]; got != sessionA {
		t.Fatalf("server did not remember browser-visible A: %q", got)
	}

	// A -> B queues A's tombstone and B's latest position.
	for _, removal := range store.Claim("Greatest", agentID, 2, characterID, sessionB) {
		hub.PublishPositionRemoval(removal)
	}
	positionB, ok := store.Apply(positions.Position{
		AgentID: agentID, Generation: 2, CharacterID: characterID, SessionID: sessionB,
		Sequence: 1, Region: 25000, X: 10, Y: 20, ObservedAt: now.Add(time.Second),
	}, now.Add(time.Second))
	if !ok {
		t.Fatal("failed to prepare session B")
	}
	hub.PublishPosition(positionB)

	// Explicit refresh lands before the pending 100 ms delta and exposes B.
	if !client.snapshot(subscription) {
		t.Fatal("session B snapshot was not delivered")
	}
	<-client.outgoing
	if got := client.positionSessionState(subscription.ID)[characterID]; got != sessionB {
		t.Fatalf("snapshot did not expose B: %q", got)
	}

	// B -> C occurs in the same coalescing window. The bounded tombstone map
	// still contains A, but delivery must remove the actually visible B.
	for _, removal := range store.Claim("Greatest", agentID, 3, characterID, sessionC) {
		hub.PublishPositionRemoval(removal)
	}
	positionC, ok := store.Apply(positions.Position{
		AgentID: agentID, Generation: 3, CharacterID: characterID, SessionID: sessionC,
		Sequence: 1, Region: 25000, X: 100, Y: 200, ObservedAt: now.Add(2 * time.Second),
	}, now.Add(2*time.Second))
	if !ok {
		t.Fatal("failed to prepare session C")
	}
	hub.PublishPosition(positionC)

	hub.positionMu.Lock()
	if hub.positionFlush != nil {
		hub.positionFlush.Stop()
		hub.positionFlush = nil
	}
	if len(hub.positionPending) > 1 || len(hub.positionRemoved) > 1 {
		hub.positionMu.Unlock()
		t.Fatal("rapid replacement state grew beyond one entry per character")
	}
	hub.positionMu.Unlock()
	hub.flushPositionDeltas()

	var delta struct {
		Data struct {
			Positions []positions.Position `json:"positions"`
			Removed   []positions.Removal  `json:"removed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(<-client.outgoing, &delta); err != nil {
		t.Fatal(err)
	}
	if len(delta.Data.Removed) != 1 || delta.Data.Removed[0].SessionID != sessionB {
		t.Fatalf("delta did not remove browser-visible B: %#v", delta.Data.Removed)
	}
	if len(delta.Data.Positions) != 1 || delta.Data.Positions[0].SessionID != sessionC {
		t.Fatalf("delta did not advance browser to C: %#v", delta.Data.Positions)
	}
	if got := client.positionSessionState(subscription.ID)[characterID]; got != sessionC {
		t.Fatalf("delivered state did not advance to C: %q", got)
	}

	// A's delayed tombstone must never delete the newer C browser state.
	hub.PublishPositionRemoval(positions.Removal{
		Server: "Greatest", CharacterID: characterID, SessionID: sessionA,
	})
	hub.positionMu.Lock()
	if hub.positionFlush != nil {
		hub.positionFlush.Stop()
		hub.positionFlush = nil
	}
	hub.positionMu.Unlock()
	hub.flushPositionDeltas()
	if got := len(client.outgoing); got != 0 {
		t.Fatalf("stale A removal produced a browser delta after C: %d", got)
	}
	if got := client.positionSessionState(subscription.ID)[characterID]; got != sessionC {
		t.Fatalf("stale A removal changed delivered C state: %q", got)
	}
}


type fakePositionTimer struct {
	due      time.Duration
	function func()
	active   bool
}

func (timer *fakePositionTimer) Stop() bool {
	wasActive := timer.active
	timer.active = false
	return wasActive
}

type fakePositionScheduler struct {
	now    time.Duration
	timers []*fakePositionTimer
}

func (scheduler *fakePositionScheduler) afterFunc(delay time.Duration, function func()) livePositionTimer {
	timer := &fakePositionTimer{
		due: scheduler.now + delay, function: function, active: true,
	}
	scheduler.timers = append(scheduler.timers, timer)
	return timer
}

func (scheduler *fakePositionScheduler) advance(delta time.Duration) {
	scheduler.now += delta
	for {
		var next *fakePositionTimer
		for _, timer := range scheduler.timers {
			if !timer.active || timer.due > scheduler.now {
				continue
			}
			if next == nil || timer.due < next.due {
				next = timer
			}
		}
		if next == nil {
			return
		}
		next.active = false
		next.function()
	}
}

func TestPositionBatchSchedulerSustainsFiftyCharactersAtFiveHz(t *testing.T) {
	hub := NewLiveHub(nil, nil, nil)
	scheduler := &fakePositionScheduler{}
	hub.positionAfterFunc = scheduler.afterFunc
	client := &liveClient{
		hub: hub, ctx: context.Background(),
		outgoing: make(chan []byte, liveOutgoingQueueSize),
		snapshotWake: make(chan struct{}, 1),
		subscriptions: map[string]liveSubscription{
			"map-positions": {
				ID: "map-positions", Revision: 1, Stream: "positions",
				Filter: liveFilter{Server: "Greatest"},
			},
		},
		revisions: map[string]uint64{"map-positions": 1},
		positionSessions: make(map[string]map[string]string),
	}
	hub.register(client)
	defer hub.unregister(client)

	now := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	batches := 0
	expectedSequence := uint64(0)
	for tick := 0; tick < 20; tick++ {
		if tick%2 == 0 {
			expectedSequence++
			for index := 0; index < 50; index++ {
				hub.PublishPosition(positions.Position{
					Server: "Greatest", AgentID: testPositionUUID(index + 200), Generation: 1,
					CharacterID: testPositionUUID(index), SessionID: testPositionUUID(index + 100),
					Sequence: expectedSequence, Region: 25000,
					X: float64(expectedSequence*100) + float64(index), Y: float64(index),
					ObservedAt: now.Add(time.Duration(tick) * 100 * time.Millisecond),
				})
			}
			hub.positionMu.Lock()
			pending := len(hub.positionPending)
			hub.positionMu.Unlock()
			if pending != 50 {
				t.Fatalf("pending map grew beyond one position per character: %d", pending)
			}
		}
		if len(client.snapshotWake) != 0 {
			t.Fatal("realtime batching triggered global snapshot invalidation")
		}

		scheduler.advance(100 * time.Millisecond)
		for len(client.outgoing) > 0 {
			batches++
			var frame struct {
				Data struct {
					Positions []positions.Position `json:"positions"`
				} `json:"data"`
			}
			if err := json.Unmarshal(<-client.outgoing, &frame); err != nil {
				t.Fatal(err)
			}
			if len(frame.Data.Positions) != 50 {
				t.Fatalf("batch %d carried %d positions, want 50", batches, len(frame.Data.Positions))
			}
			for _, position := range frame.Data.Positions {
				if position.Sequence != uint64(batches) {
					t.Fatalf("batch %d leaked non-latest sequence %d", batches, position.Sequence)
				}
			}
		}
	}

	if batches != 10 {
		t.Fatalf("two seconds at 5 Hz produced %d batches, want 10", batches)
	}
	if batches > 20 {
		t.Fatalf("batch rate exceeded 10/sec: %d batches in two seconds", batches)
	}
	if state := client.positionSessionState("map-positions"); len(state) != 50 {
		t.Fatalf("delivered session state is not bounded per character: %d", len(state))
	}
	if len(client.snapshotWake) != 0 {
		t.Fatal("realtime batching triggered global snapshot invalidation")
	}
}

func testPositionUUID(value int) string {
	const hex = "0123456789abcdef"
	last := make([]byte, 12)
	for i := range last {
		last[i] = '0'
	}
	number := value
	for i := len(last) - 1; i >= 0 && number > 0; i-- {
		last[i] = hex[number&15]
		number >>= 4
	}
	return "11111111-2222-4333-8444-" + string(last)
}

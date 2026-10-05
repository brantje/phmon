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
	if len(views) != 1 || !views[0].Arrived || !views[0].UpdatedAt.Equal(now) {
		t.Fatalf("navigation did not receive realtime observation: %#v", views)
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

func TestPositionBatchRetainsFirstRemovedSessionAcrossRapidReplacement(t *testing.T) {
	hub := NewLiveHub(nil, nil, nil)
	characterID := testPositionUUID(1)
	first := positions.Removal{
		Server: "Greatest", CharacterID: characterID, SessionID: testPositionUUID(101),
	}
	hub.PublishPositionRemoval(first)
	hub.PublishPositionRemoval(positions.Removal{
		Server: "Greatest", CharacterID: characterID, SessionID: testPositionUUID(102),
	})
	hub.PublishPositionRemoval(positions.Removal{
		Server: "Greatest", CharacterID: characterID, SessionID: testPositionUUID(103),
	})

	hub.positionMu.Lock()
	defer hub.positionMu.Unlock()
	removal, ok := hub.positionRemoved[characterID]
	if !ok || removal.SessionID != first.SessionID {
		t.Fatalf("batch lost the browser-visible session tombstone: %#v", removal)
	}
	if hub.positionFlush != nil {
		hub.positionFlush.Stop()
		hub.positionFlush = nil
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

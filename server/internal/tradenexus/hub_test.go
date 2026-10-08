package tradenexus

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"phmon/server/internal/events"
)

type memorySightings struct {
	mu    sync.Mutex
	rows  []Sighting
	fail  bool
	calls int
}

func (m *memorySightings) Insert(_ context.Context, sighting Sighting) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.fail {
		return errors.New("store unavailable")
	}
	m.rows = append(m.rows, sighting)
	return nil
}

func (m *memorySightings) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func TestTradeNexusRelaysSubscribedSightings(t *testing.T) {
	store := &memorySightings{}
	var invalidated atomic.Int32
	invalidatedSignal := make(chan struct{}, 1)
	hub := NewHub(Options{Store: store, Invalidate: func() {
		invalidated.Add(1)
		select {
		case invalidatedSignal <- struct{}{}:
		default:
		}
	}})
	server := httptest.NewServer(hub)
	defer server.Close()

	listener := dialTradeNexus(t, server.URL)
	defer listener.CloseNow()
	expectType(t, listener, "hello")
	writeFrame(t, listener, map[string]any{"v": 1, "type": "subscribe", "servers": []string{" greatest ", "Greatest"}})
	subscribed := readFrame(t, listener)
	if subscribed["type"] != "subscribed" {
		t.Fatalf("subscribed frame: %#v", subscribed)
	}
	servers, _ := subscribed["servers"].([]any)
	if len(servers) != 1 || servers[0] != "greatest" {
		t.Fatalf("servers were not trimmed and deduped: %#v", servers)
	}
	expectThievesSnapshot(t, listener, 0)

	reporter := dialTradeNexus(t, server.URL)
	defer reporter.CloseNow()
	expectType(t, reporter, "hello")
	writeFrame(t, reporter, map[string]any{
		"v": 1, "type": "thief.report", "ref": "ref-1", "server": "GREATEST",
		"thief": map[string]string{"name": " Bandit "}, "position_source": "thief",
		"position": map[string]any{"region": 25735, "x": 10.5, "y": -4, "z": 1},
		"reporter": map[string]string{"name": "trader1", "app": "AdvancedAutoTrade", "version": "1.0.0"},
		"origin":   "phmon",
	})
	ack := readFrame(t, reporter)
	if ack["type"] != "ack" || ack["ref"] != "ref-1" || ack["sighting_id"] == "" {
		t.Fatalf("ack: %#v", ack)
	}
	echo := readFrame(t, listener)
	if echo["type"] != "thief.sighting" || echo["sighting_id"] != ack["sighting_id"] || echo["origin"] != "external" {
		t.Fatalf("broadcast: %#v", echo)
	}
	// Socket delivery can finish before Broadcast invokes its refresh callback.
	select {
	case <-invalidatedSignal:
	case <-time.After(2 * time.Second):
		t.Fatal("sighting did not invalidate the operator snapshot")
	}
	if echo["server"] != "GREATEST" || invalidated.Load() != 1 || store.callCount() != 1 {
		t.Fatalf("stored=%d invalidated=%d broadcast=%#v", store.callCount(), invalidated.Load(), echo)
	}
	thief, _ := echo["thief"].(map[string]any)
	if thief["name"] != "Bandit" {
		t.Fatalf("thief name: %#v", echo["thief"])
	}

	outsider := dialTradeNexus(t, server.URL)
	defer outsider.CloseNow()
	expectType(t, outsider, "hello")
	writeFrame(t, outsider, map[string]any{"v": 1, "type": "subscribe", "servers": []string{"Other"}})
	expectType(t, outsider, "subscribed")
	expectThievesSnapshot(t, outsider, 0)
	writeFrame(t, reporter, map[string]any{
		"v": 1, "type": "thief.report", "ref": "ref-2", "server": "Greatest",
		"thief": map[string]string{"name": "Second"},
	})
	if readFrame(t, reporter)["type"] != "ack" || readFrame(t, listener)["thief"].(map[string]any)["name"] != "Second" {
		t.Fatal("second sighting was not delivered to the subscribed listener")
	}
	outsider.SetReadLimit(MaxFrameBytes)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, _, err := outsider.Read(ctx); err == nil {
		t.Fatal("unsubscribed server received a sighting")
	}
}

func TestTradeNexusRejectsInvalidAndOversizedFrames(t *testing.T) {
	hub := NewHub(Options{MaxInvalidFrames: 5})
	server := httptest.NewServer(hub)
	defer server.Close()
	conn := dialTradeNexus(t, server.URL)
	defer conn.CloseNow()
	expectType(t, conn, "hello")

	writeFrame(t, conn, map[string]any{"v": 1, "type": "thief.report", "ref": "bad", "server": "Greatest"})
	errFrame := readFrame(t, conn)
	if errFrame["code"] != "invalid_message" || errFrame["ref"] != "bad" || !strings.Contains(errFrame["message"].(string), "thief.name") {
		t.Fatalf("validation error: %#v", errFrame)
	}
	writeFrame(t, conn, map[string]any{"v": 2, "type": "subscribe", "ref": "old"})
	if readFrame(t, conn)["code"] != "unsupported_version" {
		t.Fatal("version was accepted")
	}
	writeFrame(t, conn, map[string]any{"v": 1, "type": "chat", "ref": "nope"})
	if readFrame(t, conn)["code"] != "unsupported_type" {
		t.Fatal("unknown type was accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, []byte(strings.Repeat("x", MaxFrameBytes+1))); err != nil {
		t.Fatal(err)
	}
	if readFrame(t, conn)["code"] != "too_large" {
		t.Fatal("oversize frame was accepted")
	}
	writeFrame(t, conn, map[string]any{"v": 1, "type": "nope"})
	if readFrame(t, conn)["code"] != "unsupported_type" {
		t.Fatal("the closing error frame was not written")
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("connection stayed open after five invalid frames")
	}
}

func TestTradeNexusRateLimitAndConnectionCap(t *testing.T) {
	hub := NewHub(Options{ReportsPerWindow: 1, MaxPerIP: 1, Now: func() time.Time {
		return time.Date(2026, 10, 6, 19, 0, 0, 0, time.UTC)
	}})
	server := httptest.NewServer(hub)
	defer server.Close()
	conn := dialTradeNexus(t, server.URL)
	defer conn.CloseNow()
	expectType(t, conn, "hello")
	report := map[string]any{"v": 1, "type": "thief.report", "server": "Greatest", "thief": map[string]string{"name": "Bandit"}}
	writeFrame(t, conn, report)
	if readFrame(t, conn)["type"] != "ack" {
		t.Fatal("first report was not acknowledged")
	}
	writeFrame(t, conn, report)
	limited := readFrame(t, conn)
	if limited["code"] != "rate_limited" {
		t.Fatalf("rate limit: %#v", limited)
	}
	response, err := http.Get(server.URL + "/tradenexus")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("connection cap status = %d", response.StatusCode)
	}
}

func TestTradeNexusReplacesStaleObservedTimeAndBroadcastsStoreFailure(t *testing.T) {
	now := time.Date(2026, 10, 6, 19, 0, 0, 0, time.UTC)
	store := &memorySightings{fail: true}
	hub := NewHub(Options{Store: store, Now: func() time.Time { return now }})
	server := httptest.NewServer(hub)
	defer server.Close()
	conn := dialTradeNexus(t, server.URL)
	defer conn.CloseNow()
	expectType(t, conn, "hello")
	writeFrame(t, conn, map[string]any{"v": 1, "type": "subscribe", "servers": []string{"Greatest"}})
	expectType(t, conn, "subscribed")
	expectThievesSnapshot(t, conn, 0)
	writeFrame(t, conn, map[string]any{
		"v": 1, "type": "thief.report", "server": "Greatest", "thief": map[string]string{"name": "Bandit"},
		"observed_at": now.Add(-time.Hour).Format(time.RFC3339),
	})
	if readFrame(t, conn)["type"] != "ack" {
		t.Fatal("store failure suppressed the acknowledgement")
	}
	broadcast := readFrame(t, conn)
	if broadcast["observed_at"] != now.Format(time.RFC3339Nano) || store.callCount() != 1 {
		t.Fatalf("stale time or store call: %#v calls=%d", broadcast["observed_at"], store.callCount())
	}
}

func TestSlowConsumerIsClosed(t *testing.T) {
	hub := NewHub(Options{SendQueue: 1})
	subscriber := &client{
		hub: hub, send: make(chan outbound, 1), closed: make(chan struct{}),
		servers: map[string]struct{}{"Greatest": {}},
	}
	hub.add(subscriber)
	hub.enqueue(subscriber, outbound{payload: []byte("one")})
	hub.enqueue(subscriber, outbound{payload: []byte("two")})
	select {
	case <-subscriber.closed:
	case <-time.After(time.Second):
		t.Fatal("full send queue left the client connected")
	}
}

func TestSightingFromEventUsesThiefPositionThenObserver(t *testing.T) {
	region, x, y, z := 25000, 12.0, 34.0, 8.0
	withThief := eventsPayload(t, map[string]any{
		"value": "bandit", "position_source": "thief", "plugin_version": "1.9.24",
		"thief": map[string]any{"name": "Bandit", "region": region, "x": x, "y": y},
	})
	sighting, ok := SightingFromEvent(eventWithPayload(withThief, &region, &x, &y, &z))
	if !ok || sighting.Origin != OriginPhMon || sighting.PositionSource != SourceThief || sighting.Position.Region != region || sighting.Reporter.Version != "1.9.24" {
		t.Fatalf("thief position sighting: %+v ok=%v", sighting, ok)
	}
	if sighting.Position.Z != nil {
		t.Fatal("thief lookup position copied the observer Z")
	}
	observerOnly, ok := SightingFromEvent(eventWithPayload([]byte(`{"value":"Bandit"}`), &region, &x, &y, &z))
	if !ok || observerOnly.PositionSource != SourceObserver || observerOnly.Position.Z == nil || *observerOnly.Position.Z != z {
		t.Fatalf("observer fallback: %+v", observerOnly)
	}
}

func eventsPayload(t *testing.T, value map[string]any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func eventWithPayload(payload []byte, region *int, x, y, z *float64) events.Event {
	return events.Event{
		ID: "00000000-0000-4000-8000-000000000010", Kind: "job.thief_seen", Server: "Greatest", Character: "nuker1",
		OccurredAt: time.Date(2026, 10, 6, 19, 0, 0, 0, time.UTC), Payload: payload,
		Region: region, X: x, Y: y, Z: z,
	}
}

func TestTradeNexusSmokeScript(t *testing.T) {
	hub := NewHub(Options{})
	server := httptest.NewServer(hub)
	defer server.Close()
	script := filepath.Clean(filepath.Join("..", "..", "..", "scripts", "tradenexus_smoke.py"))
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("smoke script: %v", err)
	}
	cmd := exec.Command("python3", script)
	cmd.Env = append(os.Environ(), "TRADENEXUS_URL="+server.URL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("smoke: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "tradenexus smoke passed") {
		t.Fatalf("smoke output: %s", output)
	}
}

func dialTradeNexus(t *testing.T, httpURL string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpURL, "http")+"/tradenexus", nil)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadLimit(ReadLimitBytes)
	return conn
}

func TestTradeNexusActiveThiefSnapshot(t *testing.T) {
	start := time.Date(2026, 10, 6, 19, 0, 0, 0, time.UTC)
	var current atomic.Value
	current.Store(start)
	store := &memorySightings{}
	hub := NewHub(Options{Store: store, Now: func() time.Time { return current.Load().(time.Time) }})
	server := httptest.NewServer(hub)
	defer server.Close()

	publisher := dialTradeNexus(t, server.URL)
	defer publisher.CloseNow()
	expectType(t, publisher, "hello")
	writeFrame(t, publisher, map[string]any{
		"v": 1, "type": "thief.report", "server": "Greatest",
		"thief":    map[string]string{"name": "Bandit"},
		"position": map[string]any{"region": 25735, "x": 1, "y": 2},
	})
	expectType(t, publisher, "ack")

	late := dialTradeNexus(t, server.URL)
	defer late.CloseNow()
	expectType(t, late, "hello")
	writeFrame(t, late, map[string]any{"v": 1, "type": "subscribe", "servers": []string{"Greatest"}})
	expectType(t, late, "subscribed")
	snapshot := expectThievesSnapshot(t, late, 1)
	sightings, _ := snapshot["sightings"].([]any)
	first, _ := sightings[0].(map[string]any)
	if first["type"] != "thief.sighting" || first["thief"].(map[string]any)["name"] != "Bandit" {
		t.Fatalf("snapshot sighting: %#v", first)
	}

	writeFrame(t, late, map[string]any{"v": 1, "type": "subscribe", "servers": []string{"Other"}})
	expectType(t, late, "subscribed")
	expectThievesSnapshot(t, late, 0)

	current.Store(start.Add(11 * time.Minute))
	expired := dialTradeNexus(t, server.URL)
	defer expired.CloseNow()
	expectType(t, expired, "hello")
	writeFrame(t, expired, map[string]any{"v": 1, "type": "subscribe", "servers": []string{"Greatest"}})
	expectType(t, expired, "subscribed")
	expectThievesSnapshot(t, expired, 0)

	current.Store(start.Add(2 * time.Minute))
	writeFrame(t, publisher, map[string]any{
		"v": 1, "type": "thief.report", "server": "Greatest",
		"thief":    map[string]string{"name": "Bandit"},
		"position": map[string]any{"region": 25735, "x": 9, "y": 8},
	})
	expectType(t, publisher, "ack")
	updatedConn := dialTradeNexus(t, server.URL)
	defer updatedConn.CloseNow()
	expectType(t, updatedConn, "hello")
	writeFrame(t, updatedConn, map[string]any{"v": 1, "type": "subscribe", "servers": []string{"Greatest"}})
	expectType(t, updatedConn, "subscribed")
	snapshot = expectThievesSnapshot(t, updatedConn, 1)
	sightings, _ = snapshot["sightings"].([]any)
	updated, _ := sightings[0].(map[string]any)
	position, _ := updated["position"].(map[string]any)
	if position["x"] != 9.0 || position["y"] != 8.0 {
		t.Fatalf("latest position was not retained: %#v", updated)
	}
}

func TestTradeNexusInvalidateCoalescesBursts(t *testing.T) {
	var invalidated atomic.Int32
	hub := NewHub(Options{Invalidate: func() { invalidated.Add(1) }})
	for range 40 {
		hub.scheduleInvalidate()
	}
	if invalidated.Load() != 1 {
		t.Fatalf("immediate invalidations = %d", invalidated.Load())
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && invalidated.Load() < 2 {
		time.Sleep(20 * time.Millisecond)
	}
	if invalidated.Load() != 2 {
		t.Fatalf("coalesced invalidations = %d", invalidated.Load())
	}
}

func writeFrame(t *testing.T, conn *websocket.Conn, frame map[string]any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, frame); err != nil {
		t.Fatal(err)
	}
}

func readFrame(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var frame map[string]any
	if err := wsjson.Read(ctx, conn, &frame); err != nil {
		t.Fatal(err)
	}
	return frame
}

func expectType(t *testing.T, conn *websocket.Conn, kind string) {
	t.Helper()
	frame := readFrame(t, conn)
	if frame["type"] != kind {
		t.Fatalf("frame type = %#v, want %s (%#v)", frame["type"], kind, frame)
	}
}

func expectThievesSnapshot(t *testing.T, conn *websocket.Conn, wantCount int) map[string]any {
	t.Helper()
	frame := readFrame(t, conn)
	if frame["type"] != "thieves" {
		t.Fatalf("frame type = %#v, want thieves", frame)
	}
	sightings, _ := frame["sightings"].([]any)
	if len(sightings) != wantCount {
		t.Fatalf("thieves snapshot count = %d, want %d (%#v)", len(sightings), wantCount, frame)
	}
	return frame
}

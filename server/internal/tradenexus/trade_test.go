package tradenexus

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReportTradeAcceptsTheContract(t *testing.T) {
	now := time.Date(2026, 10, 9, 7, 3, 30, 0, time.UTC)
	payload := sampleTradePayload(t, now, nil)
	report, err := reportTrade(payload, now)
	if err.Code != "" {
		t.Fatalf("report: %#v", err)
	}
	if report.Route.From != "Jangan" || report.Route.To != "Donwhang" || report.Reporter.Name != "CharName" {
		t.Fatalf("normalized report: %+v", report)
	}
	if report.Gold == nil || *report.Gold != 45000 || len(report.Waypoints) != 1 || report.Waypoints[0].X != 37643 {
		t.Fatalf("optional fields: %+v", report)
	}
	if report.FinishedAt.Format(time.RFC3339) != "2026-10-09T07:03:00Z" {
		t.Fatalf("finished_at = %s", report.FinishedAt)
	}
}

func TestReportTradeReplacesSkewedFinishedAt(t *testing.T) {
	now := time.Date(2026, 10, 9, 7, 3, 30, 0, time.UTC)
	payload := sampleTradePayload(t, now, map[string]any{"finished_at": "2026-10-09T06:00:00Z"})
	report, err := reportTrade(payload, now)
	if err.Code != "" || !report.FinishedAt.Equal(now) {
		t.Fatalf("report=%+v err=%#v", report, err)
	}
}

func TestReportTradeRejectsInvalidShapes(t *testing.T) {
	now := time.Date(2026, 10, 9, 7, 3, 30, 0, time.UTC)
	cases := []struct {
		name    string
		changes map[string]any
		message string
	}{
		{name: "missing ref", changes: map[string]any{"ref": " "}, message: "ref is required"},
		{name: "same town", changes: map[string]any{"route": map[string]string{"from": "Hotan", "to": "hotan"}}, message: "must differ"},
		{name: "unknown town", changes: map[string]any{"route": map[string]string{"from": "Jangan", "to": "Taklamakan"}}, message: "route.to"},
		{name: "reason mismatch", changes: map[string]any{"outcome": "success", "reason": "thief"}, message: "do not match"},
		{name: "detail on sold", changes: map[string]any{"detail": "extra"}, message: "detail is only allowed"},
		{name: "thief on sold", changes: map[string]any{"thief": map[string]string{"name": "Bandit"}}, message: "only allowed for reason thief"},
		{name: "missing thief name", changes: map[string]any{"outcome": "failed", "reason": "thief"}, message: "thief.name"},
		{name: "goods on empty", changes: map[string]any{"reason": "already_empty"}, message: "already_empty"},
		{name: "gold fraction", changes: map[string]any{"gold": 1.5}, message: "gold is invalid"},
		{name: "stars number", changes: map[string]any{"stars": 5}, message: "stars is invalid"},
		{name: "bad waypoint", changes: map[string]any{"waypoints": []map[string]any{{"name": "Chau", "x": 1, "y": 2}}}, message: "waypoint name"},
		{name: "missing coordinate", changes: map[string]any{"waypoints": []map[string]any{{"name": "chau", "x": 1}}}, message: "waypoint is invalid"},
		{name: "string id list", changes: map[string]any{"waypoints": []string{"chau_approach"}}, message: "waypoint is invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := reportTrade(sampleTradePayload(t, now, tc.changes), now)
			if err.Code != "invalid_message" || !strings.Contains(err.Message, tc.message) {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestTradeFrameSizeDependsOnType(t *testing.T) {
	now := time.Date(2026, 10, 9, 7, 3, 30, 0, time.UTC)
	trade := sampleTradePayload(t, now, map[string]any{
		"waypoints": repeatWaypoints(120),
	})
	if len(trade) <= MaxFrameBytes || len(trade) > MaxTradeFrameBytes {
		t.Fatalf("trade fixture length = %d", len(trade))
	}
	if _, err := parseClientFrame(trade); err.Code != "" {
		t.Fatalf("trade frame: %#v", err)
	}
	thief := []byte(`{"v":1,"type":"thief.report","server":"Greatest","thief":{"name":"Bandit"},"pad":"` + strings.Repeat("a", 5000) + `"}`)
	if len(thief) <= MaxFrameBytes {
		t.Fatalf("thief fixture length = %d", len(thief))
	}
	if _, err := parseClientFrame(thief); err.Code != "too_large" {
		t.Fatalf("oversized thief: %#v", err)
	}
	oversized := append([]byte(nil), trade...)
	oversized = append(oversized, bytesRepeat(MaxTradeFrameBytes)...)
	if len(oversized) <= MaxTradeFrameBytes {
		t.Fatal("oversized trade fixture is still inside the cap")
	}
	if _, err := parseClientFrame(oversized[:MaxTradeFrameBytes+1]); err.Code != "too_large" {
		t.Fatalf("oversized trade: %#v", err)
	}
}

func sampleTradePayload(t *testing.T, now time.Time, changes map[string]any) []byte {
	t.Helper()
	frame := map[string]any{
		"v": 1, "type": "trade.report", "ref": "aat-1728460800-3", "server": "Greatest",
		"outcome": "success", "reason": "sold",
		"route":     map[string]string{"from": "jangan", "to": "Donwhang"},
		"waypoints": []map[string]any{{"name": "chau_approach", "x": 37643, "y": 7342}},
		"goods":     []map[string]any{{"name": "Silk", "quantity": 120}},
		"gold":      45000, "duration_s": 842, "stars": "Max",
		"reporter":    map[string]string{"app": "AdvancedAutoTrade", "version": "1.0.0", "name": " CharName "},
		"finished_at": now.Add(-30 * time.Second).UTC().Format(time.RFC3339),
	}
	for key, value := range changes {
		if value == nil {
			delete(frame, key)
		} else {
			frame[key] = value
		}
	}
	payload, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func repeatWaypoints(count int) []map[string]any {
	points := make([]map[string]any, count)
	for i := range points {
		points[i] = map[string]any{"name": "node_" + strings.Repeat("a", 8), "x": 1000 + i, "y": 2000 + i}
	}
	return points
}

func bytesRepeat(count int) []byte {
	return []byte(strings.Repeat("x", count))
}

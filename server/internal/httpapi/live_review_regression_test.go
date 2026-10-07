package httpapi

import (
	"context"
	"testing"
)

func TestAnalyticsPassPreservesPendingPositionSnapshot(t *testing.T) {
	c := &liveClient{
		subscriptions: map[string]liveSubscription{
			"map-position": {ID: "map-position", Stream: "positions", Revision: 1},
			"chart":        {ID: "chart", Stream: "analytics", Revision: 1},
		},
		positionSnapshots: map[string]uint64{"map-position": 1},
	}
	c.snapshotSubscriptionsForFlags(2)
	for _, subscription := range c.snapshotSubscriptionsForFlags(1) {
		if subscription.ID == "map-position" {
			return
		}
	}
	t.Fatal("analytics-only pass discarded a pending position snapshot")
}

func TestInvalidAnalyticsFilterDoesNotRejectSharedLiveConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &liveClient{ctx: ctx, cancel: cancel, outgoing: make(chan []byte, 1)}
	if !c.subscribe(liveClientMessage{
		Type: "subscribe", ProtocolVersion: liveProtocolVersion,
		SubscriptionID: "analytics-feed", Revision: 1, Stream: "analytics",
		Filter: liveFilter{AnalyticsView: "deaths", From: "not-a-date", To: "also-not-a-date"},
	}) {
		t.Fatal("analytics filter rejection terminated the shared live connection")
	}
	if len(c.outgoing) != 1 {
		t.Fatal("invalid analytics filter was not returned as a subscription rejection")
	}
}

func TestReviewAnalyticsPassMustPreservePendingPositionSnapshots(t *testing.T) {
	c := &liveClient{subscriptions: map[string]liveSubscription{
		"map-position": {ID: "map-position", Stream: "positions", Revision: 1},
		"chart":        {ID: "chart", Stream: "analytics", Revision: 1},
	}, positionSnapshots: map[string]uint64{"map-position": 1}}
	c.snapshotSubscriptionsForFlags(2)
	next := c.snapshotSubscriptionsForFlags(1)
	for _, s := range next {
		if s.ID == "map-position" {
			return
		}
	}
	t.Fatal("analytics-only pass discarded requested map position snapshot; standard pass never sends it")
}
func TestReviewRedundantWakeMustNotBuildAllStreams(t *testing.T) {
	c := &liveClient{}
	if got := c.takeInvalidateFlags(); got != 0 {
		t.Fatalf("empty flags became %d, rebuilding both stream classes", got)
	}
}

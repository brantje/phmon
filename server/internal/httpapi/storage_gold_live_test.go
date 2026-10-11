package httpapi

import (
	"fmt"
	"testing"
	"time"
)

func TestLiveProgressAdmitsOnlyBoundedPerformanceBatches(t *testing.T) {
	ids := make([]string, 51)
	for i := range ids {
		ids[i] = fmt.Sprintf("%08x-bbbb-4ccc-8ddd-eeeeeeeeeeee", i+1)
	}
	now := time.Now().UTC()
	base := liveFilter{CharacterIDs: ids[:1], AnalyticsView: "performance", From: now.Add(-24 * time.Hour).Format(time.RFC3339Nano), To: now.Format(time.RFC3339Nano), Timezone: "UTC", Bucket: "hour"}
	for _, n := range []int{1, 50} {
		filter := base
		filter.CharacterIDs = ids[:n]
		if _, ok := validateLiveSubscription(liveClientMessage{SubscriptionID: "analytics-visible-progress", Revision: 1, Stream: "analytics", Filter: filter}); !ok {
			t.Fatalf("valid progress batch of %d rejected", n)
		}
	}
	cases := map[string]liveFilter{}
	filter := base
	filter.CharacterIDs = ids
	cases["oversized"] = filter
	filter = base
	filter.CharacterIDs = []string{ids[0], ids[0]}
	cases["duplicate"] = filter
	filter = base
	filter.CharacterIDs = []string{"invalid"}
	cases["invalid id"] = filter
	filter = base
	filter.CharacterIDs = []string{}
	cases["empty performance"] = filter
	filter = base
	filter.CharacterID = ids[0]
	cases["mixed targets"] = filter
	filter = base
	filter.AnalyticsView = "economy"
	cases["economy batch"] = filter
	for name, filter := range cases {
		if _, ok := validateLiveSubscription(liveClientMessage{SubscriptionID: "analytics-visible-progress", Revision: 1, Stream: "analytics", Filter: filter}); ok {
			t.Fatalf("invalid %s batch admitted", name)
		}
	}
	if _, ok := validateLiveSubscription(liveClientMessage{SubscriptionID: "characters", Revision: 1, Stream: "characters", Filter: liveFilter{CharacterIDs: ids[:1]}}); ok {
		t.Fatal("non-batch stream admitted character targets")
	}
}

package httpapi

import (
	"net/url"
	"testing"
	"time"
)

func TestPlayerFilterHidesOwnCharactersByDefault(t *testing.T) {
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	filter, err := playerFilter(url.Values{}, now)
	if err != nil || !filter.HideOwn {
		t.Fatalf("default hide own = %+v %v", filter.HideOwn, err)
	}
	filter, err = playerFilter(url.Values{"hide_own": {"0"}}, now)
	if err != nil || filter.HideOwn {
		t.Fatalf("hide_own=0 = %+v %v", filter.HideOwn, err)
	}
	filter, err = playerFilter(url.Values{"hide_own": {"false"}}, now)
	if err != nil || filter.HideOwn {
		t.Fatalf("hide_own=false = %+v %v", filter.HideOwn, err)
	}
	if _, err = playerFilter(url.Values{"hide_own": {"maybe"}}, now); err == nil {
		t.Fatal("expected invalid hide_own")
	}
}

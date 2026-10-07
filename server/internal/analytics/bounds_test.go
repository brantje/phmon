package analytics

import (
	"fmt"
	"testing"
)

func TestBoundTimeSeriesKeepsNewestPointsPerSeries(t *testing.T) {
	points := make([]Point, 0, 1200)
	for index := 0; index < 700; index++ {
		points = append(points,
			Point{Bucket: fmt.Sprintf("%04d", index), Series: "one"},
			Point{Bucket: fmt.Sprintf("%04d", index), Series: "two"},
		)
	}
	got, truncated := boundTimeSeries(points, 500, false)
	if !truncated || len(got) != 1000 {
		t.Fatalf("bounded points = %d, truncated=%v; want 1000 and true", len(got), truncated)
	}
	first := map[string]string{}
	last := map[string]string{}
	for _, point := range got {
		if _, exists := first[point.Series]; !exists {
			first[point.Series] = point.Bucket
		}
		last[point.Series] = point.Bucket
	}
	for _, series := range []string{"one", "two"} {
		if first[series] != "0200" || last[series] != "0699" {
			t.Fatalf("series %q has range %q..%q; want newest 500 buckets 0200..0699", series, first[series], last[series])
		}
	}
}

func TestBoundTimeSeriesPreservesExistingTruncation(t *testing.T) {
	got, truncated := boundTimeSeries([]Point{{Series: "one"}}, 500, true)
	if !truncated || len(got) != 1 {
		t.Fatalf("boundTimeSeries overwrote existing truncation metadata: points=%v truncated=%v", got, truncated)
	}
}

package analytics

import "testing"

func TestBoundTrainingPointsKeepsLargestAndAggregatesOther(t *testing.T) {
	points := []TrainingPoint{
		{Region: intPtr(1), Zone: "A", CoveredSecond: 30},
		{Region: intPtr(2), Zone: "B", CoveredSecond: 20},
		{Region: intPtr(3), Zone: "C", CoveredSecond: 10},
	}
	got, limited := boundTrainingPoints(points, 2)
	if !limited || len(got) != 3 || got[2].Zone != "Other locations" || got[2].CoveredSecond != 10 {
		t.Fatalf("bounded training result = %#v, limited=%v", got, limited)
	}
	if _, limited := boundTrainingPoints(points[:2], 2); limited {
		t.Fatal("complete training result was reported as limited")
	}
}

func intPtr(value int) *int { return &value }

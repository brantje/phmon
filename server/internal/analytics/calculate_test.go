package analytics

import (
	"testing"
	"time"
)

func int64Ptr(value int64) *int64 { return &value }

func TestCalculateBalanceRateUsesOnlyComparableCoveredIntervals(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(3 * time.Minute)
	level := 10
	maxXP := int64(1000)
	botting := true
	samples := []MetricSample{
		{SessionID: "one", At: from.Add(-10 * time.Second), Level: &level, CurrentXP: int64Ptr(90), MaxXP: &maxXP, SP: int64Ptr(210), Gold: int64Ptr(900), Botting: &botting},
		{SessionID: "one", At: from, Level: &level, CurrentXP: int64Ptr(100), MaxXP: &maxXP, SP: int64Ptr(200), Gold: int64Ptr(1000), Botting: &botting},
		{SessionID: "one", At: from.Add(30 * time.Second), Level: &level, CurrentXP: int64Ptr(130), MaxXP: &maxXP, SP: int64Ptr(190), Gold: int64Ptr(1300), Botting: &botting},
		{SessionID: "one", At: from.Add(60 * time.Second), Level: &level, CurrentXP: int64Ptr(160), MaxXP: &maxXP, SP: int64Ptr(180), Gold: int64Ptr(1200), Botting: &botting},
		// A long outage must not be bridged into the rate denominator.
		{SessionID: "one", At: from.Add(4 * time.Minute), Level: &level, CurrentXP: int64Ptr(900), MaxXP: &maxXP, SP: int64Ptr(300), Gold: int64Ptr(9000), Botting: &botting},
	}
	xp := CalculateProgressRates(samples, from, to)["xp"]
	if xp.Status != "available" || xp.Delta != 60 || xp.EligibleSeconds != 60 || xp.PerHour != 3600 {
		t.Fatalf("same-level XP result = %#v", xp)
	}
	sp := CalculateProgressRates(samples, from, to)["sp"]
	if sp.Delta != -20 || sp.PerHour != -1200 {
		t.Fatalf("negative SP balance change was not preserved: %#v", sp)
	}
	if xp.GeneralCoverageSecond != 60 || xp.BottingSeconds != 60 || xp.UnknownBotSeconds != 0 {
		t.Fatalf("coverage = %#v", xp)
	}
}

func TestCalculateRatesBreakOnSessionGapLevelOrUnknownValues(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(2 * time.Minute)
	level10, level11 := 10, 11
	max := int64(1000)
	falseValue := false
	samples := []MetricSample{
		{SessionID: "old", At: from, Level: &level10, CurrentXP: int64Ptr(900), MaxXP: &max, Gold: int64Ptr(20), Botting: &falseValue},
		{SessionID: "old", At: from.Add(30 * time.Second), Level: &level11, CurrentXP: int64Ptr(40), MaxXP: &max, Gold: int64Ptr(15), Botting: &falseValue},
		{SessionID: "old", At: from.Add(time.Minute), Level: &level11, CurrentXP: int64Ptr(80), MaxXP: &max, Gold: int64Ptr(10), Botting: &falseValue},
		{SessionID: "new", At: from.Add(70 * time.Second), Level: &level11, CurrentXP: nil, MaxXP: &max, Gold: int64Ptr(20), Botting: nil},
		{SessionID: "new", At: from.Add(2 * time.Minute), Level: &level11, CurrentXP: int64Ptr(50), MaxXP: &max, Gold: int64Ptr(30), Botting: nil},
	}
	rates := CalculateProgressRates(samples, from, to)
	if rates["xp"].HasRate || rates["xp"].Status != "insufficient_history" {
		t.Fatalf("XP rollover or missing endpoint should not produce a rate: %#v", rates["xp"])
	}
	if !rates["gold"].HasRate || rates["gold"].Delta != -10 || rates["gold"].EligibleSeconds != 60 {
		t.Fatalf("gold rate must preserve the observed loss without bridging sessions: %#v", rates["gold"])
	}
}

func TestCalculateBalanceRateReportsInsufficientHistoryNotZero(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	samples := []MetricSample{
		{SessionID: "same", At: from, Gold: int64Ptr(500)},
		{SessionID: "same", At: from.Add(30 * time.Second), Gold: int64Ptr(500)},
	}
	got := CalculateBalanceRate(samples, goldBalance, from, from.Add(time.Minute))
	if got.HasRate || got.Status != "insufficient_history" || got.Reason != "insufficient_history" || got.PerHour != 0 {
		t.Fatalf("sub-minute history must be null/insufficient, got %#v", got)
	}
}

func TestLocalCalendarDayCountUsesIANADateBoundariesAcrossDST(t *testing.T) {
	cases := []struct {
		name string
		from time.Time
		to   time.Time
	}{
		{
			name: "spring-forward day",
			from: time.Date(2026, 3, 28, 23, 0, 0, 0, time.UTC),
			to:   time.Date(2026, 3, 29, 22, 0, 0, 0, time.UTC),
		},
		{
			name: "fall-back day",
			from: time.Date(2026, 10, 24, 22, 0, 0, 0, time.UTC),
			to:   time.Date(2026, 10, 25, 23, 0, 0, 0, time.UTC),
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			days, err := localCalendarDays(test.from, test.to, "Europe/Amsterdam")
			if err != nil || days != 1 {
				t.Fatalf("local date count = %d, error=%v; want one day", days, err)
			}
		})
	}
}

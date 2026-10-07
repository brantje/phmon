package analytics

import (
	"math"
	"math/big"
	"sort"
	"strconv"
	"time"
)

const (
	CalculationVersion = "analytics-v1"
	MaxSampleGap       = 30 * time.Second
	MinimumRateHistory = 60 * time.Second
)

type MetricSample struct {
	SessionID string
	At        time.Time
	Level     *int
	CurrentXP *int64
	MaxXP     *int64
	SP        *int64
	Gold      *int64
	Botting   *bool
}

type RateResult struct {
	Delta                 float64 `json:"delta"`
	DeltaExact            string  `json:"delta_exact,omitempty"`
	PerHour               float64 `json:"per_hour"`
	HasRate               bool    `json:"has_rate"`
	EligibleSeconds       float64 `json:"eligible_seconds"`
	GeneralCoverageSecond float64 `json:"general_coverage_seconds"`
	UnknownBotSeconds     float64 `json:"unknown_bot_seconds"`
	BottingSeconds        float64 `json:"botting_seconds"`
	IdleSeconds           float64 `json:"idle_seconds"`
	Status                string  `json:"status"`
	Reason                string  `json:"reason,omitempty"`
}

// CalculateBalanceRate calculates an observed net balance rate from adjacent
// samples in the same session. The ending sample owns the delta's bucket, so a
// rate window includes a delta only when both endpoints lie within it.
func CalculateBalanceRate(samples []MetricSample, metric func(MetricSample) *int64, from, to time.Time) RateResult {
	return calculateBalanceRate(samples, metric, from, to, false)
}

func calculateBalanceRate(samples []MetricSample, metric func(MetricSample) *int64, from, to time.Time, requireComparableLevel bool) RateResult {
	ordered := append([]MetricSample(nil), samples...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].At.Before(ordered[j].At) })
	result := RateResult{Status: "insufficient_history", Reason: "insufficient_history"}
	exactDelta := new(big.Int)
	if metric == nil || from.IsZero() || to.IsZero() || !to.After(from) {
		result.Status, result.Reason = "unsupported", "invalid_window_or_metric"
		return result
	}
	for i := 1; i < len(ordered); i++ {
		before, after := ordered[i-1], ordered[i]
		if before.SessionID == "" || before.SessionID != after.SessionID || !after.At.After(before.At) {
			continue
		}
		span := after.At.Sub(before.At)
		if span > MaxSampleGap {
			continue
		}
		left, right := before.At, after.At
		if left.Before(from) {
			left = from
		}
		if right.After(to) {
			right = to
		}
		if !right.After(left) {
			continue
		}
		covered := right.Sub(left).Seconds()
		result.GeneralCoverageSecond += covered
		if before.Botting != nil && after.Botting != nil && *before.Botting == *after.Botting {
			if *before.Botting {
				result.BottingSeconds += covered
			} else {
				result.IdleSeconds += covered
			}
		} else {
			result.UnknownBotSeconds += covered
		}
		// The observed delta is attributed to the ending sample. Never split a
		// boundary interval to manufacture a balance change for this window.
		if before.At.Before(from) || after.At.After(to) {
			continue
		}
		oldValue, newValue := metric(before), metric(after)
		if oldValue == nil || newValue == nil {
			continue
		}
		if requireComparableLevel && (before.Level == nil || after.Level == nil || *before.Level != *after.Level || before.MaxXP == nil || after.MaxXP == nil || *before.MaxXP != *after.MaxXP) {
			continue
		}
		intervalDelta := new(big.Int).Sub(big.NewInt(*newValue), big.NewInt(*oldValue))
		exactDelta.Add(exactDelta, intervalDelta)
		result.EligibleSeconds += span.Seconds()
	}
	result.DeltaExact = exactDelta.String()
	result.Delta, _ = new(big.Float).SetInt(exactDelta).Float64()
	if result.EligibleSeconds >= MinimumRateHistory.Seconds() {
		result.PerHour = result.Delta * 3600 / result.EligibleSeconds
		if math.IsInf(result.PerHour, 0) || math.IsNaN(result.PerHour) {
			result.PerHour = 0
			result.Status, result.Reason = "insufficient_history", "non_finite_rate"
			return result
		}
		result.HasRate = true
		result.Status = "available"
		result.Reason = ""
	}
	return result
}

func currentXP(sample MetricSample) *int64   { return sample.CurrentXP }
func skillPoints(sample MetricSample) *int64 { return sample.SP }
func goldBalance(sample MetricSample) *int64 { return sample.Gold }

func CalculateProgressRates(samples []MetricSample, from, to time.Time) map[string]RateResult {
	return map[string]RateResult{
		"xp":   calculateBalanceRate(samples, currentXP, from, to, true),
		"sp":   CalculateBalanceRate(samples, skillPoints, from, to),
		"gold": CalculateBalanceRate(samples, goldBalance, from, to),
	}
}

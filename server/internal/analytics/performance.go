package analytics

import (
	"context"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type observedMetricSample struct {
	MetricSample
	Region    *int
	Zone      *string
	StartedAt time.Time
	EndedAt   *time.Time
}

// queryPerformance reads at most one character's 24-hour sample window. The
// 30-second lookback supplies a possible predecessor without bridging gaps.
func queryPerformance(ctx context.Context, tx pgx.Tx, filter Filter, snapshot *Snapshot) error {
	rows, err := tx.Query(ctx, `SELECT c.character_id::text,c.character_name,c.server_key,cs.started_at,cs.ended_at,
       s.session_id::text,s.sampled_at,s.level,s.current_exp,s.max_exp,s.sp,s.gold,
       s.botting,s.region,s.zone_name
FROM (
    SELECT * FROM character_metric_samples WHERE character_id=$1::uuid AND ($2='' OR server_key=$2)
      AND sampled_at >= $3::timestamptz-interval '30 seconds' AND sampled_at < $4
    ORDER BY sampled_at DESC,sample_id DESC LIMIT 10001
) s
JOIN characters c ON c.character_id=s.character_id
JOIN character_sessions cs ON cs.session_id=s.session_id
ORDER BY s.sampled_at,s.sample_id`, filter.CharacterID, filter.Server, filter.From, filter.To)
	if err != nil {
		return err
	}
	defer rows.Close()

	var performance CharacterPerformance
	var samples []observedMetricSample
	sessions := make(map[string]*SessionPerformance)
	for rows.Next() {
		var characterID, name, server, sessionID string
		var sessionStart, sampledAt time.Time
		var sessionEnd *time.Time
		var sample observedMetricSample
		if err := rows.Scan(&characterID, &name, &server, &sessionStart, &sessionEnd,
			&sessionID, &sampledAt, &sample.Level, &sample.CurrentXP, &sample.MaxXP,
			&sample.SP, &sample.Gold, &sample.Botting, &sample.Region, &sample.Zone); err != nil {
			return err
		}
		if performance.CharacterID == "" {
			performance.CharacterID, performance.Character, performance.Server = characterID, name, server
		}
		sample.SessionID, sample.At = sessionID, sampledAt.UTC()
		sample.StartedAt = sessionStart.UTC()
		if sessionEnd != nil {
			ended := sessionEnd.UTC()
			sample.EndedAt = &ended
		}
		samples = append(samples, sample)
		span := sessions[sessionID]
		if span == nil {
			span = &SessionPerformance{SessionID: sessionID, StartedAt: sessionStart.UTC(), FirstSeen: sample.At, LastSeen: sample.At}
			if sessionEnd != nil {
				ended := sessionEnd.UTC()
				span.EndedAt = &ended
			}
			sessions[sessionID] = span
		} else {
			span.LastSeen = sample.At
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE kind='character.died'),
count(*) FILTER(WHERE kind='drop.item'),count(*) FILTER(WHERE kind='drop.rare')
FROM activity_events WHERE character_id=$1::uuid AND lower(server_name)=CASE WHEN $2='' THEN lower(server_name) ELSE $2 END
AND occurred_at >= $3 AND occurred_at < $4`, filter.CharacterID, filter.Server,
		filter.To.Add(-24*time.Hour), filter.To).Scan(&performance.Deaths24h, &performance.NormalDrops24h, &performance.RareDrops24h); err != nil {
		return err
	}
	if len(samples) == 0 {
		snapshot.Status = "insufficient_history"
		snapshot.Reason = "no_accepted_state_samples_in_window"
		snapshot.Performance = &CharacterPerformance{CharacterID: filter.CharacterID, Rates: map[string]RateResult{}, Training: []TrainingPoint{}, Sessions: []SessionPerformance{}}
		return nil
	}
	if len(samples) > maxPerformanceSamples {
		samples = samples[len(samples)-maxPerformanceSamples:]
		performance.Truncated = true
		// The query selects the latest samples. Rebuild session metadata from that
		// bounded window so dropped older rows do not inflate observed spans.
		sessions = make(map[string]*SessionPerformance)
		for _, sample := range samples {
			span := sessions[sample.SessionID]
			if span == nil {
				span = &SessionPerformance{SessionID: sample.SessionID, StartedAt: sample.StartedAt, FirstSeen: sample.At, LastSeen: sample.At, EndedAt: sample.EndedAt}
				sessions[sample.SessionID] = span
			} else {
				span.LastSeen = sample.At
			}
		}
	}
	metricSamples := make([]MetricSample, len(samples))
	for index := range samples {
		metricSamples[index] = samples[index].MetricSample
	}
	resetAt, err := latestRateReset(ctx, tx, filter.CharacterID)
	if err != nil {
		return err
	}
	rateFrom := filter.From
	if resetAt != nil && resetAt.After(rateFrom) {
		if resetAt.Before(filter.To) {
			rateFrom = *resetAt
		} else if resetAt.Sub(filter.To) <= 5*time.Minute {
			// A live client may still be subscribed with its pre-reset fixed To.
			// Treat a nearby reset as the effective end of that live rate window;
			// old, genuinely historical windows remain unchanged.
			rateFrom = filter.To
		}
	}
	if rateFrom.Before(filter.To) {
		performance.Rates = CalculateProgressRates(metricSamples, rateFrom, filter.To)
	} else {
		performance.Rates = unavailableProgressRates("rate_window_reset")
	}
	performance.Training = trainingCoverage(samples, filter.From, filter.To)
	var trainingLimited bool
	performance.Training, trainingLimited = boundTrainingPoints(performance.Training, maxPerformanceTrainingPoints)
	performance.Sessions = sessionRows(sessions, samples, filter.From, filter.To)
	performance.SessionCount = len(performance.Sessions)
	if len(performance.Sessions) > maxPerformanceSessions {
		performance.SessionCount = len(performance.Sessions)
		performance.Sessions = performance.Sessions[:maxPerformanceSessions]
		performance.Truncated = true
	}
	performance.Truncated = performance.Truncated || trainingLimited

	for index := 1; index < len(samples); index++ {
		before, after := samples[index-1], samples[index]
		if before.SessionID != after.SessionID || !after.At.After(before.At) || after.At.Sub(before.At) > MaxSampleGap {
			continue
		}
		left, right := before.At, after.At
		if left.Before(filter.From) {
			left = filter.From
		}
		if right.After(filter.To) {
			right = filter.To
		}
		if !right.After(left) {
			continue
		}
		seconds := right.Sub(left).Seconds()
		performance.CoverageSeconds += seconds
		if before.Botting != nil && after.Botting != nil && *before.Botting == *after.Botting {
			if *before.Botting {
				performance.BottingSeconds += seconds
			} else {
				performance.IdleSeconds += seconds
			}
		} else {
			performance.UnknownBotSeconds += seconds
		}
	}

	latest := samples[len(samples)-1]
	latestAt := latest.At
	performance.LastSampleAt = &latestAt
	performance.Stale = !latest.At.After(time.Now().UTC().Add(-90 * time.Second))
	performance.CurrentLevel, performance.CurrentXP, performance.MaxXP = latest.Level, latest.CurrentXP, latest.MaxXP
	if !performance.Stale && latest.Level != nil && latest.CurrentXP != nil && latest.MaxXP != nil && *latest.MaxXP > *latest.CurrentXP {
		currentLevelRate := calculateCurrentLevelXPRate(samples, *latest.Level, *latest.MaxXP, rateFrom, filter.To)
		if currentLevelRate.HasRate && currentLevelRate.PerHour > 0 {
			percentRate := currentLevelRate.PerHour / float64(*latest.MaxXP) * 100
			performance.XPPercentPerHour = &percentRate
			seconds := float64(*latest.MaxXP-*latest.CurrentXP) / (currentLevelRate.PerHour / 3600)
			if !math.IsInf(seconds, 0) && !math.IsNaN(seconds) && seconds >= 0 {
				performance.LevelETASeconds = &seconds
			}
		}
	}

	snapshot.Status = "available"
	snapshot.Reason = ""
	if performance.Truncated || performance.Stale {
		snapshot.Status = "limited"
		if performance.Stale {
			snapshot.Reason = "latest_state_sample_stale"
		} else {
			snapshot.Reason = "performance_history_bounded"
		}
		snapshot.Truncated = performance.Truncated
	}
	snapshot.Total = "0"
	snapshot.Performance = &performance
	return nil
}

const (
	maxPerformanceSamples        = 10000
	maxPerformanceSessions       = 100
	maxPerformanceTrainingPoints = 100
)

func boundTrainingPoints(points []TrainingPoint, limit int) ([]TrainingPoint, bool) {
	if len(points) <= limit {
		return points, false
	}
	other := TrainingPoint{Zone: "Other locations"}
	for _, point := range points[limit:] {
		other.CoveredSecond += point.CoveredSecond
	}
	bounded := append([]TrainingPoint(nil), points[:limit]...)
	bounded = append(bounded, other)
	return bounded, true
}

func latestRateReset(ctx context.Context, tx pgx.Tx, characterID string) (*time.Time, error) {
	var resetAt *time.Time
	err := tx.QueryRow(ctx, `SELECT max(reset_at) FROM character_rate_resets WHERE character_id=$1::uuid`, characterID).Scan(&resetAt)
	if err != nil {
		return nil, err
	}
	if resetAt != nil {
		value := resetAt.UTC()
		resetAt = &value
	}
	return resetAt, nil
}

func unavailableProgressRates(reason string) map[string]RateResult {
	result := make(map[string]RateResult, 3)
	for _, metric := range []string{"xp", "sp", "gold"} {
		result[metric] = RateResult{Status: "insufficient_history", Reason: reason, DeltaExact: "0"}
	}
	return result
}

func calculateCurrentLevelXPRate(samples []observedMetricSample, level int, requirement int64, from, to time.Time) RateResult {
	result := RateResult{Status: "insufficient_history", Reason: "insufficient_history"}
	for index := 1; index < len(samples); index++ {
		before, after := samples[index-1], samples[index]
		if before.SessionID == "" || before.SessionID != after.SessionID || !after.At.After(before.At) || after.At.Sub(before.At) > MaxSampleGap {
			continue
		}
		if before.Level == nil || after.Level == nil || *before.Level != level || *after.Level != level || before.MaxXP == nil || after.MaxXP == nil || *before.MaxXP <= 0 || *before.MaxXP != requirement || *after.MaxXP != requirement || before.CurrentXP == nil || after.CurrentXP == nil {
			continue
		}
		if before.At.Before(from) || after.At.After(to) {
			continue
		}
		result.Delta += float64(*after.CurrentXP - *before.CurrentXP)
		result.EligibleSeconds += after.At.Sub(before.At).Seconds()
	}
	if result.EligibleSeconds >= MinimumRateHistory.Seconds() {
		result.PerHour = result.Delta * 3600 / result.EligibleSeconds
		result.HasRate, result.Status, result.Reason = true, "available", ""
	}
	return result
}

func trainingCoverage(samples []observedMetricSample, from, to time.Time) []TrainingPoint {
	type key struct {
		region string
		zone   string
	}
	totals := make(map[key]float64)
	regions := make(map[key]*int)
	for index := 1; index < len(samples); index++ {
		before, after := samples[index-1], samples[index]
		if before.SessionID == "" || before.SessionID != after.SessionID || !after.At.After(before.At) || after.At.Sub(before.At) > MaxSampleGap || before.Region == nil || after.Region == nil || *before.Region != *after.Region || before.Zone == nil || after.Zone == nil || *before.Zone != *after.Zone {
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
		k := key{region: fmtInt(*after.Region), zone: *after.Zone}
		region := *after.Region
		regions[k] = &region
		totals[k] += right.Sub(left).Seconds()
	}
	result := make([]TrainingPoint, 0, len(totals))
	for key, seconds := range totals {
		result = append(result, TrainingPoint{Region: regions[key], Zone: key.zone, CoveredSecond: seconds})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CoveredSecond == result[j].CoveredSecond {
			if result[i].Zone == result[j].Zone {
				return *result[i].Region < *result[j].Region
			}
			return result[i].Zone < result[j].Zone
		}
		return result[i].CoveredSecond > result[j].CoveredSecond
	})
	return result
}

func sessionRows(values map[string]*SessionPerformance, samples []observedMetricSample, from, to time.Time) []SessionPerformance {
	for index := 1; index < len(samples); index++ {
		before, after := samples[index-1], samples[index]
		if before.SessionID != after.SessionID || !after.At.After(before.At) || after.At.Sub(before.At) > MaxSampleGap {
			continue
		}
		left, right := before.At, after.At
		if left.Before(from) {
			left = from
		}
		if right.After(to) {
			right = to
		}
		if right.After(left) {
			if session := values[after.SessionID]; session != nil {
				session.CoveredSeconds += right.Sub(left).Seconds()
			}
		}
	}
	result := make([]SessionPerformance, 0, len(values))
	for _, value := range values {
		result = append(result, *value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].FirstSeen.After(result[j].FirstSeen) })
	return result
}

func fmtInt(value int) string { return strconv.Itoa(value) }

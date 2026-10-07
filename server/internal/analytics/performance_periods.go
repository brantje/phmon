package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

const performancePeriodWindow = 7 * 24 * time.Hour
const maxPerformanceLocationComparisons = 20

type performancePeriodKey struct {
	characterID string
	granularity string
	startDate   string
}

type performanceLocationKey struct {
	characterID string
	region      string
	zone        string
}

func queryPerformancePeriods(ctx context.Context, tx pgx.Tx, filter Filter, snapshot *Snapshot) error {
	characterIDs := append([]string(nil), filter.CharacterIDs...)
	if len(characterIDs) == 0 && filter.CharacterID != "" {
		characterIDs = append(characterIDs, filter.CharacterID)
	}
	if len(characterIDs) == 0 {
		return nil
	}
	performance := make(map[string]*CharacterPerformance, len(characterIDs))
	if len(characterIDs) == 1 && snapshot.Performance != nil {
		performance[characterIDs[0]] = snapshot.Performance
	}
	for id, value := range snapshot.PerformanceBatch {
		performance[id] = value
	}
	for _, id := range characterIDs {
		if performance[id] == nil {
			performance[id] = &CharacterPerformance{CharacterID: id}
		}
		performance[id].Periods = []PerformancePeriod{}
		performance[id].Locations = []PerformanceLocation{}
	}

	start, end := filter.To.Add(-performancePeriodWindow), filter.To
	periods := make(map[performancePeriodKey]*PerformancePeriod)
	locations := make(map[performanceLocationKey]*PerformanceLocation)
	rows, err := tx.Query(ctx, `WITH ordered AS (
    SELECT s.character_id::text,s.session_id,s.sampled_at,s.sample_id,s.level,s.current_exp,s.max_exp,s.sp,s.gold,s.region,s.zone_name,
           lag(s.sampled_at) OVER(PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id) AS before_at,
           lag(s.level) OVER(PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id) AS before_level,
           lag(s.current_exp) OVER(PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id) AS before_exp,
           lag(s.max_exp) OVER(PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id) AS before_max_exp,
           lag(s.sp) OVER(PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id) AS before_sp,
           lag(s.gold) OVER(PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id) AS before_gold,
           lag(s.region) OVER(PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id) AS before_region,
           lag(s.zone_name) OVER(PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id) AS before_zone
    FROM character_metric_samples s
    WHERE s.character_id=ANY($1::uuid[]) AND ($2='' OR s.server_key=$2)
      AND s.sampled_at >= $3::timestamptz-interval '30 seconds' AND s.sampled_at < $4
), base AS (
    SELECT *,date_trunc('day',sampled_at AT TIME ZONE $5)::date AS local_day,
      date_trunc('day',before_at AT TIME ZONE $5)::date AS before_day,
      date_trunc('week',sampled_at AT TIME ZONE $5)::date AS local_week,
      date_trunc('week',before_at AT TIME ZONE $5)::date AS before_week,
      CASE WHEN level IS NOT NULL AND level=before_level AND max_exp IS NOT NULL AND max_exp=before_max_exp
        AND current_exp IS NOT NULL AND before_exp IS NOT NULL AND current_exp>=before_exp
        THEN current_exp::numeric-before_exp::numeric END AS xp_delta,
      CASE WHEN sp IS NOT NULL AND before_sp IS NOT NULL THEN sp::numeric-before_sp::numeric END AS sp_delta,
      CASE WHEN gold IS NOT NULL AND before_gold IS NOT NULL THEN gold::numeric-before_gold::numeric END AS gold_delta,
      EXTRACT(EPOCH FROM sampled_at-before_at)::float8 AS interval_seconds
    FROM ordered
    WHERE before_at IS NOT NULL AND sampled_at>before_at AND sampled_at-before_at<=interval '30 seconds'
      AND sampled_at >= $3 AND sampled_at < $4 AND before_at >= $3
), period_rows AS (
    SELECT character_id,'day'::text AS granularity,local_day AS period_start,false AS has_location,
      NULL::integer AS region,''::text AS zone,sum(xp_delta)::text AS xp_gain,
      sum(interval_seconds) FILTER(WHERE xp_delta IS NOT NULL)::float8 AS xp_seconds,
      sum(sp_delta)::text AS sp_net,sum(interval_seconds) FILTER(WHERE sp_delta IS NOT NULL)::float8 AS sp_seconds,
      sum(gold_delta)::text AS gold_net,sum(interval_seconds) FILTER(WHERE gold_delta IS NOT NULL)::float8 AS gold_seconds,
      sum(interval_seconds)::float8 AS covered_seconds,count(*)::bigint AS sample_intervals
    FROM base WHERE local_day=before_day GROUP BY character_id,local_day
    UNION ALL
    SELECT character_id,'week',local_week,false,NULL::integer,''::text,sum(xp_delta)::text,
      sum(interval_seconds) FILTER(WHERE xp_delta IS NOT NULL)::float8,sum(sp_delta)::text,
      sum(interval_seconds) FILTER(WHERE sp_delta IS NOT NULL)::float8,sum(gold_delta)::text,
      sum(interval_seconds) FILTER(WHERE gold_delta IS NOT NULL)::float8,sum(interval_seconds)::float8,count(*)::bigint
    FROM base WHERE local_week=before_week GROUP BY character_id,local_week
    UNION ALL
    SELECT character_id,'location',NULL::date,true,region,COALESCE(zone_name,''),sum(xp_delta)::text,
      sum(interval_seconds) FILTER(WHERE xp_delta IS NOT NULL)::float8,sum(sp_delta)::text,
      sum(interval_seconds) FILTER(WHERE sp_delta IS NOT NULL)::float8,sum(gold_delta)::text,
      sum(interval_seconds) FILTER(WHERE gold_delta IS NOT NULL)::float8,sum(interval_seconds)::float8,count(*)::bigint
    FROM base WHERE region IS NOT DISTINCT FROM before_region AND zone_name IS NOT DISTINCT FROM before_zone
      AND (region IS NOT NULL OR zone_name IS NOT NULL)
    GROUP BY character_id,region,zone_name
)
SELECT character_id,granularity,period_start::text,has_location,region,zone,xp_gain,xp_seconds,sp_net,sp_seconds,gold_net,gold_seconds,covered_seconds,sample_intervals
FROM period_rows`, characterIDs, filter.Server, start, end, filter.Timezone)
	if err != nil {
		return err
	}
	for rows.Next() {
		var characterID, granularity string
		var startDate *string
		var hasLocation bool
		var region *int
		var zone string
		var xpGain, spNet, goldNet *string
		var xpSeconds, spSeconds, goldSeconds, coveredSeconds *float64
		var intervals int64
		if err := rows.Scan(&characterID, &granularity, &startDate, &hasLocation, &region, &zone, &xpGain, &xpSeconds, &spNet, &spSeconds, &goldNet, &goldSeconds, &coveredSeconds, &intervals); err != nil {
			rows.Close()
			return err
		}
		if hasLocation {
			key := performanceLocationKey{characterID: characterID, region: optionalRegionKey(region), zone: zone}
			locations[key] = &PerformanceLocation{Region: region, Zone: zone, XPGain: xpGain, XPSeconds: valueOrZero(xpSeconds), SPNet: spNet, SPSeconds: valueOrZero(spSeconds), GoldNet: goldNet, GoldSeconds: valueOrZero(goldSeconds), CoveredSeconds: valueOrZero(coveredSeconds), SampleIntervals: intervals}
			continue
		}
		if startDate == nil {
			continue
		}
		key := performancePeriodKey{characterID: characterID, granularity: granularity, startDate: *startDate}
		periods[key] = &PerformancePeriod{Granularity: granularity, StartDate: *startDate, XPGain: xpGain, XPSeconds: valueOrZero(xpSeconds), SPNet: spNet, SPSeconds: valueOrZero(spSeconds), GoldNet: goldNet, GoldSeconds: valueOrZero(goldSeconds), CoveredSeconds: valueOrZero(coveredSeconds), SampleIntervals: intervals}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if err := queryPerformancePeriodEvents(ctx, tx, characterIDs, filter.Server, start, end, filter.Timezone, periods, locations); err != nil {
		return err
	}
	for key, period := range periods {
		if target := performance[key.characterID]; target != nil {
			target.Periods = append(target.Periods, *period)
		}
	}
	for key, location := range locations {
		if target := performance[key.characterID]; target != nil {
			target.Locations = append(target.Locations, *location)
		}
	}
	for _, target := range performance {
		sort.Slice(target.Periods, func(i, j int) bool {
			if target.Periods[i].Granularity != target.Periods[j].Granularity {
				return target.Periods[i].Granularity < target.Periods[j].Granularity
			}
			return target.Periods[i].StartDate < target.Periods[j].StartDate
		})
		sort.Slice(target.Locations, func(i, j int) bool {
			if target.Locations[i].CoveredSeconds != target.Locations[j].CoveredSeconds {
				return target.Locations[i].CoveredSeconds > target.Locations[j].CoveredSeconds
			}
			left, right := optionalRegionKey(target.Locations[i].Region), optionalRegionKey(target.Locations[j].Region)
			if left != right {
				return left < right
			}
			return target.Locations[i].Zone < target.Locations[j].Zone
		})
		if len(target.Locations) > maxPerformanceLocationComparisons {
			target.Locations = target.Locations[:maxPerformanceLocationComparisons]
			target.Truncated = true
		}
	}
	return nil
}

func queryPerformancePeriodEvents(ctx context.Context, tx pgx.Tx, ids []string, server string, from, to time.Time, timezone string, periods map[performancePeriodKey]*PerformancePeriod, locations map[performanceLocationKey]*PerformanceLocation) error {
	rows, err := tx.Query(ctx, `WITH scoped AS (
  SELECT e.character_id::text,e.kind,e.region,COALESCE(e.zone_name,'') AS zone_name,
    date_trunc('day',e.occurred_at AT TIME ZONE $5)::date AS local_day,
    date_trunc('week',e.occurred_at AT TIME ZONE $5)::date AS local_week
  FROM activity_events e
  WHERE e.character_id=ANY($1::uuid[]) AND ($2='' OR lower(e.server_name)=$2)
    AND e.occurred_at >= $3 AND e.occurred_at < $4
), counts AS (
  SELECT character_id,'day'::text AS granularity,local_day AS period_start,false AS has_location,
    NULL::integer AS region,''::text AS zone,
    count(*) FILTER(WHERE kind='character.died') AS deaths,
    count(*) FILTER(WHERE kind='drop.item') AS normal_drops,
    count(*) FILTER(WHERE kind='drop.rare') AS rare_drops
  FROM scoped GROUP BY character_id,local_day
  UNION ALL
  SELECT character_id,'week',local_week,false,NULL::integer,''::text,
    count(*) FILTER(WHERE kind='character.died'),count(*) FILTER(WHERE kind='drop.item'),count(*) FILTER(WHERE kind='drop.rare')
  FROM scoped GROUP BY character_id,local_week
  UNION ALL
  SELECT character_id,'location',NULL::date,true,region,zone_name,
    count(*) FILTER(WHERE kind='character.died'),count(*) FILTER(WHERE kind='drop.item'),count(*) FILTER(WHERE kind='drop.rare')
  FROM scoped GROUP BY character_id,region,zone_name
)
SELECT character_id,granularity,period_start::text,has_location,region,zone, deaths,normal_drops,rare_drops FROM counts`, ids, server, from, to, timezone)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var characterID, granularity string
		var startDate *string
		var hasLocation bool
		var region *int
		var zone string
		var deaths, normalDrops, rareDrops int64
		if err := rows.Scan(&characterID, &granularity, &startDate, &hasLocation, &region, &zone, &deaths, &normalDrops, &rareDrops); err != nil {
			return err
		}
		if hasLocation {
			key := performanceLocationKey{characterID: characterID, region: optionalRegionKey(region), zone: zone}
			location := locations[key]
			if location == nil {
				location = &PerformanceLocation{Region: region, Zone: zone}
				locations[key] = location
			}
			location.Deaths, location.NormalDrops, location.RareDrops = deaths, normalDrops, rareDrops
			continue
		}
		if startDate == nil {
			continue
		}
		key := performancePeriodKey{characterID: characterID, granularity: granularity, startDate: *startDate}
		period := periods[key]
		if period == nil {
			period = &PerformancePeriod{Granularity: granularity, StartDate: *startDate}
			periods[key] = period
		}
		period.Deaths, period.NormalDrops, period.RareDrops = deaths, normalDrops, rareDrops
	}
	return rows.Err()
}

func optionalRegionKey(region *int) string {
	if region == nil {
		return ""
	}
	return fmt.Sprintf("%d", *region)
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

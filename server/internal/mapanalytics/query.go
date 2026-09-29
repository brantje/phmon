package mapanalytics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agentdomain "phmon/server/internal/agents"
)

var (
	ErrInvalidFilter          = errors.New("invalid heatmap filter")
	ErrBroadResetUnconfirmed = errors.New("broad heatmap reset requires explicit confirmation")
)

func validLayer(layer string) bool {
	switch layer {
	case LayerMobDensity, LayerMobObserverAvg, LayerMobTypes, LayerDeaths, LayerDrops, LayerUniqueSightings, LayerPlayerMovement:
		return true
	default:
		return false
	}
}

func layerSupportsMobFilters(layer string) bool {
	return layer == LayerMobDensity || layer == LayerMobObserverAvg || layer == LayerMobTypes
}

func defaultResolution(window time.Duration) float64 {
	switch {
	case window <= time.Hour:
		return 48
	case window <= 24*time.Hour:
		return 96
	case window <= 7*24*time.Hour:
		return 192
	default:
		return 192
	}
}

func NormalizeFilter(filter Filter) (Filter, error) {
	filter.Server = strings.TrimSpace(filter.Server)
	filter.AreaID = strings.TrimSpace(filter.AreaID)
	filter.FloorID = strings.TrimSpace(filter.FloorID)
	filter.CharacterID = strings.TrimSpace(filter.CharacterID)
	filter.MonsterType = strings.TrimSpace(filter.MonsterType)
	if !validLayer(filter.Layer) || filter.Server == "" || len(filter.Server) > 100 || !validDatasetID(filter.DatasetID) ||
		filter.AreaID == "" || len(filter.AreaID) > 96 || filter.FloorID == "" || len(filter.FloorID) > 32 ||
		filter.From.IsZero() || filter.To.IsZero() || !filter.To.After(filter.From) || filter.To.Sub(filter.From) > MaxQueryWindow ||
		(filter.CharacterID != "" && !agentdomain.ValidAgentID(filter.CharacterID)) || len(filter.MonsterType) > 64 ||
		(!layerSupportsMobFilters(filter.Layer) && (filter.MonsterType != "" || filter.ModelID != nil)) ||
		(filter.ModelID != nil && (*filter.ModelID < 0 || *filter.ModelID > 4294967295)) ||
		(filter.Region != nil && !validMapRegion(*filter.Region)) {
		return Filter{}, ErrInvalidFilter
	}
	if filter.Limit == 0 {
		filter.Limit = MaxHeatmapPoints
	}
	if filter.Limit < 1 || filter.Limit > MaxHeatmapPoints {
		return Filter{}, ErrInvalidFilter
	}
	if filter.Resolution == 0 {
		filter.Resolution = defaultResolution(filter.To.Sub(filter.From))
	}
	if filter.Resolution != 48 && filter.Resolution != 96 && filter.Resolution != 192 {
		return Filter{}, ErrInvalidFilter
	}
	return filter, nil
}

func baseResult(filter Filter, status, metric, interpretation string) Result {
	return Result{
		Layer: filter.Layer, Status: status, Metric: metric, Interpretation: interpretation,
		Server: filter.Server, DatasetVersion: filter.DatasetID, AreaID: filter.AreaID, FloorID: filter.FloorID,
		Region: filter.Region, From: filter.From.UTC(), To: filter.To.UTC(), Resolution: filter.Resolution, Points: []Point{},
	}
}

func (s *Store) Heatmap(ctx context.Context, input Filter) (Result, error) {
	filter, err := NormalizeFilter(input)
	if err != nil {
		return Result{}, err
	}
	if filter.AreaID != "world" || filter.FloorID != "world" {
		result := baseResult(filter, StatusUnsupported, "", "Historical coordinates are not plotted until this area and floor have a validated transform.")
		result.Reason = "coordinate_transform_unverified"
		return result, nil
	}
	if filter.Layer == LayerMobDensity {
		result := baseResult(filter, StatusUnsupported, "", "Spatial mob density requires a verified observation-coverage footprint; current phBot snapshots do not establish which surrounding cells were observable.")
		result.Reason = "observation_coverage_unverified"
		return result, nil
	}
	switch filter.Layer {
	case LayerMobObserverAvg:
		return s.observerAverage(ctx, filter)
	case LayerMobTypes:
		return s.mobSightings(ctx, filter)
	case LayerPlayerMovement:
		return s.positionHeatmap(ctx, filter)
	default:
		return s.eventHeatmap(ctx, filter)
	}
}

func (s *Store) eventHeatmap(ctx context.Context, filter Filter) (Result, error) {
	kindSQL := "e.kind='character.died'"
	metric := "death_occurrences"
	interpretation := "Recorded character death occurrences with valid map coordinates."
	switch filter.Layer {
	case LayerDrops:
		kindSQL = "e.kind IN ('drop.item','drop.rare')"
		metric = "drop_occurrences"
		interpretation = "Recorded world drop occurrences with valid map coordinates; owned-item acquisition events are not included."
	case LayerUniqueSightings:
		kindSQL = "e.kind='world.unique_spawned'"
		metric = "unique_spawn_occurrences"
		interpretation = "Recorded unique-spawn callback occurrences with valid map coordinates."
	case LayerDeaths:
	default:
		return Result{}, ErrInvalidFilter
	}
	query := fmt.Sprintf(`WITH source AS (
		SELECT e.character_id,e.occurred_at,e.region,e.x,e.y,
		EXISTS(SELECT 1 FROM map_heatmap_resets r WHERE r.layer=$1 AND lower(r.server_name)=lower($2)
			AND r.dataset_id=$3 AND r.area_id=$4 AND r.floor_id=$5
			AND (r.region IS NULL OR r.region=e.region) AND (r.character_id IS NULL OR r.character_id=e.character_id)
			AND e.occurred_at>=r.from_time AND e.occurred_at<r.to_time) AS suppressed
		FROM activity_events e WHERE lower(e.server_name)=lower($2) AND %s
			AND e.occurred_at >= $6 AND e.occurred_at < $7 AND e.region IS NOT NULL AND e.x IS NOT NULL AND e.y IS NOT NULL
			AND ($8::integer IS NULL OR e.region=$8) AND ($9='' OR e.character_id=$9::uuid)
	), grouped AS (
		SELECT region,floor(x/$10)::bigint bx,floor(y/$10)::bigint by,count(*)::bigint n
		FROM source WHERE NOT suppressed GROUP BY region,bx,by
	), ranked AS (
		SELECT *,count(*) OVER()::bigint total_groups FROM grouped ORDER BY n DESC,region,bx,by LIMIT $11
	)
	SELECT region,(bx+0.5)*$10,(by+0.5)*$10,n,total_groups FROM ranked`, kindSQL)
	rows, err := s.pool.Query(ctx, query, filter.Layer, filter.Server, filter.DatasetID, filter.AreaID, filter.FloorID,
		filter.From.UTC(), filter.To.UTC(), filter.Region, filter.CharacterID, filter.Resolution, filter.Limit)
	if err != nil {
		return Result{}, err
	}
	defer rows.Close()
	result := baseResult(filter, StatusAvailable, metric, interpretation)
	var totalGroups int64
	for rows.Next() {
		var point Point
		if err := rows.Scan(&point.Region, &point.X, &point.Y, &point.Count, &totalGroups); err != nil {
			return Result{}, err
		}
		point.Weight = float64(point.Count)
		result.SourceRows += point.Count
		result.Points = append(result.Points, point)
	}
	if err := rows.Err(); err != nil {
		return Result{}, err
	}
	result.Truncated = totalGroups > int64(filter.Limit)
	countQuery := fmt.Sprintf(`SELECT count(*) FROM activity_events e WHERE lower(e.server_name)=lower($2) AND %s
		AND e.occurred_at >= $6 AND e.occurred_at < $7 AND e.region IS NOT NULL AND e.x IS NOT NULL AND e.y IS NOT NULL
		AND ($8::integer IS NULL OR e.region=$8) AND ($9='' OR e.character_id=$9::uuid)
		AND EXISTS(SELECT 1 FROM map_heatmap_resets r WHERE r.layer=$1 AND lower(r.server_name)=lower($2)
			AND r.dataset_id=$3 AND r.area_id=$4 AND r.floor_id=$5 AND (r.region IS NULL OR r.region=e.region)
			AND (r.character_id IS NULL OR r.character_id=e.character_id) AND e.occurred_at>=r.from_time AND e.occurred_at<r.to_time)`, kindSQL)
	if err := s.pool.QueryRow(ctx, countQuery, filter.Layer, filter.Server, filter.DatasetID, filter.AreaID, filter.FloorID,
		filter.From.UTC(), filter.To.UTC(), filter.Region, filter.CharacterID).Scan(&result.SuppressedRows); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (s *Store) positionHeatmap(ctx context.Context, filter Filter) (Result, error) {
	rows, err := s.pool.Query(ctx, `WITH source AS (
		SELECT p.character_id,p.sampled_at,p.region,p.x,p.y,
		EXISTS(SELECT 1 FROM map_heatmap_resets r WHERE r.layer=$1 AND lower(r.server_name)=lower($2)
			AND r.dataset_id=$3 AND r.area_id=$4 AND r.floor_id=$5 AND (r.region IS NULL OR r.region=p.region)
			AND (r.character_id IS NULL OR r.character_id=p.character_id) AND p.sampled_at>=r.from_time AND p.sampled_at<r.to_time) AS suppressed
		FROM character_position_samples p WHERE lower(p.server_name)=lower($2) AND p.dataset_id=$3
			AND p.sampled_at >= $6 AND p.sampled_at < $7 AND ($8::integer IS NULL OR p.region=$8)
			AND ($9='' OR p.character_id=$9::uuid)
	), grouped AS (
		SELECT region,floor(x/$10)::bigint bx,floor(y/$10)::bigint by,count(*)::bigint n
		FROM source WHERE NOT suppressed GROUP BY region,bx,by
	), ranked AS (
		SELECT *,count(*) OVER()::bigint total_groups FROM grouped ORDER BY n DESC,region,bx,by LIMIT $11
	)
	SELECT region,(bx+0.5)*$10,(by+0.5)*$10,n,total_groups FROM ranked`, filter.Layer, filter.Server, filter.DatasetID,
		filter.AreaID, filter.FloorID, filter.From.UTC(), filter.To.UTC(), filter.Region, filter.CharacterID, filter.Resolution, filter.Limit)
	if err != nil {
		return Result{}, err
	}
	defer rows.Close()
	result := baseResult(filter, StatusAvailable, "movement_sample_count", "Server-sampled historical character positions after stationary/rate suppression.")
	var totalGroups int64
	for rows.Next() {
		var point Point
		if err := rows.Scan(&point.Region, &point.X, &point.Y, &point.Count, &totalGroups); err != nil {
			return Result{}, err
		}
		point.Weight = float64(point.Count)
		result.SourceRows += point.Count
		result.Points = append(result.Points, point)
	}
	if err := rows.Err(); err != nil {
		return Result{}, err
	}
	result.Truncated = totalGroups > int64(filter.Limit)
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM character_position_samples p WHERE lower(p.server_name)=lower($2) AND p.dataset_id=$3
		AND p.sampled_at >= $6 AND p.sampled_at < $7 AND ($8::integer IS NULL OR p.region=$8) AND ($9='' OR p.character_id=$9::uuid)
		AND EXISTS(SELECT 1 FROM map_heatmap_resets r WHERE r.layer=$1 AND lower(r.server_name)=lower($2) AND r.dataset_id=$3
			AND r.area_id=$4 AND r.floor_id=$5 AND (r.region IS NULL OR r.region=p.region) AND (r.character_id IS NULL OR r.character_id=p.character_id)
			AND p.sampled_at>=r.from_time AND p.sampled_at<r.to_time)`, filter.Layer, filter.Server, filter.DatasetID,
		filter.AreaID, filter.FloorID, filter.From.UTC(), filter.To.UTC(), filter.Region, filter.CharacterID).Scan(&result.SuppressedRows); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (s *Store) mobSightings(ctx context.Context, filter Filter) (Result, error) {
	rows, err := s.pool.Query(ctx, `WITH source AS (
		SELECT s.character_id,s.sampled_at,o.region,o.x,o.y,o.monster_type,o.model_id,
		EXISTS(SELECT 1 FROM map_heatmap_resets r WHERE r.layer=$1 AND lower(r.server_name)=lower($2)
			AND r.dataset_id=$3 AND r.area_id=$4 AND r.floor_id=$5 AND (r.region IS NULL OR r.region=o.region)
			AND (r.character_id IS NULL OR r.character_id=s.character_id)
			AND (r.monster_type IS NULL OR r.monster_type=o.monster_type) AND (r.model_id IS NULL OR r.model_id=o.model_id)
			AND s.sampled_at>=r.from_time AND s.sampled_at<r.to_time) AS suppressed
		FROM mob_observation_samples s JOIN mob_observations o ON o.sample_id=s.sample_id
		WHERE lower(s.server_name)=lower($2) AND s.dataset_id=$3 AND s.sampled_at >= $6 AND s.sampled_at < $7
			AND ($8::integer IS NULL OR o.region=$8) AND ($9='' OR s.character_id=$9::uuid)
			AND ($10='' OR o.monster_type=$10) AND ($11::bigint IS NULL OR o.model_id=$11)
	), grouped AS (
		SELECT region,floor(x/$12)::bigint bx,floor(y/$12)::bigint by,count(*)::bigint n
		FROM source WHERE NOT suppressed GROUP BY region,bx,by
	), ranked AS (
		SELECT *,count(*) OVER()::bigint total_groups FROM grouped ORDER BY n DESC,region,bx,by LIMIT $13
	)
	SELECT region,(bx+0.5)*$12,(by+0.5)*$12,n,total_groups FROM ranked`, filter.Layer, filter.Server, filter.DatasetID,
		filter.AreaID, filter.FloorID, filter.From.UTC(), filter.To.UTC(), filter.Region, filter.CharacterID,
		filter.MonsterType, filter.ModelID, filter.Resolution, filter.Limit)
	if err != nil {
		return Result{}, err
	}
	defer rows.Close()
	result := baseResult(filter, StatusAvailable, "monster_sighting_count", "Historical monster sightings at observed monster coordinates; this layer is not observation-normalized density.")
	var totalGroups int64
	for rows.Next() {
		var point Point
		if err := rows.Scan(&point.Region, &point.X, &point.Y, &point.Count, &totalGroups); err != nil {
			return Result{}, err
		}
		point.Weight = float64(point.Count)
		result.SourceRows += point.Count
		result.Points = append(result.Points, point)
	}
	if err := rows.Err(); err != nil {
		return Result{}, err
	}
	result.Truncated = totalGroups > int64(filter.Limit)
	return result, nil
}

func (s *Store) observerAverage(ctx context.Context, filter Filter) (Result, error) {
	rows, err := s.pool.Query(ctx, `WITH samples AS (
		SELECT s.* FROM mob_observation_samples s WHERE lower(s.server_name)=lower($2) AND s.dataset_id=$3
			AND s.sampled_at >= $6 AND s.sampled_at < $7 AND ($8::integer IS NULL OR s.region=$8)
			AND ($9='' OR s.character_id=$9::uuid)
			AND NOT EXISTS(SELECT 1 FROM map_heatmap_resets r WHERE r.layer=$1 AND lower(r.server_name)=lower($2)
				AND r.dataset_id=$3 AND r.area_id=$4 AND r.floor_id=$5 AND r.monster_type IS NULL AND r.model_id IS NULL
				AND (r.region IS NULL OR r.region=s.region) AND (r.character_id IS NULL OR r.character_id=s.character_id)
				AND s.sampled_at>=r.from_time AND s.sampled_at<r.to_time)
	), grouped AS (
		SELECT s.region,s.observer_cell_x,s.observer_cell_y,count(DISTINCT s.sample_id)::bigint denominator,
			count(o.ordinal) FILTER (WHERE NOT EXISTS(SELECT 1 FROM map_heatmap_resets r WHERE r.layer=$1 AND lower(r.server_name)=lower($2)
				AND r.dataset_id=$3 AND r.area_id=$4 AND r.floor_id=$5 AND (r.region IS NULL OR r.region=o.region)
				AND (r.character_id IS NULL OR r.character_id=s.character_id) AND (r.monster_type IS NULL OR r.monster_type=o.monster_type)
				AND (r.model_id IS NULL OR r.model_id=o.model_id) AND s.sampled_at>=r.from_time AND s.sampled_at<r.to_time))::bigint numerator
		FROM samples s LEFT JOIN mob_observations o ON o.sample_id=s.sample_id
			AND ($10='' OR o.monster_type=$10) AND ($11::bigint IS NULL OR o.model_id=$11)
		GROUP BY s.region,s.observer_cell_x,s.observer_cell_y
	), ranked AS (
		SELECT *,count(*) OVER()::bigint total_groups FROM grouped
		ORDER BY CASE WHEN denominator=0 THEN 0 ELSE numerator::double precision/denominator END DESC,region,observer_cell_x,observer_cell_y LIMIT $12
	)
	SELECT region,(observer_cell_x+0.5)*$13,(observer_cell_y+0.5)*$13,numerator,denominator,
		CASE WHEN denominator=0 THEN 0 ELSE numerator::double precision/denominator END,total_groups FROM ranked`,
		filter.Layer, filter.Server, filter.DatasetID, filter.AreaID, filter.FloorID, filter.From.UTC(), filter.To.UTC(), filter.Region,
		filter.CharacterID, filter.MonsterType, filter.ModelID, filter.Limit, float64(192))
	if err != nil {
		return Result{}, err
	}
	defer rows.Close()
	filter.Resolution = 192
	result := baseResult(filter, StatusLimited, "observer_local_average_count", "Monster rows returned by eligible complete snapshots divided by complete observer samples centered in each observer cell; observed-area coverage is unverified, so this is not spatial mob density.")
	var totalGroups int64
	for rows.Next() {
		var point Point
		if err := rows.Scan(&point.Region, &point.X, &point.Y, &point.Numerator, &point.Denominator, &point.Weight, &totalGroups); err != nil {
			return Result{}, err
		}
		point.Count = point.Numerator
		result.SourceRows += point.Numerator
		result.Points = append(result.Points, point)
	}
	if err := rows.Err(); err != nil {
		return Result{}, err
	}
	result.Truncated = totalGroups > int64(filter.Limit)
	return result, nil
}

func (s *Store) MobFacets(ctx context.Context, input Filter, limit int) ([]MobFacet, error) {
	filter, err := NormalizeFilter(input)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT o.monster_type,o.model_id,count(*)::bigint
		FROM mob_observation_samples s JOIN mob_observations o ON o.sample_id=s.sample_id
		WHERE lower(s.server_name)=lower($1) AND s.dataset_id=$2 AND s.sampled_at >= $3 AND s.sampled_at < $4
			AND ($5::integer IS NULL OR o.region=$5) AND ($6='' OR s.character_id=$6::uuid)
		GROUP BY o.monster_type,o.model_id ORDER BY count(*) DESC,o.monster_type,o.model_id LIMIT $7`,
		filter.Server, filter.DatasetID, filter.From.UTC(), filter.To.UTC(), filter.Region, filter.CharacterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MobFacet, 0)
	for rows.Next() {
		var facet MobFacet
		if err := rows.Scan(&facet.MonsterType, &facet.ModelID, &facet.Count); err != nil {
			return nil, err
		}
		out = append(out, facet)
	}
	return out, rows.Err()
}

func (s *Store) Reset(ctx context.Context, input ResetScope) (ResetScope, error) {
	filter, err := NormalizeFilter(Filter{
		Layer: input.Layer, Server: input.Server, DatasetID: input.DatasetID, AreaID: input.AreaID,
		FloorID: input.FloorID, Region: input.Region, CharacterID: input.CharacterID, MonsterType: input.MonsterType,
		ModelID: input.ModelID, From: input.From, To: input.To, Limit: 1,
	})
	if err != nil || filter.Layer == LayerMobDensity {
		return ResetScope{}, ErrInvalidFilter
	}
	broad := filter.Region == nil && filter.CharacterID == ""
	if broad && !input.ConfirmBroad {
		return ResetScope{}, ErrBroadResetUnconfirmed
	}
	var reset ResetScope
	err = s.pool.QueryRow(ctx, `INSERT INTO map_heatmap_resets
		(layer,server_name,dataset_id,area_id,floor_id,region,character_id,monster_type,model_id,from_time,to_time,broad_scope)
		VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid,NULLIF($8,''),$9,$10,$11,$12)
		RETURNING reset_id::text,created_at`, filter.Layer, filter.Server, filter.DatasetID, filter.AreaID, filter.FloorID,
		filter.Region, filter.CharacterID, filter.MonsterType, filter.ModelID, filter.From.UTC(), filter.To.UTC(), broad).
		Scan(&reset.ResetID, &reset.CreatedAt)
	if err != nil {
		return ResetScope{}, err
	}
	reset.Layer, reset.Server, reset.DatasetID, reset.AreaID, reset.FloorID = filter.Layer, filter.Server, filter.DatasetID, filter.AreaID, filter.FloorID
	reset.Region, reset.CharacterID, reset.MonsterType, reset.ModelID = filter.Region, filter.CharacterID, filter.MonsterType, filter.ModelID
	reset.From, reset.To, reset.BroadScope = filter.From.UTC(), filter.To.UTC(), broad
	return reset, nil
}

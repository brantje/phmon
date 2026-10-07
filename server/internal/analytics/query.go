package analytics

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const maxBreakdownRows = 20

func (s *Store) Query(ctx context.Context, input Filter) (Snapshot, error) {
	if s == nil || s.pool == nil {
		return Snapshot{}, errors.New("analytics store unavailable")
	}
	filter, err := NormalizeFilter(input)
	if err != nil {
		return Snapshot{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout='2500ms'`); err != nil {
		return Snapshot{}, err
	}
	var asOf time.Time
	if err := tx.QueryRow(ctx, `SELECT transaction_timestamp()`).Scan(&asOf); err != nil {
		return Snapshot{}, err
	}
	if filter.CharacterID != "" {
		var serverKey string
		if err := tx.QueryRow(ctx, `SELECT server_key FROM characters WHERE character_id=$1::uuid`, filter.CharacterID).Scan(&serverKey); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Snapshot{}, ErrCharacterScope
			}
			return Snapshot{}, err
		}
		if filter.Server != "" && serverKey != filter.Server {
			return Snapshot{}, ErrCharacterScope
		}
	}
	if len(filter.CharacterIDs) > 0 {
		var matched int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM characters WHERE character_id=ANY($1::uuid[]) AND ($2='' OR server_key=$2)`, filter.CharacterIDs, filter.Server).Scan(&matched); err != nil {
			return Snapshot{}, err
		}
		if matched != len(filter.CharacterIDs) {
			return Snapshot{}, ErrCharacterScope
		}
	}
	snapshot := Snapshot{
		Filter: filter, CalculationVersion: CalculationVersion, AsOf: asOf.UTC(), Status: "available",
		Coverage: Coverage{Status: "limited", Reason: "event totals include recorded occurrences only; monitoring coverage is not continuous"},
		Summary:  []Metric{}, TimeSeries: []Point{}, Breakdown: []Point{}, Occurrences: []Occurrence{},
		Taxonomy: []Point{}, TaxonomyOptions: []TaxonomyOption{}, TaxonomyComplete: true,
	}
	if err := s.sampleCoverage(ctx, tx, filter, &snapshot.Coverage); err != nil {
		return Snapshot{}, err
	}
	switch filter.View {
	case ViewDeaths, ViewRareDrops, ViewNormalDrops, ViewAcademy, ViewAlchemy:
		if err := s.queryEventView(ctx, tx, filter, &snapshot); err != nil {
			return Snapshot{}, err
		}
	case ViewEconomy:
		if err := queryEconomy(ctx, tx, filter, &snapshot); err != nil {
			return Snapshot{}, err
		}
	case ViewPerformance:
		var err error
		if len(filter.CharacterIDs) > 0 {
			err = queryPerformanceBatch(ctx, tx, filter, &snapshot)
		} else {
			err = queryPerformance(ctx, tx, filter, &snapshot)
		}
		if err == nil {
			err = queryPerformancePeriods(ctx, tx, filter, &snapshot)
		}
		if err != nil {
			return Snapshot{}, err
		}
	default:
		return Snapshot{}, ErrInvalidFilter
	}
	s.enrichOccurrences(snapshot.Occurrences)
	if err := tx.Commit(ctx); err != nil {
		return Snapshot{}, err
	}
	snapshot.TimeSeries, snapshot.Truncated = boundTimeSeries(snapshot.TimeSeries, 500, snapshot.Truncated)
	return snapshot, nil
}

// boundTimeSeries keeps the most recent points independently for each series.
// A single global limit can remove the end of the requested range as soon as
// more than one character or guild is present.
func boundTimeSeries(points []Point, perSeries int, alreadyTruncated bool) ([]Point, bool) {
	if perSeries < 1 {
		return []Point{}, true
	}
	counts := make(map[string]int)
	for _, point := range points {
		counts[point.Series]++
	}
	kept := make([]Point, 0, len(points))
	seen := make(map[string]int, len(counts))
	truncated := alreadyTruncated
	for _, point := range points {
		series := point.Series
		if counts[series] > perSeries {
			truncated = true
			if seen[series] < counts[series]-perSeries {
				seen[series]++
				continue
			}
		}
		kept = append(kept, point)
	}
	return kept, truncated
}

func (s *Store) sampleCoverage(ctx context.Context, tx pgx.Tx, filter Filter, coverage *Coverage) error {
	var oldest, newest *time.Time
	var count int64
	var err error
	if filter.View == ViewEconomy && filter.BalanceScope == "guild_storage" {
		err = tx.QueryRow(ctx, `SELECT count(*),min(sampled_at),max(sampled_at) FROM guild_gold_samples
WHERE ($1='' OR server_key=$1) AND ($2='' OR guild_key=$2) AND ($3='' OR observer_character_id=$3::uuid)`,
			filter.Server, filter.Guild, filter.CharacterID).Scan(&count, &oldest, &newest)
	} else {
		err = tx.QueryRow(ctx, `SELECT count(*),min(s.sampled_at),max(s.sampled_at) FROM character_metric_samples s
WHERE ($1='' OR s.server_key=$1) AND ($2='' OR s.character_id=$2::uuid)
AND ($3='' OR EXISTS(SELECT 1 FROM character_group_members m WHERE m.character_id=s.character_id AND m.group_id=$3::uuid))`,
			filter.Server, filter.CharacterID, filter.GroupID).Scan(&count, &oldest, &newest)
	}
	if err != nil {
		return err
	}
	coverage.SampleCount = count
	coverage.RetentionDays = int(s.retentionDays.Load())
	if count == 0 {
		coverage.Status = "unavailable"
		coverage.Reason = "no numeric samples match the selected server/character/group scope"
	}
	if oldest != nil {
		value := oldest.UTC()
		coverage.OldestSample = &value
	}
	if newest != nil {
		value := newest.UTC()
		coverage.NewestSample = &value
	}
	return nil
}

func eventKinds(view View, source string) string {
	switch view {
	case ViewDeaths:
		return "e.kind='character.died'"
	case ViewRareDrops:
		if source == "owned_gains" {
			return `e.kind IN ('item.acquired','item.quantity_increased') AND ((e.source='phbot.state_diff' AND e.source_ref='item_container' AND e.payload->'destination_container'->>'type' IN ('inventory','pets')) OR (e.kind='item.acquired' AND e.source='joymax.pet_inventory')) AND e.item_drop_class='rare'`
		}
		return "e.kind='drop.rare'"
	case ViewNormalDrops:
		if source == "owned_gains" {
			return `e.kind IN ('item.acquired','item.quantity_increased') AND ((e.source='phbot.state_diff' AND e.source_ref='item_container' AND e.payload->'destination_container'->>'type' IN ('inventory','pets')) OR (e.kind='item.acquired' AND e.source='joymax.pet_inventory')) AND e.item_drop_class='normal'`
		}
		return "e.kind='drop.item'"
	case ViewAcademy:
		return "e.kind IN ('academy.member_joined','academy.member_left')"
	case ViewAlchemy:
		return "e.kind='alchemy.attempt'"
	default:
		return "false"
	}
}

func eventScope() string {
	return `lower(e.server_name)=CASE WHEN $1='' THEN lower(e.server_name) ELSE $1 END
 AND e.occurred_at >= $4 AND e.occurred_at < $5
 AND ($2='' OR e.character_id=$2::uuid)
 AND ($3='' OR EXISTS (SELECT 1 FROM character_group_members scope_member WHERE scope_member.character_id=e.character_id AND scope_member.group_id=$3::uuid))
 AND ($6='' OR EXISTS (SELECT 1 FROM characters scope_character WHERE scope_character.character_id=e.character_id AND scope_character.character_name ILIKE '%'||$6||'%'))
 AND ($7='' OR coalesce(e.item_code,'') ILIKE '%'||$7||'%' OR coalesce(e.payload->>'item_name',e.payload->>'name','') ILIKE '%'||$7||'%')`
}

func (s *Store) queryEventView(ctx context.Context, tx pgx.Tx, filter Filter, snapshot *Snapshot) error {
	where := eventScope() + " AND " + eventKinds(filter.View, filter.DropSource)
	args := []any{filter.Server, filter.CharacterID, filter.GroupID, filter.From, filter.To, filter.CharacterQuery, filter.ItemQuery}
	if filter.Server != "" && s.taxonomyOptions != nil {
		snapshot.TaxonomyOptions = s.taxonomyOptions(filter.Server)
	}
	if filter.ItemType != "" || filter.ItemDegree != "" {
		if filter.Server == "" || s.itemModels == nil {
			snapshot.Status, snapshot.Reason = "unsupported", "select a server with a loaded item profile to filter by type or degree"
			return nil
		}
		models, ok := s.itemModels(filter.Server, filter.ItemType, filter.ItemDegree)
		if !ok {
			snapshot.Status, snapshot.Reason = "unsupported", "the selected server has no versioned item taxonomy profile"
			return nil
		}
		where += fmt.Sprintf(" AND e.item_model=ANY($%d::bigint[])", len(args)+1)
		args = append(args, models)
	}
	if filter.View == ViewDeaths {
		total, err := queryTimeline(ctx, tx, filter, where, args, "Deaths", &snapshot.TimeSeries)
		if err != nil {
			return err
		}
		snapshot.Total = strconv.FormatInt(total, 10)
		return queryDeaths(ctx, tx, filter, where, args, total, snapshot)
	}
	var total int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM activity_events e WHERE `+where, args...).Scan(&total); err != nil {
		return err
	}
	snapshot.Total = strconv.FormatInt(total, 10)
	switch filter.View {
	case ViewDeaths:
		return queryDeaths(ctx, tx, filter, where, args, total, snapshot)
	case ViewRareDrops, ViewNormalDrops:
		return s.queryDrops(ctx, tx, filter, where, args, total, snapshot)
	case ViewAcademy:
		return queryAcademy(ctx, tx, filter, where, args, total, snapshot)
	case ViewAlchemy:
		return queryAlchemy(ctx, tx, filter, where, args, total, snapshot)
	}
	return nil
}

func queryDeaths(ctx context.Context, tx pgx.Tx, filter Filter, where string, args []any, total int64, snapshot *Snapshot) error {
	calendarDays, err := localCalendarDays(filter.From, filter.To, filter.Timezone)
	if err != nil {
		return err
	}
	var population int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM characters c
WHERE ($1='' OR c.server_key=$1) AND ($2='' OR c.character_id=$2::uuid)
AND ($3='' OR EXISTS(SELECT 1 FROM character_group_members m WHERE m.character_id=c.character_id AND m.group_id=$3::uuid))`, filter.Server, filter.CharacterID, filter.GroupID).Scan(&population); err != nil {
		return err
	}
	average := 0.0
	if population > 0 && calendarDays > 0 {
		average = float64(total) / float64(population) / float64(calendarDays)
	}
	snapshot.Summary = append(snapshot.Summary,
		Metric{Key: "total_deaths", Label: "Total recorded deaths", Value: strconv.FormatInt(total, 10), Number: float64Ptr(float64(total)), Unit: "events", Status: "available"},
		Metric{Key: "average_deaths", Label: "Average deaths / registered character / day", Number: float64Ptr(average), Unit: "deaths / registered character / local day", Status: metricStatus(population > 0, "empty_population"), Reason: metricReason(population > 0, "no_registered_characters")},
	)
	charRows, err := queryBreakdown(ctx, tx, filter, where, args, "character")
	if err != nil {
		return err
	}
	locationRows, err := queryBreakdown(ctx, tx, filter, where, args, "location")
	if err != nil {
		return err
	}
	switch filter.GroupBy {
	case "group":
		snapshot.Breakdown, err = queryBreakdown(ctx, tx, filter, where, args, "group")
	case "location":
		snapshot.Breakdown = locationRows
	default:
		snapshot.Breakdown = charRows
	}
	if err != nil {
		return err
	}
	snapshot.Summary = append(snapshot.Summary,
		leaderMetric("most_deaths_character", "Character with most recorded deaths", charRows, eventEvidencePrefix(filter, "character.died", false)),
		leaderMetric("most_deaths_location", "Location with most recorded deaths", locationRows, ""),
	)
	return queryOccurrences(ctx, tx, filter, where, args, &snapshot.Occurrences, &snapshot.NextCursor)
}

func (s *Store) queryDrops(ctx context.Context, tx pgx.Tx, filter Filter, where string, args []any, total int64, snapshot *Snapshot) error {
	kind := "Rare drops"
	if filter.View == ViewNormalDrops {
		kind = "Normal drops"
	}
	if filter.DropSource == "owned_gains" {
		kind = strings.TrimSuffix(kind, " drops") + " owned gains"
	}
	if _, err := queryTimeline(ctx, tx, filter, where, args, kind, &snapshot.TimeSeries); err != nil {
		return err
	}
	chars, err := queryBreakdown(ctx, tx, filter, where, args, "character")
	if err != nil {
		return err
	}
	items, err := queryBreakdown(ctx, tx, filter, where, args, "item")
	if err != nil {
		return err
	}
	taxonomyType, taxonomyDegree, taxonomyKnown, taxonomyUnknown, taxonomyDegreeUnknown, complete, err := s.queryDropTaxonomy(ctx, tx, filter, where, args, total, snapshot)
	if err != nil {
		return err
	}
	snapshot.TaxonomyKnown, snapshot.TaxonomyUnknown, snapshot.TaxonomyDegreeUnknown, snapshot.TaxonomyComplete = taxonomyKnown, taxonomyUnknown, taxonomyDegreeUnknown, complete
	if filter.GroupBy == "item" {
		snapshot.Breakdown = items
	} else if filter.GroupBy == "type" {
		snapshot.Breakdown = taxonomyType
	} else if filter.GroupBy == "degree" {
		snapshot.Breakdown = taxonomyDegree
	} else if filter.GroupBy == "location" {
		snapshot.Breakdown, err = queryBreakdown(ctx, tx, filter, where, args, "location")
	} else {
		snapshot.Breakdown = chars
	}
	if err != nil {
		return err
	}
	if filter.GroupBy == "type" && taxonomyUnknown > 0 {
		snapshot.Breakdown = append(snapshot.Breakdown, Point{Label: "Unknown", Value: strconv.FormatInt(taxonomyUnknown, 10), Series: "unknown taxonomy"})
	}
	unknownTaxonomyValue, unknownTaxonomyStatus, unknownTaxonomyReason := strconv.FormatInt(taxonomyUnknown, 10), "available", ""
	degreeUnknownStatus, degreeUnknownReason := "available", ""
	if !complete {
		unknownTaxonomyValue, unknownTaxonomyStatus, unknownTaxonomyReason = "incomplete", "limited", "item taxonomy coverage is incomplete"
		degreeUnknownStatus, degreeUnknownReason = "limited", "item taxonomy coverage is incomplete"
	}
	snapshot.Summary = append(snapshot.Summary,
		Metric{Key: "drop_total", Label: dropPopulationLabel(filter.DropSource), Value: strconv.FormatInt(total, 10), Number: float64Ptr(float64(total)), Unit: dropPopulationUnit(filter.DropSource), Status: "available"},
		leaderMetric("leading_drop_character", "Character with most drop observations", chars, dropLeaderHref(filter)),
		Metric{Key: "item_taxonomy_known", Label: "Classified item observations", Value: strconv.FormatInt(taxonomyKnown, 10), Unit: "observations", Status: taxonomyStatus(complete), Reason: taxonomyReason(complete)},
		Metric{Key: "item_taxonomy_unknown", Label: "Unknown item taxonomy", Value: unknownTaxonomyValue, Unit: "observations", Status: unknownTaxonomyStatus, Reason: unknownTaxonomyReason},
		Metric{Key: "item_degree_unknown", Label: "Unknown item degree", Value: strconv.FormatInt(taxonomyDegreeUnknown, 10), Unit: "observations", Status: degreeUnknownStatus, Reason: degreeUnknownReason},
	)
	knownTypeIndex := 0
	for _, point := range taxonomyType {
		if point.Label == "Other" || point.Label == "Unknown" || taxonomyKnown == 0 || knownTypeIndex == 5 {
			continue
		}
		count, parseErr := strconv.ParseInt(point.Value, 10, 64)
		if parseErr != nil {
			continue
		}
		share := float64(count) / float64(taxonomyKnown) * 100
		snapshot.Summary = append(snapshot.Summary, Metric{
			Key: fmt.Sprintf("item_type_share_%d", knownTypeIndex+1), Label: "Known type share · " + point.Label,
			Value: fmt.Sprintf("%d / %d (%.1f%%)", count, taxonomyKnown, share), Number: float64Ptr(share),
			Unit: "% of classified observations", Status: taxonomyStatus(complete), Reason: taxonomyReason(complete),
		})
		knownTypeIndex++
	}
	if filter.DropSource == "owned_gains" {
		var quantity int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(CASE WHEN e.payload->>'quantity_delta' ~ '^[0-9]{1,9}$' THEN (e.payload->>'quantity_delta')::bigint ELSE 1 END),0) FROM activity_events e WHERE `+where, args...).Scan(&quantity); err != nil {
			return err
		}
		snapshot.Summary = append(snapshot.Summary, Metric{Key: "owned_gain_quantity", Label: "Observed item quantity gained", Value: strconv.FormatInt(quantity, 10), Unit: "items", Status: "available", Reason: "sum of accepted quantity deltas; destination transfers are excluded"})
	}
	if !complete {
		snapshot.TaxonomyComplete = false
		snapshot.Status = "limited"
		snapshot.Reason = "item taxonomy is unavailable or truncated; occurrence totals remain exact"
	}
	return queryOccurrences(ctx, tx, filter, where, args, &snapshot.Occurrences, &snapshot.NextCursor)
}

func (s *Store) queryDropTaxonomy(ctx context.Context, tx pgx.Tx, filter Filter, where string, args []any, total int64, snapshot *Snapshot) (types, degrees []Point, known, unknown, degreeUnknown int64, complete bool, err error) {
	complete = true
	if s.itemTaxonomy == nil {
		unknownType := Point{Label: "Unknown", Value: strconv.FormatInt(total, 10), Series: "unknown taxonomy"}
		unknownDegree := Point{Label: "Unknown", Value: strconv.FormatInt(total, 10), Series: "unknown item degree"}
		snapshot.Taxonomy = []Point{unknownType}
		if filter.GroupBy == "degree" {
			snapshot.Taxonomy = []Point{unknownDegree}
		}
		return nil, []Point{unknownDegree}, 0, total, total, false, nil
	}
	if filter.Server != "" && s.itemModels != nil {
		if _, ok := s.itemModels(filter.Server, "", ""); !ok {
			complete = false
		} else if s.taxonomyOptions != nil {
			snapshot.TaxonomyOptions = s.taxonomyOptions(filter.Server)
		}
	}
	rows, err := tx.Query(ctx, `SELECT lower(e.server_name),e.item_model,COALESCE(e.item_code,''),count(*)
FROM activity_events e WHERE `+where+` AND e.item_model IS NOT NULL
GROUP BY lower(e.server_name),e.item_model,COALESCE(e.item_code,'') ORDER BY count(*) DESC,e.item_model LIMIT 10001`, args...)
	if err != nil {
		return nil, nil, 0, 0, 0, false, err
	}
	type counts struct {
		byType   map[string]int64
		byDegree map[string]int64
	}
	grouped := counts{byType: map[string]int64{}, byDegree: map[string]int64{}}
	processedGroups := 0
	for rows.Next() {
		var server, code string
		var model int64
		var amount int64
		if err := rows.Scan(&server, &model, &code, &amount); err != nil {
			rows.Close()
			return nil, nil, 0, 0, 0, false, err
		}
		processedGroups++
		if processedGroups > 10000 {
			complete = false
			break
		}
		itemType, degree, ok := s.itemTaxonomy(server, model, code)
		if !ok || itemType == "" {
			continue
		}
		known += amount
		grouped.byType[itemType] += amount
		if degree != "" {
			grouped.byDegree[degree] += amount
		} else {
			degreeUnknown += amount
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, 0, 0, 0, false, err
	}
	rows.Close()
	if complete {
		unknown = total - known
	}
	// Degree reconciliation must include observations whose item type or entire
	// taxonomy is unknown. They remain a distinct Unknown bucket, never degree 0.
	degreeUnknown += unknown
	types = taxonomyPoints(grouped.byType, "item type")
	degrees = taxonomyPoints(grouped.byDegree, "item degree")
	if degreeUnknown > 0 {
		degrees = append(degrees, Point{Label: "Unknown", Value: strconv.FormatInt(degreeUnknown, 10), Series: "unknown item degree"})
	}
	snapshot.Taxonomy = types
	if filter.GroupBy == "degree" {
		snapshot.Taxonomy = degrees
	}
	if unknown > 0 {
		snapshot.Taxonomy = append(snapshot.Taxonomy, Point{Label: "Unknown", Value: strconv.FormatInt(unknown, 10), Series: "unknown taxonomy"})
	}
	if complete && filter.ItemType == "" && filter.ItemDegree == "" {
		snapshot.TaxonomyComplete = true
	}
	return types, degrees, known, unknown, degreeUnknown, complete, nil
}

func taxonomyPoints(counts map[string]int64, series string) []Point {
	points := make([]Point, 0, len(counts))
	for label, count := range counts {
		points = append(points, Point{Label: label, Value: strconv.FormatInt(count, 10), Series: series})
	}
	sort.Slice(points, func(i, j int) bool {
		left, _ := strconv.ParseInt(points[i].Value, 10, 64)
		right, _ := strconv.ParseInt(points[j].Value, 10, 64)
		if left == right {
			return points[i].Label < points[j].Label
		}
		return left > right
	})
	if len(points) > maxBreakdownRows {
		var other int64
		for _, point := range points[maxBreakdownRows:] {
			value, _ := strconv.ParseInt(point.Value, 10, 64)
			other += value
		}
		points = append(points[:maxBreakdownRows], Point{Label: "Other", Value: strconv.FormatInt(other, 10), Series: series})
	}
	return points
}

func taxonomyStatus(complete bool) string {
	if complete {
		return "available"
	}
	return "limited"
}
func taxonomyReason(complete bool) string {
	if complete {
		return ""
	}
	return "profile_missing_or_group_limit_reached"
}

func queryAcademy(ctx context.Context, tx pgx.Tx, filter Filter, where string, args []any, total int64, snapshot *Snapshot) error {
	var joins, leaves int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE e.kind='academy.member_joined'), count(*) FILTER (WHERE e.kind='academy.member_left') FROM activity_events e WHERE `+where, args...).Scan(&joins, &leaves); err != nil {
		return err
	}
	if _, err := queryTimeline(ctx, tx, filter, where, args, "Observed membership changes", &snapshot.TimeSeries); err != nil {
		return err
	}
	academyBreakdown, err := queryBreakdown(ctx, tx, filter, where, args, "academy")
	if err != nil {
		return err
	}
	var observedAcademies, contextless int64
	if err := tx.QueryRow(ctx, `SELECT count(DISTINCT e.payload->>'academy_id') FILTER(WHERE e.payload->>'academy_id' ~ '^[0-9]{1,19}$'),
count(*) FILTER(WHERE e.payload->>'academy_id' IS NULL OR e.payload->>'academy_id' !~ '^[0-9]{1,19}$') FROM activity_events e WHERE `+where, args...).Scan(&observedAcademies, &contextless); err != nil {
		return err
	}
	if filter.GroupBy == "academy" {
		snapshot.Breakdown = academyBreakdown
	} else {
		snapshot.Breakdown, err = queryBreakdown(ctx, tx, filter, where, args, "character")
		if err != nil {
			return err
		}
	}
	snapshot.Summary = append(snapshot.Summary,
		Metric{Key: "membership_changes", Label: "Observed membership changes", Value: strconv.FormatInt(total, 10), Number: float64Ptr(float64(total)), Unit: "changes", Status: "available"},
		Metric{Key: "members_joined", Label: "Observed joins", Value: strconv.FormatInt(joins, 10), Number: float64Ptr(float64(joins)), Unit: "changes", Status: "available"},
		Metric{Key: "members_left", Label: "Observed departures", Value: strconv.FormatInt(leaves, 10), Number: float64Ptr(float64(leaves)), Unit: "changes", Status: "available"},
		Metric{Key: "observed_academies", Label: "Academy IDs with recorded changes", Value: strconv.FormatInt(observedAcademies, 10), Unit: "academy IDs", Status: metricStatus(observedAcademies > 0, "no_academy_ids"), Reason: metricReason(observedAcademies > 0, "academy_id_context_not_present_in_legacy_records")},
		Metric{Key: "contextless_membership", Label: "Membership changes without academy ID", Value: strconv.FormatInt(contextless, 10), Unit: "changes", Status: "limited", Reason: "legacy records and unavailable academy context remain unassigned"},
		Metric{Key: "graduations", Label: "Graduations and graduate duration", Status: "unsupported", Reason: "the available membership source does not verify graduation or exact join time"},
	)
	return queryOccurrences(ctx, tx, filter, where, args, &snapshot.Occurrences, &snapshot.NextCursor)
}

func queryAlchemy(ctx context.Context, tx pgx.Tx, filter Filter, where string, args []any, total int64, snapshot *Snapshot) error {
	var successes, failures, unknown int64
	var highest *int
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE e.payload->>'success'='true'), count(*) FILTER(WHERE e.payload->>'success'='false'), count(*) FILTER(WHERE e.payload->>'success' IS NULL OR e.payload->>'success' NOT IN ('true','false')), max(CASE WHEN e.payload->>'plus' ~ '^[0-9]{1,3}$' THEN (e.payload->>'plus')::integer END) FROM activity_events e WHERE `+where, args...).Scan(&successes, &failures, &unknown, &highest); err != nil {
		return err
	}
	if err := queryAlchemyWeekdays(ctx, tx, filter, where, args, &snapshot.TimeSeries); err != nil {
		return err
	}
	chars, err := queryBreakdown(ctx, tx, filter, where, args, "character")
	if err != nil {
		return err
	}
	snapshot.Breakdown = chars
	successRate := 0.0
	known := successes + failures
	if known > 0 {
		successRate = float64(successes) / float64(known) * 100
	}
	snapshot.Summary = append(snapshot.Summary,
		Metric{Key: "alchemy_attempts", Label: "Recorded attempts", Value: strconv.FormatInt(total, 10), Number: float64Ptr(float64(total)), Unit: "attempts", Status: "available"},
		Metric{Key: "alchemy_successes", Label: "Successes", Value: strconv.FormatInt(successes, 10), Number: float64Ptr(float64(successes)), Unit: "attempts", Status: "available"},
		Metric{Key: "alchemy_failures", Label: "Failures", Value: strconv.FormatInt(failures, 10), Number: float64Ptr(float64(failures)), Unit: "attempts", Status: "available"},
		Metric{Key: "alchemy_unknown", Label: "Unknown outcome", Value: strconv.FormatInt(unknown, 10), Number: float64Ptr(float64(unknown)), Unit: "attempts", Status: "available"},
		Metric{Key: "alchemy_success_rate", Label: "Empirical success share", Number: float64Ptr(successRate), Unit: "% of attempts with known outcomes", Status: metricStatus(known > 0, "no_known_outcomes"), Reason: metricReason(known > 0, "no_attempts_with_known_outcome")},
		Metric{Key: "alchemy_highest_plus", Label: "Highest observed plus", Value: optionalInt(highest), Unit: "+", Status: metricStatus(highest != nil, "no_observed_plus"), Reason: metricReason(highest != nil, "no_attempts_with_plus_value")},
		Metric{Key: "alchemy_reach_target", Label: "Chance to reach target / attempts per item", Status: "unsupported", Reason: "complete comparable item attempt runs are not yet established"},
	)
	return queryOccurrences(ctx, tx, filter, where, args, &snapshot.Occurrences, &snapshot.NextCursor)
}

func queryAlchemyWeekdays(ctx context.Context, tx pgx.Tx, filter Filter, where string, args []any, target *[]Point) error {
	timezoneArg := len(args) + 1
	query := fmt.Sprintf(`SELECT EXTRACT(ISODOW FROM e.occurred_at AT TIME ZONE $%d)::integer AS weekday,
count(*) FILTER(WHERE e.payload->>'success'='true'),
count(*) FILTER(WHERE e.payload->>'success'='false'),
count(*) FILTER(WHERE e.payload->>'success' IS NULL OR e.payload->>'success' NOT IN ('true','false'))
FROM activity_events e WHERE %s GROUP BY weekday ORDER BY weekday`, timezoneArg, where)
	rows, err := tx.Query(ctx, query, append(args, filter.Timezone)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	weekdays := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	for rows.Next() {
		var weekday int
		var successes, failures, unknown int64
		if err := rows.Scan(&weekday, &successes, &failures, &unknown); err != nil {
			return err
		}
		known := successes + failures
		if weekday < 1 || weekday > len(weekdays) || known == 0 {
			continue
		}
		value := float64(successes) / float64(known) * 100
		*target = append(*target, Point{Bucket: strconv.Itoa(weekday), Label: weekdays[weekday-1], Value: strconv.FormatFloat(value, 'f', 2, 64), Series: "Observed success share", Detail: fmt.Sprintf("%d successes / %d known outcomes · %d unknown", successes, known, unknown)})
	}
	return rows.Err()
}

func queryTimeline(ctx context.Context, tx pgx.Tx, filter Filter, where string, args []any, series string, target *[]Point) (int64, error) {
	bucketArg, timezoneArg := len(args)+1, len(args)+2
	query := fmt.Sprintf(`SELECT to_char(date_trunc($%d,e.occurred_at AT TIME ZONE $%d),'YYYY-MM-DD HH24:MI:SS'), count(*) FROM activity_events e WHERE %s GROUP BY 1 ORDER BY 1 LIMIT 501`, bucketArg, timezoneArg, where)
	rows, err := tx.Query(ctx, query, append(args, filter.Bucket, filter.Timezone)...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var total int64
	for rows.Next() {
		var bucket string
		var count int64
		if err := rows.Scan(&bucket, &count); err != nil {
			return 0, err
		}
		total += count
		*target = append(*target, Point{Bucket: bucket, Label: bucket, Value: strconv.FormatInt(count, 10), Series: series})
	}
	return total, rows.Err()
}

func queryBreakdown(ctx context.Context, tx pgx.Tx, filter Filter, where string, args []any, by string) ([]Point, error) {
	labelSQL, idSQL, groupSQL := "", "NULL::text", ""
	switch by {
	case "character":
		labelSQL, idSQL, groupSQL = `c.character_name`, `c.character_id::text`, `c.character_name,c.character_id`
	case "location":
		labelSQL, groupSQL = `CASE WHEN e.zone_name IS NOT NULL AND e.region IS NOT NULL THEN 'Region '||e.region::text||' · '||e.zone_name WHEN e.zone_name IS NOT NULL THEN e.zone_name WHEN e.region IS NOT NULL THEN 'Region '||e.region::text ELSE 'Unknown location' END`, `1`
	case "group":
		labelSQL, groupSQL = `COALESCE(g.name,'Ungrouped')`, `1`
	case "item":
		labelSQL, groupSQL = `COALESCE(e.item_code,'Model '||e.item_model::text,'Unknown item')`, `1`
	case "academy":
		labelSQL, groupSQL = `COALESCE('Academy '||NULLIF(e.payload->>'academy_id',''),'Unknown academy')`, `1`
	default:
		return nil, fmt.Errorf("%w: invalid grouping", ErrInvalidFilter)
	}
	joins := `JOIN characters c ON c.character_id=e.character_id`
	if by == "group" {
		joins += ` LEFT JOIN character_group_members gm ON gm.character_id=e.character_id LEFT JOIN character_groups g ON g.group_id=gm.group_id`
	}
	rows, err := tx.Query(ctx, `WITH grouped AS (
 SELECT `+labelSQL+` AS label,`+idSQL+` AS character_id,count(*) AS amount FROM activity_events e `+joins+` WHERE `+where+` GROUP BY `+groupSQL+`
), ranked AS (SELECT label,character_id,amount,row_number() OVER (ORDER BY amount DESC,label,character_id NULLS LAST) AS ranking FROM grouped)
SELECT label,character_id,amount FROM ranked WHERE ranking<=20
UNION ALL SELECT 'Other',NULL::text,sum(amount) FROM ranked WHERE ranking>20 HAVING count(*)>0
ORDER BY amount DESC,label,character_id NULLS LAST`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Point, 0, 21)
	for rows.Next() {
		var label string
		var characterID *string
		var amount int64
		if err := rows.Scan(&label, &characterID, &amount); err != nil {
			return nil, err
		}
		point := Point{Label: label, Value: strconv.FormatInt(amount, 10), Series: by}
		if characterID != nil {
			point.CharacterID = *characterID
		}
		out = append(out, point)
	}
	return out, rows.Err()
}

func queryOccurrences(ctx context.Context, tx pgx.Tx, filter Filter, where string, args []any, target *[]Occurrence, next *string) error {
	var cursorAt *time.Time
	var cursorID *string
	if filter.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(filter.Cursor)
		if err != nil {
			return fmt.Errorf("%w: invalid cursor", ErrInvalidFilter)
		}
		parts := strings.SplitN(string(decoded), "|", 2)
		if len(parts) != 2 {
			return fmt.Errorf("%w: invalid cursor", ErrInvalidFilter)
		}
		value, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return fmt.Errorf("%w: invalid cursor", ErrInvalidFilter)
		}
		id := parts[1]
		if len(id) != 36 || strings.Count(id, "-") != 4 {
			return fmt.Errorf("%w: invalid cursor", ErrInvalidFilter)
		}
		cursorAt = &value
		cursorID = &id
	}
	cursorAtArg, cursorIDArg, limitArg := len(args)+1, len(args)+2, len(args)+3
	query := fmt.Sprintf(`SELECT e.event_id::text,e.kind,e.character_id::text,c.character_name,e.server_name,e.occurred_at,
COALESCE(e.payload->>'cause',e.payload->>'source',''),
CASE WHEN e.zone_name IS NOT NULL AND e.region IS NOT NULL THEN 'Region '||e.region::text||' · '||e.zone_name WHEN e.zone_name IS NOT NULL THEN e.zone_name WHEN e.region IS NOT NULL THEN 'Region '||e.region::text ELSE '' END,
e.region,e.item_model,COALESCE(e.item_code,''),e.payload->>'success',e.payload->>'plus',e.payload
FROM activity_events e JOIN characters c ON c.character_id=e.character_id WHERE %s
AND ($%d::timestamptz IS NULL OR (e.occurred_at,e.event_id)<($%d,$%d::uuid))
ORDER BY e.occurred_at DESC,e.event_id DESC LIMIT $%d`, where, cursorAtArg, cursorAtArg, cursorIDArg, limitArg)
	queryArgs := append(append([]any{}, args...), cursorAt, cursorID, filter.PageSize+1)
	rows, err := tx.Query(ctx, query, queryArgs...)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := make([]Occurrence, 0, filter.PageSize+1)
	for rows.Next() {
		var item Occurrence
		var successText, plusText *string
		var payload []byte
		if err := rows.Scan(&item.ID, &item.Kind, &item.CharacterID, &item.Character, &item.Server, &item.OccurredAt, &item.Detail, &item.Location, &item.Region, &item.ItemModel, &item.ItemCode, &successText, &plusText, &payload); err != nil {
			return err
		}
		item.Payload = map[string]any{}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &item.Payload); err != nil {
				return err
			}
		}
		if successText != nil {
			value := *successText == "true"
			item.Success = &value
		}
		if plusText != nil {
			if value, parseErr := strconv.Atoi(*plusText); parseErr == nil {
				item.Plus = &value
			}
		}
		item.OccurredAt = item.OccurredAt.UTC()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(items) > filter.PageSize {
		last := items[filter.PageSize-1]
		*next = base64.RawURLEncoding.EncodeToString([]byte(last.OccurredAt.Format(time.RFC3339Nano) + "|" + last.ID))
		items = items[:filter.PageSize]
	}
	*target = items
	return nil
}

func (s *Store) enrichOccurrences(occurrences []Occurrence) {
	if s == nil || s.itemDetails == nil {
		return
	}
	for index := range occurrences {
		item := occurrences[index]
		payloadItem, _ := item.Payload["item"].(map[string]any)
		metadata, details := s.itemDetails(item.Server, item.ItemModel, item.ItemCode, payloadItem)
		occurrences[index].ItemMetadata = metadata
		occurrences[index].ItemDetails = details
		if name, ok := metadata["name"].(string); ok {
			occurrences[index].ItemName = name
		}
		if icon, ok := metadata["icon_url"].(string); ok {
			occurrences[index].ItemIconURL = icon
		}
	}
}

func localCalendarDays(from, to time.Time, timezone string) (int, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return 0, err
	}
	start := from.In(location)
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, location)
	end := to.Add(-time.Nanosecond).In(location)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, location)
	days := 0
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		days++
		if days > 367 {
			return 0, fmt.Errorf("%w: too many calendar days", ErrInvalidFilter)
		}
	}
	return days, nil
}

func float64Ptr(value float64) *float64 { return &value }
func metricStatus(available bool, emptyReason string) string {
	if available {
		return "available"
	}
	return "empty"
}
func metricReason(available bool, reason string) string {
	if available {
		return ""
	}
	return reason
}
func optionalInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}
func leaderMetric(key, label string, points []Point, hrefPrefix string) Metric {
	// "Other" is a presentation aggregate, not a real character or location.
	for _, point := range points {
		if strings.EqualFold(strings.TrimSpace(point.Label), "other") {
			continue
		}
		metric := Metric{Key: key, Label: label, Value: point.Label, Status: "available"}
		if point.CharacterID != "" && hrefPrefix != "" {
			metric.Href = hrefPrefix + "&character_id=" + url.QueryEscape(point.CharacterID)
		}
		return metric
	}
	if len(points) == 0 {
		return Metric{Key: key, Label: label, Status: "empty", Reason: "no_recorded_occurrences"}
	}
	return Metric{Key: key, Label: label, Status: "empty", Reason: "no_ranked_entity"}
}

func dropEventKind(view View) string {
	if view == ViewRareDrops {
		return "drop.rare"
	}
	return "drop.item"
}

func dropPopulationLabel(source string) string {
	if source == "owned_gains" {
		return "Recorded owned-item gain events"
	}
	return "Recorded world-drop observations"
}

func dropPopulationUnit(source string) string {
	if source == "owned_gains" {
		return "gain events"
	}
	return "observations"
}

func dropLeaderHref(filter Filter) string {
	if filter.DropSource == "owned_gains" {
		return ""
	}
	return eventEvidencePrefix(filter, dropEventKind(filter.View), true)
}

func eventEvidencePrefix(filter Filter, kind string, worldDrops bool) string {
	prefix := "/events?kind=" + url.QueryEscape(kind) +
		"&from_ts=" + url.QueryEscape(filter.From.UTC().Format(time.RFC3339Nano)) +
		"&to_ts=" + url.QueryEscape(filter.To.UTC().Format(time.RFC3339Nano))
	if filter.Server != "" {
		prefix += "&server=" + url.QueryEscape(filter.Server)
	}
	if worldDrops {
		prefix += "&include_owned_gains=false&include_pet_pickups=false"
	}
	return prefix
}

// Ensure the production store and tests use the same pool/transaction contract.
var _ interface {
	Query(context.Context, Filter) (Snapshot, error)
} = (*Store)(nil)

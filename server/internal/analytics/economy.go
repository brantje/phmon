package analytics

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
)

func queryEconomy(ctx context.Context, tx pgx.Tx, filter Filter, snapshot *Snapshot) error {
	if filter.BalanceScope == "guild_storage" {
		return queryGuildGold(ctx, tx, filter, snapshot)
	}
	return queryCharacterGold(ctx, tx, filter, snapshot)
}

func queryCharacterGold(ctx context.Context, tx pgx.Tx, filter Filter, snapshot *Snapshot) error {
	where := `s.sampled_at >= $1 AND s.sampled_at < $2 AND ($3='' OR s.server_key=$3)
AND ($4='' OR s.character_id=$4::uuid)
AND ($5='' OR EXISTS(SELECT 1 FROM character_group_members m WHERE m.character_id=s.character_id AND m.group_id=$5::uuid))`
	args := []any{filter.From, filter.To, filter.Server, filter.CharacterID, filter.GroupID, filter.Bucket, filter.Timezone}
	var entityCount int64
	if err := tx.QueryRow(ctx, `SELECT count(DISTINCT s.character_id) FROM character_metric_samples s WHERE `+where+` AND s.gold IS NOT NULL`, args[:5]...).Scan(&entityCount); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `WITH eligible AS (
    SELECT s.character_id,c.character_name,s.gold,s.sampled_at,s.sample_id,
           date_trunc($6,s.sampled_at AT TIME ZONE $7) AS bucket
    FROM character_metric_samples s JOIN characters c ON c.character_id=s.character_id
    WHERE `+where+` AND s.gold IS NOT NULL
), selected AS (
    SELECT character_id FROM eligible GROUP BY character_id
    ORDER BY max(sampled_at) DESC,character_id LIMIT 20
), latest AS (
    SELECT DISTINCT ON(e.character_id,e.bucket) e.bucket,e.character_name,e.character_id,e.gold
    FROM eligible e JOIN selected selected_character ON selected_character.character_id=e.character_id
    ORDER BY e.character_id,e.bucket,e.sampled_at DESC,e.sample_id DESC
)
SELECT to_char(bucket,'YYYY-MM-DD HH24:MI:SS'),character_name,character_id,gold::text
FROM latest ORDER BY bucket,character_name,character_id`, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var bucket, name, id, gold string
		if err := rows.Scan(&bucket, &name, &id, &gold); err != nil {
			rows.Close()
			return err
		}
		snapshot.TimeSeries = append(snapshot.TimeSeries, Point{Bucket: bucket, Label: bucket, Value: gold, Series: name, CharacterID: id})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if entityCount > 20 {
		snapshot.Truncated = true
		snapshot.Summary = append(snapshot.Summary, Metric{Key: "balance_series_omitted", Label: "Omitted character balance series", Value: strconv.FormatInt(entityCount-20, 10), Unit: "characters", Status: "limited", Reason: "chart is bounded to the 20 most recently observed characters"})
	}

	leaders, err := tx.Query(ctx, `SELECT character_name,character_id::text,gold::text FROM (
SELECT DISTINCT ON(s.character_id) c.character_name,s.character_id,s.gold,s.sampled_at
FROM character_metric_samples s JOIN characters c ON c.character_id=s.character_id
WHERE `+where+` AND s.gold IS NOT NULL ORDER BY s.character_id,s.sampled_at DESC,s.sample_id DESC
) latest ORDER BY gold::numeric DESC,character_name,character_id LIMIT 20`, filter.From, filter.To, filter.Server, filter.CharacterID, filter.GroupID)
	if err != nil {
		return err
	}
	for leaders.Next() {
		var name, id, gold string
		if err := leaders.Scan(&name, &id, &gold); err != nil {
			leaders.Close()
			return err
		}
		snapshot.Breakdown = append(snapshot.Breakdown, Point{Label: name, Value: gold, Series: "gold", CharacterID: id})
	}
	if err := leaders.Err(); err != nil {
		leaders.Close()
		return err
	}
	leaders.Close()

	var delta float64
	var observedDays int64
	err = tx.QueryRow(ctx, `WITH sequenced AS (
SELECT s.character_id,s.session_id,s.sampled_at,date_trunc('day',s.sampled_at AT TIME ZONE $6) AS local_day,s.gold,
lag(s.sampled_at) OVER(PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id) AS before_at,
lag(s.gold) OVER(PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id) AS before_gold
FROM character_metric_samples s WHERE s.sampled_at >= ($1::timestamptz-interval '30 seconds') AND s.sampled_at < $2
AND ($3='' OR s.server_key=$3) AND ($4='' OR s.character_id=$4::uuid)
AND ($5='' OR EXISTS(SELECT 1 FROM character_group_members m WHERE m.character_id=s.character_id AND m.group_id=$5::uuid))
), eligible AS (
SELECT character_id,local_day,gold-before_gold AS delta FROM sequenced
WHERE sampled_at >= $1 AND sampled_at < $2 AND before_at >= $1 AND sampled_at-before_at <= interval '30 seconds' AND gold IS NOT NULL AND before_gold IS NOT NULL
)
SELECT COALESCE(sum(delta),0)::float8,count(DISTINCT(character_id,local_day)) FROM eligible`, filter.From, filter.To, filter.Server, filter.CharacterID, filter.GroupID, filter.Timezone).Scan(&delta, &observedDays)
	if err != nil {
		return err
	}
	if observedDays > 0 {
		snapshot.Summary = append(snapshot.Summary, Metric{Key: "gold_net_per_observed_character_day", Label: "Average net gold change / observed character-day", Number: float64Ptr(delta / float64(observedDays)), Unit: "gold per observed character-day", Status: "available"})
	} else {
		snapshot.Summary = append(snapshot.Summary, Metric{Key: "gold_net_per_observed_character_day", Label: "Average net gold change / observed character-day", Status: "insufficient_history", Reason: "no_comparable_balance_intervals_in_selected_window"})
	}
	if len(snapshot.Breakdown) > 0 {
		leader := snapshot.Breakdown[0]
		snapshot.Summary = append(snapshot.Summary, Metric{Key: "highest_character_balance", Label: "Character with highest observed balance", Value: leader.Label + " · " + leader.Value + " gold", Status: "available", Href: "/characters/" + leader.CharacterID})
	} else {
		snapshot.Summary = append(snapshot.Summary, Metric{Key: "highest_character_balance", Label: "Character with highest observed balance", Status: "empty", Reason: "no_observed_character_gold_in_selected_window"})
	}
	snapshot.Summary = append(snapshot.Summary, Metric{Key: "stall_sales", Label: "Recorded stall sales", Status: "unsupported", Reason: "no verified sale transaction source is available"})
	snapshot.Total = strconv.FormatInt(entityCount, 10)
	if len(snapshot.TimeSeries) == 0 {
		snapshot.Status = "empty"
		snapshot.Reason = "no_character_balance_samples_in_selected_window"
	}
	return nil
}

func queryGuildGold(ctx context.Context, tx pgx.Tx, filter Filter, snapshot *Snapshot) error {
	where := `s.sampled_at >= $1 AND s.sampled_at < $2 AND ($3='' OR s.server_key=$3)
AND ($4='' OR s.guild_key=$4) AND ($5='' OR s.observer_character_id=$5::uuid)`
	args := []any{filter.From, filter.To, filter.Server, filter.Guild, filter.CharacterID, filter.Bucket, filter.Timezone}
	var entityCount int64
	if err := tx.QueryRow(ctx, `SELECT count(DISTINCT (s.server_key,s.guild_key)) FROM guild_gold_samples s WHERE `+where, args[:5]...).Scan(&entityCount); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `WITH eligible AS (
    SELECT s.server_key,s.guild_key,s.gold,s.sampled_at,s.observer_character_id,
           date_trunc($6,s.sampled_at AT TIME ZONE $7) AS bucket
    FROM guild_gold_samples s WHERE `+where+`
), selected AS (
    SELECT server_key,guild_key FROM eligible GROUP BY server_key,guild_key
    ORDER BY max(sampled_at) DESC,server_key,guild_key LIMIT 20
), latest AS (
    SELECT DISTINCT ON(e.server_key,e.guild_key,e.bucket) e.bucket,e.server_key,e.guild_key,e.gold
    FROM eligible e JOIN selected selected_guild ON selected_guild.server_key=e.server_key AND selected_guild.guild_key=e.guild_key
    ORDER BY e.server_key,e.guild_key,e.bucket,e.sampled_at DESC,e.observer_character_id
)
SELECT to_char(bucket,'YYYY-MM-DD HH24:MI:SS'),server_key,guild_key,gold::text
FROM latest ORDER BY bucket,server_key,guild_key`, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var bucket, server, guild, gold string
		if err := rows.Scan(&bucket, &server, &guild, &gold); err != nil {
			rows.Close()
			return err
		}
		series := guild
		if filter.Server == "" {
			series = server + "/" + guild
		}
		snapshot.TimeSeries = append(snapshot.TimeSeries, Point{Bucket: bucket, Label: bucket, Value: gold, Series: series, Server: server})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if entityCount > 20 {
		snapshot.Truncated = true
		snapshot.Summary = append(snapshot.Summary, Metric{Key: "balance_series_omitted", Label: "Omitted guild balance series", Value: strconv.FormatInt(entityCount-20, 10), Unit: "guilds", Status: "limited", Reason: "chart is bounded to the 20 most recently observed guild/server scopes"})
	}
	leaders, err := tx.Query(ctx, `SELECT server_key,guild_key,gold::text FROM (
SELECT DISTINCT ON(s.server_key,s.guild_key) s.server_key,s.guild_key,s.gold,s.sampled_at,s.observer_character_id
FROM guild_gold_samples s WHERE `+where+`
ORDER BY s.server_key,s.guild_key,s.sampled_at DESC,s.observer_character_id) latest
		ORDER BY gold::numeric DESC,guild_key,server_key LIMIT 20`, filter.From, filter.To, filter.Server, filter.Guild, filter.CharacterID)
	if err != nil {
		return err
	}
	for leaders.Next() {
		var server, guild, gold string
		if err := leaders.Scan(&server, &guild, &gold); err != nil {
			leaders.Close()
			return err
		}
		label := guild
		if filter.Server == "" {
			label = server + "/" + guild
		}
		snapshot.Breakdown = append(snapshot.Breakdown, Point{Label: label, Value: gold, Series: "gold", Server: server})
	}
	if err := leaders.Err(); err != nil {
		leaders.Close()
		return err
	}
	leaders.Close()
	if len(snapshot.Breakdown) > 0 {
		point := snapshot.Breakdown[0]
		snapshot.Summary = append(snapshot.Summary, Metric{Key: "highest_guild_balance", Label: "Guild with highest observed balance", Value: point.Label + " · " + point.Value + " gold", Status: "available"})
	} else {
		snapshot.Summary = append(snapshot.Summary, Metric{Key: "highest_guild_balance", Label: "Guild with highest observed balance", Status: "empty", Reason: "no_guild_storage_gold_samples_in_selected_window"})
	}
	snapshot.Summary = append(snapshot.Summary,
		Metric{Key: "guild_balance_change", Label: "Guild storage net change", Status: "unsupported", Reason: "overlapping observers cannot yet be reconciled into one continuous guild balance series"},
		Metric{Key: "stall_sales", Label: "Recorded stall sales", Status: "unsupported", Reason: "no verified sale transaction source is available"})
	snapshot.Total = strconv.FormatInt(entityCount, 10)
	if len(snapshot.TimeSeries) == 0 {
		snapshot.Status = "empty"
		snapshot.Reason = "no_guild_storage_gold_samples_in_selected_window"
	}
	return nil
}

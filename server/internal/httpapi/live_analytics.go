package httpapi

import (
	"errors"
	"strings"

	"phmon/server/internal/analytics"
)

func analyticsFilter(input liveFilter) (analytics.Filter, error) {
	from, err := parseEventBound(input.From, false)
	if err != nil {
		return analytics.Filter{}, errors.New("invalid analytics start time")
	}
	to, err := parseEventBound(input.To, true)
	if err != nil {
		return analytics.Filter{}, errors.New("invalid analytics end time")
	}
	return analytics.NormalizeFilter(analytics.Filter{
		Server: strings.TrimSpace(input.Server), CharacterID: input.CharacterID, CharacterIDs: input.CharacterIDs, CharacterQuery: input.Query, GroupID: input.GroupID,
		View: analytics.View(input.AnalyticsView), From: from, To: to,
		Timezone: input.Timezone, Bucket: input.Bucket, GroupBy: input.GroupBy,
		Guild: input.Guild, BalanceScope: input.BalanceScope, DropSource: input.DropSource, ItemType: input.ItemType, ItemDegree: input.ItemDegree, ItemQuery: input.Item,
		PageSize: input.PageSize, Cursor: input.Cursor,
	})
}

func hasAnalyticsFilters(filter liveFilter) bool {
	return filter.AnalyticsView != "" || filter.Timezone != "" || filter.Bucket != "" ||
		filter.GroupBy != "" || filter.Guild != "" || filter.PageSize != 0 || filter.BalanceScope != "" || filter.DropSource != "" || filter.ItemType != "" || filter.ItemDegree != ""
}

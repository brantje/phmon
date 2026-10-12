package phproxy

import "time"

func (s *Store) projectEnrichment(server string, st *storedFields) *EnrichmentProjection {
	if st == nil {
		return nil
	}
	if st.ModelRefID == nil && len(st.Gear) == 0 && st.GearObservedAt == nil && st.Extended.isEmpty() {
		return nil
	}
	out := &EnrichmentProjection{
		Source:        "ph-proxy",
		ModelRefID:    st.ModelRefID,
		FieldEvidence: st.FieldEvidence,
		Level:         st.Extended.Level,
		Job:           st.Extended.Job,
		JobLevel:      st.Extended.JobLevel,
		Race:          st.Extended.Race,
		Guild:         st.Extended.Guild,
		Position:      st.Extended.Position,
	}
	if len(st.Extended.JobItemIDs) > 0 || st.Extended.JobItemIDsObservedAt != nil {
		out.JobItemIDs = append([]int(nil), st.Extended.JobItemIDs...)
	}
	if len(st.Extended.IdentityLinks) > 0 || st.Extended.IdentityLinksObservedAt != nil {
		out.IdentityLinks = append([]IdentityLink(nil), st.Extended.IdentityLinks...)
	}
	copyObservedAt(st.GearObservedAt, &out.GearObservedAt)
	copyObservedAt(st.Extended.AvatarGearObservedAt, &out.AvatarGearObservedAt)
	copyObservedAt(st.Extended.LevelObservedAt, &out.LevelObservedAt)
	copyObservedAt(st.Extended.JobObservedAt, &out.JobObservedAt)
	copyObservedAt(st.Extended.PositionObservedAt, &out.PositionObservedAt)
	copyObservedAt(st.Extended.JobItemIDsObservedAt, &out.JobItemIDsObservedAt)
	copyObservedAt(st.Extended.IdentityLinksObservedAt, &out.IdentityLinksObservedAt)
	for _, g := range st.Gear {
		out.Gear = append(out.Gear, s.projectGear(server, g))
	}
	for _, g := range st.Extended.AvatarGear {
		out.AvatarGear = append(out.AvatarGear, s.projectGear(server, g))
	}
	return out
}

func (e ExtendedPayload) isEmpty() bool {
	return len(e.AvatarGear) == 0 && e.Level == nil && e.Job == nil && e.JobLevel == nil &&
		e.Race == nil && e.Guild == nil && e.Position == nil && len(e.JobItemIDs) == 0 &&
		len(e.IdentityLinks) == 0
}

func copyObservedAt(src *time.Time, dst **time.Time) {
	if src == nil {
		return
	}
	t := src.UTC()
	*dst = &t
}

func (s *Store) projectGear(server string, g GearEntry) GearProjection {
	model := int64(g.RefItemID)
	item := map[string]any{
		"model":                  g.RefItemID,
		"visible_equipment_only": true,
	}
	if g.OptLevel != nil {
		item["plus"] = *g.OptLevel
	}
	if s.present != nil {
		if meta := s.present.ItemPresentation(server, &model, ""); len(meta) > 0 {
			item["metadata"] = meta
		}
	}
	return GearProjection{
		RefItemID: g.RefItemID,
		OptLevel:  g.OptLevel,
		Slot:      g.Slot,
		Item:      item,
	}
}

func (s *Store) BuildListDisplayFallback(nativeLevel *int, nativeJob *string, nativeJobLevel *int, nativeGuild *string, nativeModel *int64, st *storedFields) *ListDisplayFallback {
	if st == nil {
		return nil
	}
	out := &ListDisplayFallback{}
	if nativeLevel == nil && st.ProxyLevel != nil {
		out.Level = st.ProxyLevel
		out.LevelSource = "ph-proxy"
	}
	if nativeJob == nil && st.ProxyJob != nil {
		out.Job = *st.ProxyJob
		out.JobSource = "ph-proxy"
	}
	if nativeJobLevel == nil && st.ProxyJobLevel != nil {
		out.JobLevel = st.ProxyJobLevel
		out.JobLevelSource = "ph-proxy"
	}
	if nativeGuild == nil && st.ProxyGuild != nil {
		out.Guild = *st.ProxyGuild
		out.GuildSource = "ph-proxy"
	}
	if nativeModel == nil && st.ModelRefID != nil {
		v := int64(*st.ModelRefID)
		out.ModelID = &v
		out.ModelSource = "ph-proxy"
	}
	if out.Level == nil && out.Job == "" && out.JobLevel == nil && out.Guild == "" && out.ModelID == nil {
		return nil
	}
	return out
}

package phproxy

import (
	"fmt"
	"reflect"
	"time"
)

func mergeV2Fields(current *storedFields, vp validatedPlayer) mergeOutcome {
	out := mergeOutcome{}
	out = mergeOutcomeUnion(out, mergeAvatarGear(current, vp))
	out = mergeOutcomeUnion(out, mergeLevel(current, vp))
	out = mergeOutcomeUnion(out, mergeJob(current, vp))
	out = mergeOutcomeUnion(out, mergeJobLevel(current, vp))
	out = mergeOutcomeUnion(out, mergeRace(current, vp))
	out = mergeOutcomeUnion(out, mergeGuild(current, vp))
	out = mergeOutcomeUnion(out, mergePosition(current, vp))
	out = mergeOutcomeUnion(out, mergeJobItemIDs(current, vp))
	out = mergeOutcomeUnion(out, mergeIdentityLinks(current, vp))
	return out
}

func mergeOutcomeUnion(left, right mergeOutcome) mergeOutcome {
	return mergeOutcome{
		Changed:       left.Changed || right.Changed,
		Stale:         left.Stale || right.Stale,
		EqualConflict: left.EqualConflict || right.EqualConflict,
	}
}

func mergeAvatarGear(current *storedFields, vp validatedPlayer) mergeOutcome {
	if !vp.HasAvatarGear {
		return mergeOutcome{}
	}
	at := vp.Evidence["avatar_gear"].ObservedAt.UTC()
	if current.Extended.AvatarGearObservedAt == nil {
		current.Extended.AvatarGear = append([]GearEntry(nil), vp.AvatarGear...)
		current.Extended.AvatarGearObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"avatar_gear": vp.Evidence["avatar_gear"],
		})
		return mergeOutcome{Changed: true}
	}
	curAt := current.Extended.AvatarGearObservedAt.UTC()
	switch {
	case at.After(curAt):
		current.Extended.AvatarGear = append([]GearEntry(nil), vp.AvatarGear...)
		current.Extended.AvatarGearObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"avatar_gear": vp.Evidence["avatar_gear"],
		})
		return mergeOutcome{Changed: true}
	case at.Before(curAt):
		return mergeOutcome{Stale: true}
	default:
		if gearEqual(current.Extended.AvatarGear, vp.AvatarGear) {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func mergeLevel(current *storedFields, vp validatedPlayer) mergeOutcome {
	if !vp.HasLevel {
		return mergeOutcome{}
	}
	at := vp.Evidence["level"].ObservedAt.UTC()
	if current.Extended.LevelObservedAt == nil {
		current.Extended.Level = vp.Level
		current.Extended.LevelObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"level": vp.Evidence["level"],
		})
		return mergeOutcome{Changed: true}
	}
	curAt := current.Extended.LevelObservedAt.UTC()
	switch {
	case at.After(curAt):
		current.Extended.Level = vp.Level
		current.Extended.LevelObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"level": vp.Evidence["level"],
		})
		return mergeOutcome{Changed: true}
	case at.Before(curAt):
		return mergeOutcome{Stale: true}
	default:
		if intPtrEqual(current.Extended.Level, vp.Level) {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func mergeJob(current *storedFields, vp validatedPlayer) mergeOutcome {
	if !vp.HasJob {
		return mergeOutcome{}
	}
	at := vp.Evidence["job"].ObservedAt.UTC()
	if current.Extended.JobObservedAt == nil {
		current.Extended.Job = vp.Job
		current.Extended.JobObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"job": vp.Evidence["job"],
		})
		return mergeOutcome{Changed: true}
	}
	curAt := current.Extended.JobObservedAt.UTC()
	switch {
	case at.After(curAt):
		current.Extended.Job = vp.Job
		current.Extended.JobObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"job": vp.Evidence["job"],
		})
		return mergeOutcome{Changed: true}
	case at.Before(curAt):
		return mergeOutcome{Stale: true}
	default:
		if intPtrEqual(current.Extended.Job, vp.Job) {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func mergeJobLevel(current *storedFields, vp validatedPlayer) mergeOutcome {
	if !vp.HasJobLevel {
		return mergeOutcome{}
	}
	at := vp.Evidence["job_level"].ObservedAt.UTC()
	if current.Extended.JobLevelObservedAt == nil {
		current.Extended.JobLevel = vp.JobLevel
		current.Extended.JobLevelObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"job_level": vp.Evidence["job_level"],
		})
		return mergeOutcome{Changed: true}
	}
	curAt := current.Extended.JobLevelObservedAt.UTC()
	switch {
	case at.After(curAt):
		current.Extended.JobLevel = vp.JobLevel
		current.Extended.JobLevelObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"job_level": vp.Evidence["job_level"],
		})
		return mergeOutcome{Changed: true}
	case at.Before(curAt):
		return mergeOutcome{Stale: true}
	default:
		if intPtrEqual(current.Extended.JobLevel, vp.JobLevel) {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func mergeRace(current *storedFields, vp validatedPlayer) mergeOutcome {
	return mergeStringField(
		&current.Extended.Race, &current.Extended.RaceObservedAt,
		vp.HasRace, vp.Race, vp.Evidence["race"], current,
		"race",
	)
}

func mergeGuild(current *storedFields, vp validatedPlayer) mergeOutcome {
	return mergeStringField(
		&current.Extended.Guild, &current.Extended.GuildObservedAt,
		vp.HasGuild, vp.Guild, vp.Evidence["guild"], current,
		"guild",
	)
}

func mergeStringField(
	current **string, currentAt **time.Time,
	has bool, incoming *string, ev FieldEvidence, st *storedFields, key string,
) mergeOutcome {
	if !has || incoming == nil {
		return mergeOutcome{}
	}
	at := ev.ObservedAt.UTC()
	if *currentAt == nil {
		*current = incoming
		*currentAt = &at
		st.FieldEvidence = mergeFieldEvidence(st.FieldEvidence, map[string]FieldEvidence{key: ev})
		return mergeOutcome{Changed: true}
	}
	curAt := (*currentAt).UTC()
	switch {
	case at.After(curAt):
		*current = incoming
		*currentAt = &at
		st.FieldEvidence = mergeFieldEvidence(st.FieldEvidence, map[string]FieldEvidence{key: ev})
		return mergeOutcome{Changed: true}
	case at.Before(curAt):
		return mergeOutcome{Stale: true}
	default:
		if *current != nil && **current == *incoming {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func mergePosition(current *storedFields, vp validatedPlayer) mergeOutcome {
	if !vp.HasPosition || vp.Position == nil {
		return mergeOutcome{}
	}
	at := vp.Evidence["position"].ObservedAt.UTC()
	if current.Extended.PositionObservedAt == nil {
		current.Extended.Position = vp.Position
		current.Extended.PositionObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"position": vp.Evidence["position"],
		})
		return mergeOutcome{Changed: true}
	}
	curAt := current.Extended.PositionObservedAt.UTC()
	switch {
	case at.After(curAt):
		current.Extended.Position = vp.Position
		current.Extended.PositionObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"position": vp.Evidence["position"],
		})
		return mergeOutcome{Changed: true}
	case at.Before(curAt):
		return mergeOutcome{Stale: true}
	default:
		if reflect.DeepEqual(current.Extended.Position, vp.Position) {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func mergeJobItemIDs(current *storedFields, vp validatedPlayer) mergeOutcome {
	if !vp.HasJobItemIDs {
		return mergeOutcome{}
	}
	at := vp.Evidence["job_item_ids"].ObservedAt.UTC()
	if current.Extended.JobItemIDsObservedAt == nil {
		current.Extended.JobItemIDs = append([]int(nil), vp.JobItemIDs...)
		current.Extended.JobItemIDsObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"job_item_ids": vp.Evidence["job_item_ids"],
		})
		return mergeOutcome{Changed: true}
	}
	curAt := current.Extended.JobItemIDsObservedAt.UTC()
	switch {
	case at.After(curAt):
		current.Extended.JobItemIDs = append([]int(nil), vp.JobItemIDs...)
		current.Extended.JobItemIDsObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"job_item_ids": vp.Evidence["job_item_ids"],
		})
		return mergeOutcome{Changed: true}
	case at.Before(curAt):
		return mergeOutcome{Stale: true}
	default:
		if reflect.DeepEqual(current.Extended.JobItemIDs, vp.JobItemIDs) {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func mergeIdentityLinks(current *storedFields, vp validatedPlayer) mergeOutcome {
	if !vp.HasIdentityLinks {
		return mergeOutcome{}
	}
	at := vp.Evidence["identity_links"].ObservedAt.UTC()
	if current.Extended.IdentityLinksObservedAt == nil {
		current.Extended.IdentityLinks = append([]IdentityLink(nil), vp.IdentityLinks...)
		current.Extended.IdentityLinksObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"identity_links": vp.Evidence["identity_links"],
		})
		return mergeOutcome{Changed: true}
	}
	curAt := current.Extended.IdentityLinksObservedAt.UTC()
	switch {
	case at.After(curAt):
		current.Extended.IdentityLinks = append([]IdentityLink(nil), vp.IdentityLinks...)
		current.Extended.IdentityLinksObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"identity_links": vp.Evidence["identity_links"],
		})
		return mergeOutcome{Changed: true}
	case at.Before(curAt):
		return mergeOutcome{Stale: true}
	default:
		if reflect.DeepEqual(current.Extended.IdentityLinks, vp.IdentityLinks) {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func syncProxyFallbacks(st *storedFields) {
	st.ProxyLevel = nil
	st.ProxyJob = nil
	st.ProxyJobLevel = nil
	st.ProxyGuild = nil
	st.ProxyRace = nil
	if st.Extended.Level != nil && isCharacterNameEvidence(st.FieldEvidence["level"]) {
		st.ProxyLevel = st.Extended.Level
	}
	if st.Extended.Job != nil && isCharacterNameEvidence(st.FieldEvidence["job"]) {
		st.ProxyJob = intPtrString(st.Extended.Job)
	}
	if st.Extended.JobLevel != nil && isCharacterNameEvidence(st.FieldEvidence["job_level"]) {
		st.ProxyJobLevel = st.Extended.JobLevel
	}
	if st.Extended.Guild != nil && isCharacterNameEvidence(st.FieldEvidence["guild"]) {
		st.ProxyGuild = st.Extended.Guild
	}
	if st.Extended.Race != nil && isCharacterNameEvidence(st.FieldEvidence["race"]) {
		st.ProxyRace = st.Extended.Race
	}
}

func isCharacterNameEvidence(ev FieldEvidence) bool {
	return ev.SourceIdentityKind == IdentityKindCharacterName || ev.SourceIdentityKind == ""
}

func intPtrString(v *int) *string {
	if v == nil {
		return nil
	}
	s := fmt.Sprintf("%d", *v)
	return &s
}

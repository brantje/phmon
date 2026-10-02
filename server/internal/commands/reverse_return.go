package commands

import (
	"context"
	"time"
)

// ReverseReturnContext contains current-session resource evidence, not command eligibility.
// ScrollObserved is advisory; only the native reverse_return API decides availability.
type ReverseReturnContext struct {
	SessionID          string     `json:"session_id"`
	PartyStatus        string     `json:"party_status"`
	PartyNames         []string   `json:"party_names"`
	PartyCheckedAt     *time.Time `json:"party_checked_at,omitempty"`
	ScrollObserved     *bool      `json:"scroll_observed"`
	InventoryCheckedAt *time.Time `json:"inventory_checked_at,omitempty"`
}

type ReverseReturnContextProvider interface {
	ReverseReturnContexts(context.Context, map[string]string, time.Time) (map[string]ReverseReturnContext, error)
}

type ReverseReturnLocationProvider interface {
	ReverseReturnLocations(server string) []string
}

func (s *Service) SetReverseReturnLocations(provider ReverseReturnLocationProvider) {
	s.reverseLocations = provider
}

func (s *Service) reverseReturnLocations(server string) []string {
	if s.reverseLocations == nil {
		return nil
	}
	return s.reverseLocations.ReverseReturnLocations(server)
}

func (s *Service) namedReverseReturnReason(server, name string) string {
	locations := s.reverseReturnLocations(server)
	if len(locations) == 0 {
		return "named_location_names_unavailable"
	}
	for _, location := range locations {
		if location == name {
			return ""
		}
	}
	return "named_location_not_found"
}

// addReverseReturnContexts is best-effort: resource context enriches controls but
// must not make the core control projection unavailable when its query fails.
func (s *Service) addReverseReturnContexts(ctx context.Context, snapshots []map[string]any) {
	if s.reverseReturn == nil {
		return
	}
	sessions := make(map[string]string)
	for _, snapshot := range snapshots {
		id, _ := snapshot["character_id"].(string)
		session, _ := snapshot["session_id"].(string)
		if session != "" {
			sessions[id] = session
		}
	}
	if len(sessions) == 0 {
		return
	}
	contexts, err := s.reverseReturn.ReverseReturnContexts(ctx, sessions, s.now())
	if err != nil {
		return
	}
	for _, snapshot := range snapshots {
		id, _ := snapshot["character_id"].(string)
		if evidence, ok := contexts[id]; ok {
			snapshot["reverse_return"] = evidence
		}
	}
}

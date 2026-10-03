package resources

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode"

	"phmon/server/internal/commands"
)

func (s *Store) ReverseReturnContexts(ctx context.Context, sessions map[string]string, now time.Time) (map[string]commands.ReverseReturnContext, error) {
	result := make(map[string]commands.ReverseReturnContext, len(sessions))
	ids := make([]string, 0, len(sessions))
	for id, session := range sessions {
		ids = append(ids, id)
		result[id] = commands.ReverseReturnContext{SessionID: session, PartyStatus: "unavailable", PartyNames: []string{}}
	}
	if len(ids) == 0 || s == nil || s.pool == nil {
		return result, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT o.observer_character_id::text,o.session_id::text,o.resource_key,o.availability,o.payload,o.updated_at
 FROM character_resource_observations o
 JOIN character_resource_state rs ON rs.character_id=o.observer_character_id AND rs.session_id=o.session_id
 JOIN character_sessions cs ON cs.character_id=o.observer_character_id AND cs.session_id=o.session_id AND cs.ended_at IS NULL
 AND cs.agent_id=rs.agent_id AND cs.connection_generation=rs.connection_generation
 WHERE o.observer_character_id=ANY($1::uuid[]) AND o.resource_key IN ('party','inventory')`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, session, key, availability string
		var payload []byte
		var checked time.Time
		if err := rows.Scan(&id, &session, &key, &availability, &payload, &checked); err != nil {
			return nil, err
		}
		if sessions[id] != session {
			continue
		}
		value := result[id]
		applyReverseReturnResource(&value, key, availability, payload, checked, now)
		result[id] = value
	}
	return result, rows.Err()
}

func applyReverseReturnResource(value *commands.ReverseReturnContext, key, availability string, payload []byte, checked, now time.Time) {
	fresh := now.Sub(checked) >= -5*time.Second && now.Sub(checked) <= 35*time.Second
	if key == "party" {
		value.PartyCheckedAt = &checked
		if availability != "observed" {
			value.PartyStatus = "unavailable"
			return
		}
		if !fresh {
			value.PartyStatus = "stale"
			return
		}
		value.PartyStatus = "observed"
		seen := map[string]bool{}
		for _, member := range decodePartyMembers(payload) {
			name := member.Name
			if !reverseReturnName(name) || seen[strings.ToLower(name)] {
				continue
			}
			seen[strings.ToLower(name)] = true
			value.PartyNames = append(value.PartyNames, name)
		}
	} else if key == "inventory" {
		value.InventoryCheckedAt = &checked
		if availability != "observed" || !fresh {
			return
		}
		var container struct {
			Slots []struct {
				Item struct {
					Servername string `json:"servername"`
					Quantity   int    `json:"quantity"`
				} `json:"item"`
			} `json:"slots"`
		}
		if json.Unmarshal(payload, &container) != nil {
			return
		}
		observed := false
		for _, slot := range container.Slots {
			if slot.Item.Servername == "ITEM_MALL_REVERSE_RETURN_SCROLL" && slot.Item.Quantity > 0 {
				observed = true
				break
			}
		}
		value.ScrollObserved = &observed
	}
}

func reverseReturnName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

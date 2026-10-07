package tradenexus

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
)

type activeCache struct {
	mu        sync.Mutex
	byServer  map[string]map[string]Sighting
	truncated map[string]bool
}

func newActiveCache() *activeCache {
	return &activeCache{
		byServer:  map[string]map[string]Sighting{},
		truncated: map[string]bool{},
	}
}

func activeServerKey(server string) string {
	return strings.ToLower(strings.TrimSpace(server))
}

func (c *activeCache) remember(now time.Time, sighting Sighting) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cutoff := now.Add(-ActiveTTL)
	for server, names := range c.byServer {
		for key, row := range names {
			if row.ReceivedAt.Before(cutoff) {
				delete(names, key)
			}
		}
		if len(names) == 0 {
			delete(c.byServer, server)
			delete(c.truncated, server)
		}
	}
	server := activeServerKey(sighting.Server)
	if c.byServer[server] == nil {
		c.byServer[server] = map[string]Sighting{}
	}
	nameKey := strings.ToLower(sighting.ThiefName)
	c.byServer[server][nameKey] = sighting
	if len(c.byServer[server]) <= MaxRecentSightings {
		return
	}
	rows := make([]Sighting, 0, len(c.byServer[server]))
	for _, row := range c.byServer[server] {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ReceivedAt.Equal(rows[j].ReceivedAt) {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].ReceivedAt.Before(rows[j].ReceivedAt)
	})
	c.truncated[server] = true
	kept := rows[len(rows)-MaxRecentSightings:]
	next := make(map[string]Sighting, len(kept))
	for _, row := range kept {
		next[strings.ToLower(row.ThiefName)] = row
	}
	c.byServer[server] = next
}

func (c *activeCache) snapshot(now time.Time, servers []string) ([]Sighting, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cutoff := now.Add(-ActiveTTL)
	want := make(map[string]struct{}, len(servers))
	for _, server := range servers {
		want[activeServerKey(server)] = struct{}{}
	}
	collected := make([]Sighting, 0)
	truncated := false
	for server, names := range c.byServer {
		if _, ok := want[server]; !ok {
			continue
		}
		if c.truncated[server] {
			truncated = true
		}
		for _, row := range names {
			if row.ReceivedAt.Before(cutoff) {
				continue
			}
			collected = append(collected, row)
		}
	}
	sort.Slice(collected, func(i, j int) bool {
		if collected[i].ReceivedAt.Equal(collected[j].ReceivedAt) {
			return collected[i].ID < collected[j].ID
		}
		return collected[i].ReceivedAt.Before(collected[j].ReceivedAt)
	})
	return collected, truncated
}

func marshalThievesSnapshot(sightings []Sighting, truncated bool) ([]byte, error) {
	encoded := make([]json.RawMessage, 0, len(sightings))
	for _, sighting := range sightings {
		payload, err := marshalSighting(sighting)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, json.RawMessage(payload))
	}
	return json.Marshal(map[string]any{
		"v":         ProtocolVersion,
		"type":      "thieves",
		"sightings": encoded,
		"truncated": truncated,
	})
}

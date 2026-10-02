package httpapi

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"phmon/server/internal/mapprofile"
	"phmon/server/internal/mobs"
	"phmon/server/internal/npcs"
)

const (
	maxMapNPCs       = 256
	maxNPCObservers  = 16
	npcMatchDistance = 8.0
)

type mapNPCObserver struct {
	CharacterID string `json:"character_id"`
	SessionID   string `json:"session_id"`
	Name        string `json:"name"`
}

type mapTeleportRoute struct {
	Destination  string `json:"destination"`
	TeleportCode *int64 `json:"teleport_code,omitempty"`
}

type mapNPC struct {
	ID             string             `json:"id"`
	Name           string             `json:"name,omitempty"`
	ServerName     string             `json:"servername,omitempty"`
	Model          *int64             `json:"model_id,omitempty"`
	Role           string             `json:"role"`
	Region         int                `json:"region"`
	X              float64            `json:"x"`
	Y              float64            `json:"y"`
	ObserverZ      *float64           `json:"observer_z,omitempty"`
	ObservedAt     string             `json:"observed_at"`
	TeleportRoutes []mapTeleportRoute `json:"teleport_routes,omitempty"`
	Observers      []mapNPCObserver   `json:"observers"`
	observedAt     time.Time
}

type mapNPCSnapshot struct {
	Status    string   `json:"status"`
	Truncated bool     `json:"truncated,omitempty"`
	NPCs      []mapNPC `json:"npcs"`
}

type npcCluster struct {
	item     mapNPC
	sessions map[string]struct{}
	server   string
	identity string
}

func projectNPCs(profile mapprofile.Profile, snapshots []npcs.LiveSnapshot, areaID, floorID string, regionFilter int) mapNPCSnapshot {
	areaKind := ""
	for _, area := range profile.Areas {
		if area.ID == areaID {
			areaKind = area.Kind
			break
		}
	}
	clusters := make([]npcCluster, 0)
	anyObserved := false
	anyTruncated := false
	inScope := 0
	for _, snapshot := range snapshots {
		if !npcSnapshotInScope(profile, snapshot, areaKind, areaID, floorID) {
			continue
		}
		inScope++
		if snapshot.Status == "truncated" || snapshot.Truncated {
			anyTruncated = true
		}
		if snapshot.Status == "unavailable" {
			continue
		}
		anyObserved = true
		for _, npc := range snapshot.NPCs {
			if !npcInScope(profile, snapshot, npc, areaKind, areaID, floorID, regionFilter) {
				continue
			}
			candidate := mapNPC{
				Name: npc.Name, ServerName: npc.ServerName, Model: npc.Model, Role: npc.Role,
				Region: npc.Region, X: npc.X, Y: npc.Y, ObserverZ: snapshot.ObserverZ,
				ObservedAt: snapshot.ObservedAt.UTC().Format(time.RFC3339Nano), observedAt: snapshot.ObservedAt,
				TeleportRoutes: copyMapTeleportRoutes(npc.TeleportRoutes),
				Observers: []mapNPCObserver{{
					CharacterID: snapshot.CharacterID, SessionID: snapshot.SessionID, Name: snapshot.Character,
				}},
			}
			matched := -1
			for index, cluster := range clusters {
				if _, seen := cluster.sessions[snapshot.SessionID]; seen ||
					!strings.EqualFold(cluster.server, snapshot.Server) ||
					cluster.identity != npcIdentity(npc) ||
					!mobs.RegionsMatch(cluster.item.Region, npc.Region) ||
					math.Hypot(cluster.item.X-npc.X, cluster.item.Y-npc.Y) > npcMatchDistance {
					continue
				}
				matched = index
				break
			}
			if matched < 0 {
				candidate.ID = npcMarkerID(snapshot.Server, npc)
				clusters = append(clusters, npcCluster{
					item: candidate, sessions: map[string]struct{}{snapshot.SessionID: {}},
					server: snapshot.Server, identity: npcIdentity(npc),
				})
				continue
			}
			clusters[matched].sessions[snapshot.SessionID] = struct{}{}
			current := clusters[matched].item
			if candidate.observedAt.After(current.observedAt) {
				candidate.ID = current.ID
				candidate.Observers = append(current.Observers, candidate.Observers...)
				candidate.TeleportRoutes = mergeMapTeleportRoutes(current.TeleportRoutes, candidate.TeleportRoutes)
				clusters[matched].item = candidate
			} else {
				current.Observers = append(current.Observers, candidate.Observers...)
				current.TeleportRoutes = mergeMapTeleportRoutes(current.TeleportRoutes, candidate.TeleportRoutes)
				clusters[matched].item = current
			}
		}
	}
	rows := make([]mapNPC, 0, len(clusters))
	for _, cluster := range clusters {
		item := cluster.item
		if len(item.Observers) > maxNPCObservers {
			item.Observers = item.Observers[:maxNPCObservers]
		}
		rows = append(rows, item)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].ID < rows[j].ID
	})
	truncated := anyTruncated
	if len(rows) > maxMapNPCs {
		rows = rows[:maxMapNPCs]
		truncated = true
	}
	status := "unavailable"
	if inScope > 0 && anyObserved {
		status = "observed"
	}
	if truncated {
		status = "truncated"
	}
	return mapNPCSnapshot{Status: status, Truncated: truncated, NPCs: rows}
}

func npcSnapshotInScope(profile mapprofile.Profile, snapshot npcs.LiveSnapshot, areaKind, areaID, floorID string) bool {
	region := snapshot.Region
	if areaKind == "cave" {
		area, floor, ok := mapprofile.ClassifyCave(profile, &region, snapshot.ObserverZ)
		return ok && area == areaID && floor == floorID
	}
	area, _, ok := mapprofile.ClassifyCave(profile, &region, snapshot.ObserverZ)
	return !(ok && area != "")
}

func npcInScope(profile mapprofile.Profile, snapshot npcs.LiveSnapshot, npc npcs.NPC, areaKind, areaID, floorID string, regionFilter int) bool {
	if regionFilter != 0 && !mobs.RegionsMatch(regionFilter, npc.Region) {
		return false
	}
	if areaKind != "cave" {
		return true
	}
	region := npc.Region
	area, floor, ok := mapprofile.ClassifyCave(profile, &region, snapshot.ObserverZ)
	return ok && area == areaID && floor == floorID
}

func npcIdentity(npc npcs.NPC) string {
	model := "none"
	if npc.Model != nil {
		model = fmt.Sprintf("%d", *npc.Model)
	}
	return npc.ServerName + "|" + model
}

func npcMarkerID(server string, npc npcs.NPC) string {
	model := "none"
	if npc.Model != nil {
		model = fmt.Sprintf("%d", *npc.Model)
	}
	return fmt.Sprintf("npc:%s:%d:%s:%s:%s:%.1f:%.1f", strings.ToLower(strings.TrimSpace(server)), npc.Region, npc.ServerName, model, npc.ID, npc.X, npc.Y)
}

func copyMapTeleportRoutes(routes []npcs.TeleportRoute) []mapTeleportRoute {
	if len(routes) == 0 {
		return nil
	}
	out := make([]mapTeleportRoute, 0, len(routes))
	for _, route := range routes {
		out = append(out, mapTeleportRoute{Destination: route.Destination, TeleportCode: route.TeleportCode})
	}
	return out
}

func mergeMapTeleportRoutes(existing, incoming []mapTeleportRoute) []mapTeleportRoute {
	if len(existing) == 0 {
		return incoming
	}
	if len(incoming) == 0 {
		return existing
	}
	merged := make([]mapTeleportRoute, 0, len(existing)+len(incoming))
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	for _, route := range existing {
		key := strings.ToLower(strings.TrimSpace(route.Destination))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, route)
	}
	for _, route := range incoming {
		key := strings.ToLower(strings.TrimSpace(route.Destination))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, route)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Destination < merged[j].Destination
	})
	return merged
}

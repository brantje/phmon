package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"phmon/server/internal/players"
	"phmon/server/internal/resources"
)

type playerHandler struct {
	store     *players.Store
	resources *resources.Store
	live      *LiveHub
}

func playerError(w http.ResponseWriter, err error) {
	status, message := http.StatusServiceUnavailable, "service unavailable"
	var pgError *pgconn.PgError
	switch {
	case errors.Is(err, players.ErrInvalid):
		status, message = http.StatusBadRequest, "invalid player request"
	case errors.Is(err, players.ErrNotFound):
		status, message = http.StatusNotFound, "player not found"
	case errors.Is(err, players.ErrConflict), errors.As(err, &pgError) && pgError.Code == "23505":
		status, message = http.StatusConflict, "identity conflict or stale review; refresh before retrying"
	}
	respondJSON(w, status, map[string]string{"error": message})
}
func playerPageArgs(r *http.Request) (int, string, error) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			return 0, "", players.ErrInvalid
		}
		limit = n
	}
	return limit, r.URL.Query().Get("cursor"), nil
}
func playerFilter(r *http.Request) (players.Filter, error) {
	q := r.URL.Query()
	limit, token, err := playerPageArgs(r)
	if err != nil {
		return players.Filter{}, err
	}
	allowed := map[string]bool{"server": true, "q": true, "guild": true, "min_level": true, "max_level": true, "job": true, "seen": true, "from": true, "to": true, "identity": true, "equipment": true, "sort": true, "direction": true, "limit": true, "cursor": true}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 {
			return players.Filter{}, players.ErrInvalid
		}
	}
	f := players.Filter{Server: q.Get("server"), Name: q.Get("q"), Guild: q.Get("guild"), Job: q.Get("job"), Seen: q.Get("seen"), Identity: q.Get("identity"), Equipment: q.Get("equipment"), Sort: q.Get("sort"), Direction: q.Get("direction"), Limit: limit, Cursor: token}
	for key, target := range map[string]**int{"min_level": &f.MinLevel, "max_level": &f.MaxLevel} {
		if raw := q.Get(key); raw != "" {
			value, e := strconv.Atoi(raw)
			if e != nil {
				return f, players.ErrInvalid
			}
			*target = &value
		}
	}
	for key, target := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
		if raw := q.Get(key); raw != "" {
			parsed, e := parseEventBound(raw, key == "to")
			if e != nil {
				return f, players.ErrInvalid
			}
			*target = &parsed
		}
	}
	return f, f.Validate()
}
func (h *playerHandler) list(w http.ResponseWriter, r *http.Request) {
	f, err := playerFilter(r)
	if err != nil {
		playerError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.List(ctx, f)
	if err != nil {
		playerError(w, err)
		return
	}
	respondJSON(w, 200, page)
}
func (h *playerHandler) servers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	servers, err := h.store.Servers(ctx)
	if err != nil {
		playerError(w, err)
		return
	}
	respondJSON(w, 200, map[string]any{"servers": servers})
}
func (h *playerHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	record, err := h.store.Get(ctx, r.PathValue("id"))
	if r.URL.Query().Get("source") == "1" {
		record, err = h.store.GetSource(ctx, r.PathValue("id"))
	}
	if err != nil {
		playerError(w, err)
		return
	}
	h.enrich(record.Server, record.Gear)
	respondJSON(w, 200, record)
}
func (h *playerHandler) equipment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	record, err := h.store.Get(ctx, r.PathValue("id"))
	if err != nil {
		playerError(w, err)
		return
	}
	h.enrich(record.Server, record.Gear)
	respondJSON(w, 200, map[string]any{"equipment": record.Gear, "gear_hash": record.GearHash, "identity_gear_hash": record.IdentityGearHash})
}
func (h *playerHandler) enrich(server string, equipment *players.Equipment) {
	if equipment == nil {
		return
	}
	for i := range equipment.Slots {
		slot := &equipment.Slots[i]
		if slot.State != "occupied" {
			continue
		}
		item := map[string]any{}
		for key, value := range slot.Item {
			item[key] = value
		}
		if slot.ModelID != nil {
			item["model"] = *slot.ModelID
		}
		if slot.Plus != nil {
			item["plus"] = *slot.Plus
		}
		if slot.Durability != nil {
			item["durability"] = *slot.Durability
		}
		if h.resources != nil {
			item = h.resources.EnrichItemRecord(server, item)
		}
		slot.Item = item
	}
}
func (h *playerHandler) history(w http.ResponseWriter, r *http.Request) {
	limit, cursor, err := playerPageArgs(r)
	if err != nil {
		playerError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.EquipmentHistory(ctx, r.PathValue("id"), limit, cursor)
	if err != nil {
		playerError(w, err)
		return
	}
	record, err := h.store.Get(ctx, r.PathValue("id"))
	if err != nil {
		playerError(w, err)
		return
	}
	for i := range page.Items {
		h.enrich(record.Server, &page.Items[i].Equipment)
	}
	respondJSON(w, 200, page)
}
func (h *playerHandler) aliases(w http.ResponseWriter, r *http.Request) {
	limit, cursor, err := playerPageArgs(r)
	if err != nil {
		playerError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.Aliases(ctx, r.PathValue("id"), limit, cursor)
	if err != nil {
		playerError(w, err)
		return
	}
	respondJSON(w, 200, page)
}
func (h *playerHandler) observations(w http.ResponseWriter, r *http.Request) {
	limit, cursor, err := playerPageArgs(r)
	if err != nil {
		playerError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.Observations(ctx, r.PathValue("id"), limit, cursor)
	if err != nil {
		playerError(w, err)
		return
	}
	respondJSON(w, 200, page)
}
func (h *playerHandler) candidates(w http.ResponseWriter, r *http.Request) {
	limit, cursor, err := playerPageArgs(r)
	if err != nil {
		playerError(w, err)
		return
	}
	q := r.URL.Query()
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.Links(ctx, q.Get("server"), q.Get("player_id"), q.Get("status"), limit, cursor)
	if err != nil {
		playerError(w, err)
		return
	}
	respondJSON(w, 200, page)
}
func readPlayerJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return players.ErrInvalid
	}
	if decoder.Decode(new(any)) != io.EOF {
		return players.ErrInvalid
	}
	return nil
}
func playerActor(r *http.Request) string {
	if _, ok := operatorSessionFromContext(r.Context()); ok {
		return "operator"
	}
	return "test_operator"
}
func (h *playerHandler) decide(w http.ResponseWriter, r *http.Request) {
	var body players.LinkRequest
	if err := readPlayerJSON(w, r, &body); err != nil {
		playerError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	id, err := h.store.Decide(ctx, body, playerActor(r))
	if err != nil {
		playerError(w, err)
		return
	}
	respondJSON(w, 201, map[string]string{"id": id})
	if h.live != nil {
		h.live.Invalidate()
	}
}
func (h *playerHandler) unlink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Confirmed bool   `json:"confirmed"`
		Reason    string `json:"reason"`
		Revision  int64  `json:"revision"`
	}
	if err := readPlayerJSON(w, r, &body); err != nil || !body.Confirmed {
		playerError(w, players.ErrInvalid)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := h.store.Unlink(ctx, r.PathValue("id"), body.Revision, body.Reason, playerActor(r)); err != nil {
		playerError(w, err)
		return
	}
	if h.live != nil {
		h.live.Invalidate()
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *playerHandler) classify(w http.ResponseWriter, r *http.Request) {
	var body players.Classification
	if err := readPlayerJSON(w, r, &body); err != nil {
		playerError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := h.store.Classify(ctx, r.PathValue("id"), body, playerActor(r)); err != nil {
		playerError(w, err)
		return
	}
	if h.live != nil {
		h.live.Invalidate()
	}
	w.WriteHeader(http.StatusNoContent)
}

func registerPlayers(register func(string, bool, http.HandlerFunc), store *players.Store, resources *resources.Store, live *LiveHub) {
	if store == nil {
		return
	}
	h := &playerHandler{store: store, resources: resources, live: live}
	register("GET /api/players", false, h.list)
	register("GET /api/players/servers", false, h.servers)
	register("GET /api/players/match-candidates", false, h.candidates)
	register("GET /api/players/{id}", false, h.get)
	register("GET /api/players/{id}/equipment", false, h.equipment)
	register("GET /api/players/{id}/equipment/history", false, h.history)
	register("GET /api/players/{id}/aliases", false, h.aliases)
	register("GET /api/players/{id}/observations", false, h.observations)
	register("POST /api/players/links", true, h.decide)
	register("DELETE /api/players/links/{id}", true, h.unlink)
	register("POST /api/players/{id}/aliases", true, h.classify)
}

// Attach profile IDs without changing runtime IDs or live marker deduplication.
func (h *LiveHub) registryPlayerSnapshot(ctx context.Context, server string, snapshot mapPlayerSnapshot) mapPlayerSnapshot {
	if h.playerRegistry == nil {
		return snapshot
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	names := map[string][]string{}
	for _, p := range snapshot.Players {
		key := server
		if p.server != "" {
			key = p.server
		}
		names[key] = append(names[key], p.Name)
	}
	ids := map[string]map[string]string{}
	for key, values := range names {
		resolved, err := h.playerRegistry.ResolveNames(ctx, key, values)
		if err == nil {
			ids[key] = resolved
		}
	}
	for i := range snapshot.Players {
		key := server
		if snapshot.Players[i].server != "" {
			key = snapshot.Players[i].server
		}
		snapshot.Players[i].RegistryID = ids[key][strings.ToLower(snapshot.Players[i].Name)]
	}
	return snapshot
}

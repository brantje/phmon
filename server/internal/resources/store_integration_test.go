package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
)

func TestResourceSnapshotsPersistFencingAndFreshness(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run resource integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	if err := agents.NewStore(pool).CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	server := "slice4-resource-" + credential.AgentID[:8]
	guild := "resource-guild"
	charactersStore := characters.NewStore(pool)
	characterID, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: "owner", Guild: &guild})
	if err != nil {
		t.Fatal(err)
	}
	observerID, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: "observer", Guild: &guild})
	if err != nil {
		t.Fatal(err)
	}
	cleanup := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(cleanup, `DELETE FROM characters WHERE server_key=$1`, strings.ToLower(server))
		_, _ = pool.Exec(cleanup, `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	ownerSession, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	observerSession, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, observerID, 1)
	if err != nil {
		t.Fatal(err)
	}
	state := characters.State{}
	if err := charactersStore.SnapshotSession(ctx, credential.AgentID, characterID, 1, ownerSession, state); err != nil {
		t.Fatal(err)
	}
	if err := charactersStore.SnapshotSession(ctx, credential.AgentID, observerID, 1, observerSession, state); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	ownerFull := Snapshot{Revision: 1, Full: true, Resources: map[string]json.RawMessage{
		"inventory":       json.RawMessage(`{"availability":"observed","capacity":2,"used_slots":1,"slots":[{"source_slot":13,"displayed_slot":0,"item":{"model":7,"name":"Stack","servername":"ITEM_STACK","quantity":9007199254740993}} ,null]}`),
		"storage":         json.RawMessage(`{"availability":"not_observed","reason":"getter_unavailable_or_not_open"}`),
		"guild_storage":   json.RawMessage(`{"availability":"observed","gold":4000,"capacity":1,"used_slots":1,"slots":[{"source_slot":0,"displayed_slot":0,"item":{"model":8,"quantity":3}}]}`),
		"item_enrichment": json.RawMessage(`{"availability":"unavailable","protocol":"unknown","reason":"passive_item_packet_decoder_not_enabled"}`),
	}}
	if err := store.Apply(ctx, credential.AgentID, characterID, 1, 1, ownerSession, ownerFull); err != nil {
		t.Fatal(err)
	}
	contexts, err := store.ReverseReturnContexts(ctx, map[string]string{characterID: ownerSession, observerID: observerSession}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if contexts[characterID].SessionID != ownerSession || contexts[characterID].ScrollObserved == nil || *contexts[characterID].ScrollObserved {
		t.Fatalf("current inventory evidence: %#v", contexts)
	}
	mismatch, err := store.ReverseReturnContexts(ctx, map[string]string{characterID: observerSession}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if mismatch[characterID].ScrollObserved != nil {
		t.Fatal("resource context crossed a session boundary")
	}
	first, err := store.Character(ctx, characterID)
	if err != nil {
		t.Fatal(err)
	}
	firstInventory := first.Resources["inventory"].ObservedAt
	if firstInventory == "" || first.Resources["storage"].Availability != "not_observed" {
		t.Fatalf("first observation freshness/availability = %+v", first.Resources)
	}

	// A delta only updates its named container; untouched observations remain durable.
	if err := store.Apply(ctx, credential.AgentID, characterID, 1, 2, ownerSession, Snapshot{Revision: 2, BaseRevision: 1, Resources: map[string]json.RawMessage{
		"storage":         json.RawMessage(`{"availability":"observed","capacity":1,"used_slots":1,"slots":[{"source_slot":0,"displayed_slot":0,"item":{"model":9,"quantity":4}}]}`),
		"item_enrichment": json.RawMessage(`{"availability":"not_observed","protocol":"vsro-1.188","source":"vsro_1188_packet","decoder_build":"vsro_1188_passive_r1","observed_items":0,"reason":"session_start"}`),
	}}); err != nil {
		t.Fatal(err)
	}
	updated, err := store.Character(ctx, characterID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Resources["inventory"].Availability != "observed" || updated.Resources["storage"].Availability != "observed" {
		t.Fatalf("delta replaced untouched container state: %+v", updated.Resources)
	}
	var enrichment map[string]any
	if err := json.Unmarshal(updated.Resources["item_enrichment"].Payload, &enrichment); err != nil {
		t.Fatal(err)
	}
	if updated.Resources["item_enrichment"].Availability != "not_observed" || enrichment["decoder_build"] != "vsro_1188_passive_r1" || enrichment["protocol"] != "vsro-1.188" {
		t.Fatalf("latest parser diagnostic was not persisted: availability=%s payload=%v", updated.Resources["item_enrichment"].Availability, enrichment)
	}

	// Repeated identical contents update status freshness without making the
	// original item contents look newly observed.
	time.Sleep(5 * time.Millisecond)
	if err := store.Apply(ctx, credential.AgentID, characterID, 1, 3, ownerSession, Snapshot{Revision: 3, Full: true, Resources: map[string]json.RawMessage{
		"inventory":     ownerFull.Resources["inventory"],
		"storage":       updated.Resources["storage"].Payload,
		"guild_storage": ownerFull.Resources["guild_storage"],
	}}); err != nil {
		t.Fatal(err)
	}
	updated, err = store.Character(ctx, characterID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Resources["inventory"].ObservedAt != firstInventory {
		t.Fatalf("identical cached contents advanced observed_at: %s -> %s", firstInventory, updated.Resources["inventory"].ObservedAt)
	}

	// A closed/unopened storage observation retains its last confirmed payload,
	// marks the source unavailable and leaves the contents timestamp unchanged.
	if err := store.Apply(ctx, credential.AgentID, characterID, 1, 4, ownerSession, Snapshot{Revision: 4, BaseRevision: 3, Resources: map[string]json.RawMessage{
		"storage": json.RawMessage(`{"availability":"not_observed","reason":"getter_unavailable_or_not_open"}`),
	}}); err != nil {
		t.Fatal(err)
	}
	updated, err = NewStore(pool).Character(ctx, characterID)
	if err != nil {
		t.Fatal(err)
	}
	var storagePayload map[string]any
	if err := json.Unmarshal(updated.Resources["storage"].Payload, &storagePayload); err != nil {
		t.Fatal(err)
	}
	if updated.Resources["storage"].Availability != "not_observed" || len(storagePayload["slots"].([]any)) != 1 {
		t.Fatalf("last-known storage was not retained and marked: %+v payload=%v", updated.Resources["storage"], storagePayload)
	}

	// A second guild observer's fresh contents remain authoritative when the
	// first observer repeats a cached poll with older observed_at evidence.
	if err := store.Apply(ctx, credential.AgentID, observerID, 1, 1, observerSession, Snapshot{Revision: 1, Full: true, Resources: map[string]json.RawMessage{
		"guild_storage": json.RawMessage(`{"availability":"observed","gold":10000,"capacity":1,"used_slots":1,"slots":[{"source_slot":0,"displayed_slot":0,"item":{"model":99,"quantity":1}}]}`),
	}}); err != nil {
		t.Fatal(err)
	}
	var observerCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM guild_gold_samples WHERE server_key=$1 AND guild_key=$2`, strings.ToLower(server), guild).Scan(&observerCount); err != nil || observerCount != 2 {
		t.Fatalf("guild balance must retain one observation per observer rather than summing, count=%d err=%v", observerCount, err)
	}
	if err := store.Apply(ctx, credential.AgentID, characterID, 1, 5, ownerSession, Snapshot{Revision: 5, BaseRevision: 4, Resources: map[string]json.RawMessage{
		"guild_storage": ownerFull.Resources["guild_storage"],
	}}); err != nil {
		t.Fatal(err)
	}
	guildRows, err := store.GuildStorage(ctx, server, guild)
	if err != nil {
		t.Fatal(err)
	}
	var newestGuild struct {
		Slots []struct {
			Item struct {
				Model json.Number `json:"model"`
			} `json:"item"`
		} `json:"slots"`
	}
	if len(guildRows) == 1 {
		decoder := json.NewDecoder(bytes.NewReader(guildRows[0].Payload))
		decoder.UseNumber()
		if err := decoder.Decode(&newestGuild); err != nil {
			t.Fatal(err)
		}
	}
	if len(guildRows) != 1 || len(newestGuild.Slots) != 1 || newestGuild.Slots[0].Item.Model.String() != "99" || guildRows[0].ObserverName != "observer" || guildRows[0].ObserverCharacterID != observerID {
		t.Fatalf("stale observer replaced newer guild evidence: %+v", guildRows)
	}
	deletedObservations, deletedItems, deletedGoldSamples, err := store.DeleteGuildStorage(ctx, server, guild)
	if err != nil || deletedObservations != 2 || deletedItems != 2 || deletedGoldSamples != 2 {
		t.Fatalf("guild storage purge observations=%d items=%d gold samples=%d err=%v", deletedObservations, deletedItems, deletedGoldSamples, err)
	}
	guildRows, err = store.GuildStorage(ctx, server, guild)
	if err != nil || len(guildRows) != 0 {
		t.Fatalf("guild storage rows remain after purge: rows=%+v err=%v", guildRows, err)
	}
	if err := store.Apply(ctx, credential.AgentID, characterID, 1, 6, ownerSession, Snapshot{Revision: 6, BaseRevision: 5, Resources: map[string]json.RawMessage{
		"guild_storage": ownerFull.Resources["guild_storage"],
	}}); err != nil {
		t.Fatal(err)
	}
	guildRows, err = store.GuildStorage(ctx, server, guild)
	if err != nil || len(guildRows) != 1 {
		t.Fatalf("fresh phBot observation did not repopulate a cleared scope: rows=%+v err=%v", guildRows, err)
	}
	var repopulatedGoldSamples int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM guild_gold_samples WHERE server_key=$1 AND guild_key=$2`, strings.ToLower(server), guild).Scan(&repopulatedGoldSamples); err != nil || repopulatedGoldSamples != 1 {
		t.Fatalf("fresh observation did not create fresh guild balance history, count=%d err=%v", repopulatedGoldSamples, err)
	}

	var quantity string
	if err := pool.QueryRow(ctx, `SELECT quantity::text FROM character_resource_items WHERE observer_character_id=$1 AND container_key='inventory' AND source_slot=13`, characterID).Scan(&quantity); err != nil {
		t.Fatal(err)
	}
	if quantity != "9007199254740993" {
		t.Fatalf("large quantity lost precision in normalized item row: %s", quantity)
	}

	// Reconnect on a new connection generation starts a new baseline revision at 1.
	if err := charactersStore.EndSession(ctx, credential.AgentID, characterID, 1, ownerSession, "agent_disconnected"); err != nil {
		t.Fatal(err)
	}
	newSession, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, characterID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := charactersStore.SnapshotSession(ctx, credential.AgentID, characterID, 2, newSession, state); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(ctx, credential.AgentID, characterID, 2, 1, newSession, Snapshot{Revision: 1, Full: true, Resources: map[string]json.RawMessage{
		"inventory": json.RawMessage(`{"availability":"observed","capacity":0,"used_slots":0,"slots":[]}`),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(ctx, credential.AgentID, characterID, 1, 6, ownerSession, Snapshot{Revision: 6, Full: true, Resources: map[string]json.RawMessage{
		"inventory": json.RawMessage(`{"availability":"observed","capacity":0,"used_slots":0,"slots":[]}`),
	}}); !errors.Is(err, ErrStale) {
		t.Fatalf("prior session update error = %v", err)
	}
}

func TestCurrentPartyObservationsFenceSessionAndAvailability(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run resource integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	if err := agents.NewStore(pool).CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	server := "party-resource-" + credential.AgentID[:8]
	charactersStore := characters.NewStore(pool)
	characterID, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: "observer"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM characters WHERE server_key=$1`, strings.ToLower(server))
		_, _ = pool.Exec(cleanup, `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	sessionID, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	region, z := 25273, 5.0
	if err := charactersStore.SnapshotSession(ctx, credential.AgentID, characterID, 1, sessionID, characters.State{Region: &region, Z: &z}); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	party := json.RawMessage(`{"availability":"observed","members":[{"party_id":"55","player_id":700,"name":"Ally","guild":"Guild","level":110,"hp_percent":80,"mp_percent":90,"x":123.5,"y":-456.25}]}`)
	if err := store.Apply(ctx, credential.AgentID, characterID, 1, 1, sessionID, Snapshot{Revision: 1, Full: true, Resources: map[string]json.RawMessage{"party": party}}); err != nil {
		t.Fatal(err)
	}
	rows, truncated, err := store.CurrentPartyObservations(ctx, server)
	if err != nil || truncated || len(rows) != 1 || len(rows[0].Members) != 1 {
		t.Fatalf("current party observations rows=%+v truncated=%v err=%v", rows, truncated, err)
	}
	member := rows[0].Members[0]
	if rows[0].SessionID != sessionID || rows[0].ObserverRegion == nil || *rows[0].ObserverRegion != region ||
		member.PlayerID != 700 || member.X == nil || *member.X != 123.5 || member.HPPercent == nil || *member.HPPercent != 80 {
		t.Fatalf("current party observation lost bounded fields or scope: row=%+v member=%+v", rows[0], member)
	}

	if err := store.Apply(ctx, credential.AgentID, characterID, 1, 2, sessionID, Snapshot{Revision: 2, BaseRevision: 1, Resources: map[string]json.RawMessage{
		"party": json.RawMessage(`{"availability":"observed","members":[]}`),
	}}); err != nil {
		t.Fatal(err)
	}
	rows, _, err = store.CurrentPartyObservations(ctx, server)
	if err != nil || len(rows) != 1 || rows[0].Availability != "observed" || len(rows[0].Members) != 0 {
		t.Fatalf("observed empty party did not clear members: rows=%+v err=%v", rows, err)
	}

	if err := store.Apply(ctx, credential.AgentID, characterID, 1, 3, sessionID, Snapshot{Revision: 3, BaseRevision: 2, Resources: map[string]json.RawMessage{
		"party": json.RawMessage(`{"availability":"unavailable","members":[]}`),
	}}); err != nil {
		t.Fatal(err)
	}
	rows, _, err = store.CurrentPartyObservations(ctx, server)
	if err != nil || len(rows) != 1 || rows[0].Availability != "unavailable" || len(rows[0].Members) != 0 {
		t.Fatalf("unavailable party retained stale payload members: rows=%+v err=%v", rows, err)
	}

	if err := charactersStore.EndSession(ctx, credential.AgentID, characterID, 1, sessionID, "agent_disconnected"); err != nil {
		t.Fatal(err)
	}
	rows, _, err = store.CurrentPartyObservations(ctx, server)
	if err != nil || len(rows) != 0 {
		t.Fatalf("ended session retained party observation: rows=%+v err=%v", rows, err)
	}

	newSession, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, characterID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := charactersStore.SnapshotSession(ctx, credential.AgentID, characterID, 2, newSession, characters.State{Region: &region, Z: &z}); err != nil {
		t.Fatal(err)
	}
	rows, _, err = store.CurrentPartyObservations(ctx, server)
	if err != nil || len(rows) != 0 {
		t.Fatalf("replacement session reused prior party row before new baseline: rows=%+v err=%v", rows, err)
	}
	if err := store.Apply(ctx, credential.AgentID, characterID, 2, 1, newSession, Snapshot{Revision: 1, Full: true, Resources: map[string]json.RawMessage{"party": party}}); err != nil {
		t.Fatal(err)
	}
	rows, _, err = store.CurrentPartyObservations(ctx, server)
	if err != nil || len(rows) != 1 || rows[0].SessionID != newSession || len(rows[0].Members) != 1 {
		t.Fatalf("replacement session party baseline unavailable: rows=%+v err=%v", rows, err)
	}
}

func TestGuildStorageGoldDistinctPerGuild(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run resource integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	if err := agents.NewStore(pool).CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	serverA := "guild-gold-a-" + credential.AgentID[:8]
	serverB := "guild-gold-b-" + credential.AgentID[:8]
	guildOne := "AlphaGuild"
	guildTwo := "BetaGuild"
	charactersStore := characters.NewStore(pool)
	applyGuild := func(name, guild string, server string, gold int64, revision uint64, full bool, base uint64) (string, string, error) {
		g := guild
		id, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: name, Guild: &g})
		if err != nil {
			return "", "", err
		}
		session, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, id, 1)
		if err != nil {
			return "", "", err
		}
		if err := charactersStore.SnapshotSession(ctx, credential.AgentID, id, 1, session, characters.State{}); err != nil {
			return "", "", err
		}
		snapshot := Snapshot{Revision: revision, Full: full, BaseRevision: base, Resources: map[string]json.RawMessage{
			"guild_storage": json.RawMessage(`{"availability":"observed","gold":` + strconv.FormatInt(gold, 10) + `,"capacity":1,"used_slots":0,"slots":[]}`),
		}}
		if err := NewStore(pool).Apply(ctx, credential.AgentID, id, 1, revision, session, snapshot); err != nil {
			return "", "", err
		}
		return id, session, nil
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM characters WHERE server_key IN ($1,$2)`, strings.ToLower(serverA), strings.ToLower(serverB))
		_, _ = pool.Exec(cleanup, `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	_, _, err = applyGuild("owner", guildOne, serverA, 4000, 1, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	observerID, observerSession, err := applyGuild("observer", guildOne, serverA, 9000, 1, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := NewStore(pool).Apply(ctx, credential.AgentID, observerID, 1, 2, observerSession, Snapshot{
		Revision: 2, BaseRevision: 1, Resources: map[string]json.RawMessage{
			"guild_storage": json.RawMessage(`{"availability":"observed","gold":10000,"capacity":1,"used_slots":0,"slots":[]}`),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = applyGuild("beta", guildTwo, serverA, 2500, 1, true, 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err = applyGuild("remote", guildOne, serverB, 777, 1, true, 0); err != nil {
		t.Fatal(err)
	}
	negativeGuild := "NoGold"
	if _, _, err = applyGuild("nogold", negativeGuild, serverA, -5, 1, true, 0); err != nil {
		t.Fatal(err)
	}
	entries, err := NewStore(pool).GuildStorageGold(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]int64{}
	for _, entry := range entries {
		key := strings.ToLower(entry.Server) + "\u0000" + strings.ToLower(entry.Guild)
		byKey[key] = entry.Gold
	}
	if byKey[strings.ToLower(serverA)+"\u0000"+strings.ToLower(guildOne)] != 10000 {
		t.Fatalf("expected newest observer gold for one guild, got %#v", entries)
	}
	if byKey[strings.ToLower(serverA)+"\u0000"+strings.ToLower(guildTwo)] != 2500 {
		t.Fatalf("expected second guild gold, got %#v", entries)
	}
	if byKey[strings.ToLower(serverB)+"\u0000"+strings.ToLower(guildOne)] != 777 {
		t.Fatalf("expected other-server gold, got %#v", entries)
	}
	if _, exists := byKey[strings.ToLower(serverA)+"\u0000"+strings.ToLower(negativeGuild)]; exists {
		t.Fatalf("negative guild gold must be omitted: %#v", entries)
	}
}

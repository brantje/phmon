package events

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
)

func TestGuildStorageContainerReferences(t *testing.T) {
	if !validContainerReference(map[string]any{"type": "guild_storage", "id": "guild", "slot": float64(12)}) {
		t.Fatal("observed guild storage slot should be a valid container reference")
	}
	if validContainerReference(map[string]any{"type": "guild_storage", "id": "guild", "slot": float64(-1)}) {
		t.Fatal("negative guild storage slot must remain invalid")
	}
	if validContainerReference(map[string]any{"type": "guild_storage"}) {
		t.Fatal("missing guild scope must remain invalid")
	}
	if validContainerReference(map[string]any{"type": "guild_storage", "id": ""}) {
		t.Fatal("empty guild storage id must remain invalid")
	}
}

func TestDeathEventsAreDurableIdempotentAndScoped(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run event integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
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
	server := "events-" + credential.AgentID
	characterID, err := characters.NewStore(pool).Resolve(ctx, characters.Identity{Server: server, Name: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := characters.NewStore(pool).ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	var databaseNow time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&databaseNow); err != nil {
		t.Fatal(err)
	}
	baseTime := databaseNow.UTC().Add(-15 * time.Minute).Truncate(time.Microsecond)
	if _, err := pool.Exec(ctx, `UPDATE character_sessions SET started_at=$2::timestamptz-interval '30 minutes',ended_at=$2::timestamptz-interval '10 minutes',end_reason='left' WHERE session_id=$1`, sessionID, databaseNow); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM activity_events WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})

	store := NewStore(pool)
	first := AgentDeath{
		ID: credential.AgentID, OccurredAt: baseTime,
		Source: "phbot.callback", SourceRef: "EVENT_DIED",
		Region: intPointer(25273), X: floatPointer(10), Y: floatPointer(20), Z: floatPointer(30),
		Payload: json.RawMessage(`{"cause":"unknown"}`),
	}
	inserted, err := store.AppendDeath(ctx, credential.AgentID, characterID, sessionID, first)
	if err != nil || !inserted {
		t.Fatalf("first event insert = %v, %v", inserted, err)
	}
	inserted, err = store.AppendDeath(ctx, credential.AgentID, characterID, sessionID, first)
	if err != nil || inserted {
		t.Fatalf("idempotent retry = %v, %v", inserted, err)
	}
	newSessionID, err := characters.NewStore(pool).ClaimSessionID(ctx, credential.AgentID, characterID, 2)
	if err != nil {
		t.Fatal(err)
	}
	var newSessionStarted time.Time
	if err := pool.QueryRow(ctx, `SELECT started_at FROM character_sessions WHERE session_id=$1`, newSessionID).Scan(&newSessionStarted); err != nil {
		t.Fatal(err)
	}
	invalidProjection := AgentEvent{
		ID: newTestEventID(t), Schema: 1, Kind: "chat.message_received", Category: "chat",
		CharacterID: characterID, SessionID: newSessionID, Server: server, Character: "Alpha",
		OccurredAt: databaseNow.UTC(), Source: "phbot.chat_callback", SourceRef: "handle_chat",
		Sequence: int64Pointer(100), Payload: json.RawMessage(`{"channel":"private","raw_type":"2","message":"invalid direction","sender":"Veyra","recipient":"Alpha","direction":"outbound"}`),
	}
	projectionTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, _, projectionErr := appendOne(ctx, projectionTx, credential.AgentID, invalidProjection, nil)
	if !errors.Is(projectionErr, ErrInvalidEvent) {
		_ = projectionTx.Rollback(ctx)
		t.Fatalf("invalid chat projection error = %v", projectionErr)
	}
	var invalidEventRows int
	if err := projectionTx.QueryRow(ctx, `SELECT count(*) FROM activity_events WHERE event_id=$1::uuid`, invalidProjection.ID).Scan(&invalidEventRows); err != nil {
		_ = projectionTx.Rollback(ctx)
		t.Fatal(err)
	}
	if invalidEventRows != 0 {
		_ = projectionTx.Rollback(ctx)
		t.Fatalf("invalid chat event left %d canonical activity rows", invalidEventRows)
	}
	if err := projectionTx.Commit(ctx); err != nil {
		t.Fatalf("transaction should remain usable after invalid projection: %v", err)
	}
	if inserted, err = store.AppendDeath(ctx, credential.AgentID, characterID, sessionID, first); err != nil || inserted {
		t.Fatalf("delayed replay from prior owned session = %v, %v", inserted, err)
	}
	conflicting := first
	conflicting.Payload = json.RawMessage(`{"cause":"different"}`)
	if _, err := store.AppendDeath(ctx, credential.AgentID, characterID, sessionID, conflicting); !errors.Is(err, ErrEventConflict) {
		t.Fatalf("same event id with different content: %v", err)
	}
	unowned := first
	unowned.ID = newTestEventID(t)
	unowned.OccurredAt = databaseNow.UTC()
	if _, err := store.AppendDeath(ctx, credential.AgentID, characterID, "00000000-0000-4000-8000-000000000199", unowned); !errors.Is(err, ErrUnauthorizedSession) {
		t.Fatalf("unowned session accepted: %v", err)
	}
	stale := first
	stale.ID = newTestEventID(t)
	stale.OccurredAt = newSessionStarted.UTC().Add(time.Second)
	if _, err := store.AppendDeath(ctx, credential.AgentID, characterID, sessionID, stale); !errors.Is(err, ErrUnauthorizedSession) {
		t.Fatalf("old session reported a post-session occurrence: %v", err)
	}
	deferred := first
	deferred.ID = newTestEventID(t)
	deferred.OccurredAt = baseTime.Add(-30 * time.Minute)
	deferred.Payload = json.RawMessage(`{"cause":"unknown","session_binding":"deferred"}`)
	if inserted, err := store.AppendDeath(ctx, credential.AgentID, characterID, newSessionID, deferred); err != nil || !inserted {
		t.Fatalf("deferred callback outside the new session window = %v, %v", inserted, err)
	}
	otherCharacterID, err := characters.NewStore(pool).Resolve(ctx, characters.Identity{Server: server, Name: "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	wrongCharacter := first
	wrongCharacter.ID = newTestEventID(t)
	wrongCharacter.OccurredAt = databaseNow.UTC()
	if _, err := store.AppendDeath(ctx, credential.AgentID, otherCharacterID, sessionID, wrongCharacter); !errors.Is(err, ErrUnauthorizedSession) {
		t.Fatalf("session was accepted for another character: %v", err)
	}

	second := first
	second.ID = newTestEventID(t)
	second.OccurredAt = databaseNow.UTC().Truncate(time.Microsecond)
	if inserted, err = store.AppendDeath(ctx, credential.AgentID, characterID, newSessionID, second); err != nil || !inserted {
		t.Fatalf("second event insert = %v, %v", inserted, err)
	}
	from := baseTime.Add(-31 * time.Minute)
	to := second.OccurredAt.Add(time.Minute)
	page, err := store.List(ctx, Filter{Server: server, CharacterID: characterID, From: &from, To: &to, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Events) != 1 || page.Events[0].ID != second.ID || page.NextCursor == "" {
		t.Fatalf("first filtered page: %+v", page)
	}
	page, err = store.List(ctx, Filter{Server: server, CharacterID: characterID, From: &from, To: &to, Cursor: page.NextCursor, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Events) != 1 || page.Events[0].ID != first.ID || page.NextCursor == "" {
		t.Fatalf("second filtered page: %+v", page)
	}
	page, err = store.List(ctx, Filter{Server: server, CharacterID: characterID, From: &from, To: &to, Cursor: page.NextCursor, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Events) != 1 || page.Events[0].ID != deferred.ID || page.NextCursor != "" {
		t.Fatalf("third filtered page: %+v", page)
	}
	mapRegion := 25273
	mapPage, err := store.List(ctx, Filter{Server: server, Kind: DeathKind, Region: &mapRegion, RequireMapPosition: true, Limit: MaxPageSize})
	if err != nil || mapPage.Total != 3 || len(mapPage.Events) != 3 {
		t.Fatalf("map-location event filters = %+v, err=%v", mapPage, err)
	}
	page, err = store.List(ctx, Filter{Server: "different-server", Limit: 10})
	if err != nil || page.Total != 0 {
		t.Fatalf("server filter leaked events: page=%+v err=%v", page, err)
	}

	model := int64(777)
	sequence := int64(1)
	rareDrop := AgentEvent{
		ID: newTestEventID(t), Schema: 1, Kind: "drop.rare", Category: "drop",
		CharacterID: characterID, SessionID: newSessionID, Server: server, Character: "Alpha",
		OccurredAt: databaseNow.UTC(), Sequence: &sequence, Source: "phbot.callback", SourceRef: "EVENT_RARE_DROP",
		ItemModel: &model, Zone: "Jangan", Payload: json.RawMessage(`{"model":777}`),
	}
	unauthorized := rareDrop
	unauthorized.ID = newTestEventID(t)
	unauthorized.Sequence = int64Pointer64(2)
	unauthorized.SessionID = "00000000-0000-4000-8000-000000000199"
	results, changed, err := store.AppendBatch(ctx, credential.AgentID, []AgentEvent{rareDrop, unauthorized})
	if err != nil || !changed || len(results) != 2 || results[0].Status != "persisted" || results[1].Status != "rejected" {
		t.Fatalf("batch result = %+v, changed=%v, err=%v", results, changed, err)
	}
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{rareDrop})
	if err != nil || changed || results[0].Status != "persisted" {
		t.Fatalf("idempotent batch replay = %+v, changed=%v, err=%v", results, changed, err)
	}
	exact, err := store.List(ctx, Filter{Server: server, EventID: rareDrop.ID, Limit: 1})
	if err != nil || exact.Total != 1 || len(exact.Events) != 1 || exact.Events[0].ID != rareDrop.ID {
		t.Fatalf("exact event lookup = %+v, err=%v", exact, err)
	}
	store.SetItemNameMatcher(func(query string) []string {
		if query == "Catalog Necklace" {
			return []string{strings.ToLower(server) + ":777"}
		}
		return nil
	})
	byName, nameErr := store.List(ctx, Filter{Server: server, Kind: "drop.rare", ItemQuery: "Catalog Necklace", Limit: 10})
	if nameErr != nil || byName.Total != 1 || len(byName.Events) != 1 || byName.Events[0].ID != rareDrop.ID {
		t.Fatalf("model-only callback must be searchable by its catalog name: %+v, %v", byName, nameErr)
	}
	observedItem := AgentEvent{
		ID: newTestEventID(t), Schema: 1, Kind: "item.quantity_increased", Category: "item",
		CharacterID: characterID, SessionID: newSessionID, Server: server, Character: "Alpha",
		OccurredAt: databaseNow.UTC().Add(2 * time.Second), Sequence: int64Pointer64(50),
		Source: "phbot.state_diff", SourceRef: "item_container", ItemModel: &model,
		Payload: json.RawMessage(`{"item":{"model":777,"servername":"ITEM_TEST","plus":3,"api_fields":{"blues":{"9":5}}},"quantity_delta":1,"destination_container":{"type":"inventory","slot":13},"acquisition_method":"unknown","drop_event_id":"` + rareDrop.ID + `"}`),
	}
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{observedItem})
	if err != nil || !changed || len(results) != 1 || results[0].Status != "persisted" {
		t.Fatalf("linked inventory observation = %+v, changed=%v, err=%v", results, changed, err)
	}
	exact, err = store.List(ctx, Filter{Server: server, EventID: rareDrop.ID, Limit: 1})
	if err != nil || len(exact.Events) != 1 {
		t.Fatalf("linked drop lookup = %+v, err=%v", exact, err)
	}
	var linkedPayload struct {
		Item        json.RawMessage `json:"item"`
		Observation struct {
			Association string `json:"association"`
			EventID     string `json:"event_id"`
		} `json:"item_observation"`
	}
	if err := json.Unmarshal(exact.Events[0].Payload, &linkedPayload); err != nil ||
		len(linkedPayload.Item) != 0 ||
		linkedPayload.Observation.Association != "stored_unverified_drop_event_id" ||
		linkedPayload.Observation.EventID != observedItem.ID {
		t.Fatalf("historical association must be auditable but cannot lend instance attributes: %s, err=%v", exact.Events[0].Payload, err)
	}
	if related := exact.Events[0].RelatedItemObservation; related == nil ||
		related.EventID != observedItem.ID || related.Association != "stored_unverified_drop_event_id" ||
		related.Item["plus"] != json.Number("3") {
		t.Fatalf("stored observation details must remain separately inspectable: %+v", related)
	}
	lateDrop := rareDrop
	lateDrop.ID = newTestEventID(t)
	lateDrop.Kind = "drop.item"
	lateDrop.SourceRef = "EVENT_ITEM_DROP"
	lateDrop.OccurredAt = databaseNow.UTC().Add(10 * time.Second)
	lateDrop.Sequence = int64Pointer64(51)
	lateDrop.Region = intPointer(25273)
	lateDrop.X, lateDrop.Y, lateDrop.Z = floatPointer(10), floatPointer(20), floatPointer(30)
	unlinkedGain := observedItem
	unlinkedGain.ID = newTestEventID(t)
	unlinkedGain.Kind = "item.acquired"
	unlinkedGain.OccurredAt = databaseNow.UTC().Add(11 * time.Second)
	unlinkedGain.Sequence = int64Pointer64(52)
	unlinkedGain.Region = lateDrop.Region
	unlinkedGain.X, unlinkedGain.Y, unlinkedGain.Z = floatPointer(10), floatPointer(20), floatPointer(30)
	unlinkedGain.Payload = json.RawMessage(`{"item":{"model":777,"servername":"ITEM_TEST","plus":4},"quantity_delta":1,"destination_container":{"type":"inventory","slot":14},"acquisition_method":"unknown"}`)
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{lateDrop, unlinkedGain})
	if err != nil || !changed || results[0].Status != "persisted" || results[1].Status != "persisted" {
		t.Fatalf("unlinked drop and gain = %+v, changed=%v, err=%v", results, changed, err)
	}
	exact, err = store.List(ctx, Filter{Server: server, EventID: lateDrop.ID, Limit: 1})
	var temporalPayload struct {
		Item struct {
			Plus int `json:"plus"`
		} `json:"item"`
		Observation struct {
			Association string `json:"association"`
		} `json:"item_observation"`
	}
	if err != nil || len(exact.Events) != 1 || json.Unmarshal(exact.Events[0].Payload, &temporalPayload) != nil ||
		temporalPayload.Item.Plus != 0 || temporalPayload.Observation.Association != "" {
		t.Fatalf("unlinked nearby gain must not lend an item instance: %+v, err=%v", exact, err)
	}
	if related := exact.Events[0].RelatedItemObservation; related == nil ||
		related.EventID != unlinkedGain.ID || related.Association != "nearby_same_model_observation" ||
		related.Item["plus"] != json.Number("4") {
		t.Fatalf("nearby observation must retain its own source and details: %+v", related)
	}
	competingDrop := lateDrop
	competingDrop.ID = newTestEventID(t)
	competingDrop.Sequence = int64Pointer64(53)
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{competingDrop})
	if err != nil || !changed || results[0].Status != "persisted" {
		t.Fatalf("competing drop = %+v, changed=%v, err=%v", results, changed, err)
	}
	exact, err = store.List(ctx, Filter{Server: server, EventID: lateDrop.ID, Limit: 1})
	var ambiguousPayload struct {
		Observation struct {
			Association string `json:"association"`
		} `json:"item_observation"`
	}
	if err != nil || len(exact.Events) != 1 || json.Unmarshal(exact.Events[0].Payload, &ambiguousPayload) != nil ||
		ambiguousPayload.Observation.Association != "" {
		t.Fatalf("ambiguous nearby drop was still linked: %+v, err=%v", exact, err)
	}
	if exact.Events[0].RelatedItemObservation != nil {
		t.Fatal("competing same-model callbacks must suppress the nearby observation suggestion")
	}
	// One explicitly linked acquisition remains visible when its callback is
	// outside the requested window. Count and pagination operate on the same
	// filtered logical rows, not client-side hidden items.
	from = databaseNow.UTC().Add(time.Second)
	to = databaseNow.UTC().Add(3 * time.Second)
	visibleGain, listErr := store.List(ctx, Filter{Server: server, ItemQuery: "777",
		From: &from, To: &to, Limit: 10})
	if listErr != nil || visibleGain.Total != 1 || len(visibleGain.Events) != 1 ||
		visibleGain.Events[0].ID != observedItem.ID {
		t.Fatalf("linked callback outside filter hides its gain: %+v, err=%v", visibleGain, listErr)
	}
	itemEvents, listErr := store.List(ctx, Filter{Server: server, Kind: "item.gains", Limit: 10})
	if listErr != nil || itemEvents.Total != 2 {
		t.Fatalf("owned gains must remain distinct from world drops: %+v, err=%v", itemEvents, listErr)
	}
	allItems, listErr := store.List(ctx, Filter{Server: server, ItemQuery: "777", Limit: 2})
	if listErr != nil || allItems.Total != 5 || len(allItems.Events) != 2 ||
		allItems.NextCursor == "" {
		t.Fatalf("historical item page one count/cursor = %+v, err=%v", allItems, listErr)
	}
	secondItems, listErr := store.List(ctx, Filter{Server: server, ItemQuery: "777", Limit: 2,
		Cursor: allItems.NextCursor})
	if listErr != nil || secondItems.Total != 5 || len(secondItems.Events) != 2 ||
		secondItems.NextCursor == "" {
		t.Fatalf("historical item page two = %+v, err=%v", secondItems, listErr)
	}
	thirdItems, listErr := store.List(ctx, Filter{Server: server, ItemQuery: "777", Limit: 2,
		Cursor: secondItems.NextCursor})
	if listErr != nil || thirdItems.Total != 5 || len(thirdItems.Events) != 1 ||
		thirdItems.NextCursor != "" {
		t.Fatalf("historical item page three = %+v, err=%v", thirdItems, listErr)
	}
	seen := make(map[string]bool)
	for _, row := range append(append(allItems.Events, secondItems.Events...), thirdItems.Events...) {
		if seen[row.ID] {
			t.Fatalf("repeated canonical event across pages: %s", row.ID)
		}
		seen[row.ID] = true
	}
	for _, id := range []string{rareDrop.ID, observedItem.ID, lateDrop.ID, unlinkedGain.ID, competingDrop.ID} {
		if !seen[id] {
			t.Fatalf("unverified historical link hid an independent event: %s", id)
		}
	}
	stateDiffs, listErr := store.List(ctx, Filter{Server: server, Source: "phbot.state_diff", Limit: 10})
	if listErr != nil || stateDiffs.Total < 2 {
		t.Fatalf("source-filtered canonical observations must remain visible: %+v, err=%v", stateDiffs, listErr)
	}
	for _, row := range stateDiffs.Events {
		if row.Source != "phbot.state_diff" {
			t.Fatalf("source-filtered result contains %q", row.Source)
		}
	}
	// A code-based search must not collapse an acquisition when its callback
	// only identifies a model; hiding it would lose the entire occurrence.
	byCode, listErr := store.List(ctx, Filter{Server: server, ItemQuery: "ITEM_TEST", Limit: 10})
	if listErr != nil || byCode.Total != 2 || len(byCode.Events) != 2 {
		t.Fatalf("item code search must retain source observations excluded by the callback filter: %+v, err=%v", byCode, listErr)
	}
	for _, row := range byCode.Events {
		if row.Kind != "item.acquired" && row.Kind != "item.quantity_increased" {
			t.Fatalf("callback matched a code that was not observed on it: %+v", row)
		}
	}
	onlyCallbacks, listErr := store.List(ctx, Filter{Server: server,
		Source: "phbot.callback", ItemQuery: "777", Limit: 10})
	if listErr != nil || onlyCallbacks.Total != 3 || len(onlyCallbacks.Events) != 3 {
		t.Fatalf("callback source filter must show all three independent world drops: %+v, err=%v", onlyCallbacks, listErr)
	}
	onlyItemCategory, listErr := store.List(ctx, Filter{Server: server,
		Category: "item", ItemQuery: "777", Limit: 10})
	if listErr != nil || onlyItemCategory.Total != 2 || len(onlyItemCategory.Events) != 2 {
		t.Fatalf("item category filter must retain both raw gains: %+v, err=%v", onlyItemCategory, listErr)
	}
	onlyIncrease, listErr := store.List(ctx, Filter{Server: server,
		Kind: "item.quantity_increased", ItemQuery: "777", Limit: 10})
	if listErr != nil || onlyIncrease.Total != 1 || onlyIncrease.Events[0].ID != observedItem.ID {
		t.Fatalf("explicit gain kind must bypass logical collapse: %+v, err=%v", onlyIncrease, listErr)
	}
	for _, source := range []string{"phbot.lifecycle_callback", "phbot.alchemy_callback", "joymax.unique_notice"} {
		if !ValidSourceFilter(source) {
			t.Fatalf("canonical source %s was rejected", source)
		}
	}
	for _, source := range []string{"phbot.packet", "phbot.plugin", "phbot.alchemy", "phbot.death"} {
		if ValidSourceFilter(source) {
			t.Fatalf("noncanonical source %s was accepted", source)
		}
	}
	if ValidSourceFilter("phbot.callback; DROP TABLE activity_events") {
		t.Fatal("invalid provenance filter was accepted")
	}
	exactGain, listErr := store.List(ctx, Filter{Server: server, EventID: observedItem.ID, Limit: 1})
	if listErr != nil || exactGain.Total != 1 || len(exactGain.Events) != 1 {
		t.Fatalf("canonical gain must remain directly inspectable: %+v, err=%v", exactGain, listErr)
	}
	conflict := rareDrop
	conflict.Zone = "Donwhang"
	results, _, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{conflict})
	if err != nil || results[0].Status != "rejected" || results[0].Reason != "session_or_event_rejected" {
		t.Fatalf("event ID conflict = %+v, err=%v", results, err)
	}
	sequenceConflict := rareDrop
	sequenceConflict.ID = newTestEventID(t)
	results, _, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{sequenceConflict})
	if err != nil || results[0].Status != "rejected" {
		t.Fatalf("sequence conflict = %+v, err=%v", results, err)
	}
	page, err = store.List(ctx, Filter{Server: server, Kind: "drop.rare", Category: "drop", ItemQuery: "777", Limit: 10})
	if err != nil || page.Total != 1 || len(page.Events) != 1 || page.Events[0].ID != rareDrop.ID || page.Events[0].Zone != "Jangan" {
		t.Fatalf("generic item-filtered events = %+v, err=%v", page, err)
	}

	alchemySequence := int64(2)
	alchemy := AgentEvent{
		ID: newTestEventID(t), Schema: 1, Kind: "alchemy.attempt", Category: "alchemy",
		CharacterID: characterID, SessionID: newSessionID, Server: server, Character: "Alpha",
		OccurredAt: databaseNow.UTC().Add(time.Second), Sequence: &alchemySequence,
		Source: "phbot.alchemy_callback", SourceRef: "alchemy_update",
		Payload: json.RawMessage(`{"slot":13,"success":null,"plus":5}`),
	}
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{alchemy})
	if err != nil || !changed || results[0].Status != "persisted" {
		t.Fatalf("alchemy attempt batch = %+v, changed=%v, err=%v", results, changed, err)
	}
	page, err = store.List(ctx, Filter{Server: server, Kind: "alchemy.attempt", Limit: 10})
	if err != nil || page.Total != 1 || page.Alchemy == nil || page.Alchemy.Attempts != 1 || page.Alchemy.Unknown != 1 || page.Alchemy.Successes != 0 || page.Alchemy.HighestPlus == nil || *page.Alchemy.HighestPlus != 5 {
		t.Fatalf("alchemy summary = %+v, err=%v", page, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE agents SET phbot_version='20.1.2' WHERE agent_id=$1`, credential.AgentID); err != nil {
		t.Fatal(err)
	}
	levelUp := AgentEvent{
		ID: newTestEventID(t), Schema: 1, Kind: "character.level_up", Category: "character",
		CharacterID: characterID, SessionID: newSessionID, Server: server, Character: "Alpha",
		OccurredAt: databaseNow.UTC().Add(2 * time.Second), Sequence: int64Pointer64(3),
		Source: "phbot.callback", SourceRef: "EVENT_LEVEL_UP", Payload: json.RawMessage(`{"level":71}`),
	}
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{levelUp})
	if err != nil || !changed || results[0].Status != "persisted" {
		t.Fatalf("level-up batch = %+v, changed=%v, err=%v", results, changed, err)
	}
	var levelPayload json.RawMessage
	if err := pool.QueryRow(ctx, `SELECT payload FROM activity_events WHERE event_id=$1`, levelUp.ID).Scan(&levelPayload); err != nil {
		t.Fatal(err)
	}
	var reached struct {
		Level         int `json:"level"`
		CallbackLevel int `json:"callback_level"`
	}
	if err := json.Unmarshal(levelPayload, &reached); err != nil || reached.Level != 72 || reached.CallbackLevel != 71 {
		t.Fatalf("stored reached level = %s", levelPayload)
	}
	if _, err := pool.Exec(ctx, `UPDATE agents SET phbot_version='20.1.3' WHERE agent_id=$1`, credential.AgentID); err != nil {
		t.Fatal(err)
	}
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{levelUp})
	if err != nil || changed || results[0].Status != "persisted" {
		t.Fatalf("level-up replay after version change = %+v, changed=%v, err=%v", results, changed, err)
	}
	otherVersionLevelUp := levelUp
	otherVersionLevelUp.ID = newTestEventID(t)
	otherVersionLevelUp.Sequence = int64Pointer64(4)
	otherVersionLevelUp.OccurredAt = levelUp.OccurredAt.Add(time.Second)
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{otherVersionLevelUp})
	if err != nil || !changed || results[0].Status != "persisted" {
		t.Fatalf("unverified-version level-up = %+v, changed=%v, err=%v", results, changed, err)
	}
	if err := pool.QueryRow(ctx, `SELECT payload FROM activity_events WHERE event_id=$1`, otherVersionLevelUp.ID).Scan(&levelPayload); err != nil {
		t.Fatal(err)
	}
	var unverified map[string]any
	if err := json.Unmarshal(levelPayload, &unverified); err != nil || unverified["level"] != float64(71) || unverified["callback_level"] != nil {
		t.Fatalf("unverified-version stored level = %s", levelPayload)
	}
	// A gain found across a backend outage has no known time. A unique
	// same-model drop inside its interval is related evidence only.
	gapFrom := databaseNow.UTC().Add(40 * time.Second)
	gapTo := databaseNow.UTC().Add(70 * time.Second)
	gapDrop := rareDrop
	gapDrop.ID = newTestEventID(t)
	gapDrop.OccurredAt = databaseNow.UTC().Add(60 * time.Second)
	gapDrop.Sequence = int64Pointer64(60)
	gapDrop.Region, gapDrop.X, gapDrop.Y, gapDrop.Z = nil, nil, nil, nil
	gapGain := observedItem
	gapGain.ID = newTestEventID(t)
	gapGain.Kind = "item.acquired"
	gapGain.OccurredAt = gapTo
	gapGain.Sequence = int64Pointer64(61)
	gapGain.Region, gapGain.X, gapGain.Y, gapGain.Z = nil, nil, nil, nil
	gapGain.Payload = json.RawMessage(`{"item":{"model":777,"servername":"ITEM_TEST","plus":5},"quantity_delta":1,"destination_container":{"type":"pets","id":"9","slot":11},"acquisition_method":"unknown","observation_gap":{"reason":"backend_reconnect","from":"` +
		gapFrom.Format(time.RFC3339Nano) + `","to":"` + gapTo.Format(time.RFC3339Nano) + `","previous_session_id":"` + gapDrop.SessionID + `"}}`)
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{gapDrop, gapGain})
	if err != nil || !changed || results[0].Status != "persisted" || results[1].Status != "persisted" {
		t.Fatalf("gap drop and gain = %+v, changed=%v, err=%v", results, changed, err)
	}
	exact, err = store.List(ctx, Filter{Server: server, EventID: gapDrop.ID, Limit: 1})
	var gapPayload struct {
		Item        json.RawMessage `json:"item"`
		Observation json.RawMessage `json:"item_observation"`
	}
	if err != nil || len(exact.Events) != 1 || json.Unmarshal(exact.Events[0].Payload, &gapPayload) != nil ||
		len(gapPayload.Item) != 0 || len(gapPayload.Observation) != 0 {
		t.Fatalf("a gap observation must not become drop enrichment: %+v, err=%v", exact, err)
	}
	if related := exact.Events[0].RelatedItemObservation; related == nil ||
		related.EventID != gapGain.ID || related.Association != "gap_reconciled_same_model" ||
		related.Item["plus"] != json.Number("5") {
		t.Fatalf("gap observation must be offered as related evidence: %+v", related)
	}
	secondGapDrop := gapDrop
	secondGapDrop.ID = newTestEventID(t)
	secondGapDrop.OccurredAt = databaseNow.UTC().Add(65 * time.Second)
	secondGapDrop.Sequence = int64Pointer64(62)
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{secondGapDrop})
	if err != nil || !changed || results[0].Status != "persisted" {
		t.Fatalf("second gap drop = %+v, changed=%v, err=%v", results, changed, err)
	}
	exact, err = store.List(ctx, Filter{Server: server, EventID: gapDrop.ID, Limit: 1})
	if err != nil || len(exact.Events) != 1 || exact.Events[0].RelatedItemObservation != nil {
		t.Fatalf("competing same-model drops in one gap must suppress the suggestion: %+v, err=%v", exact, err)
	}
}

func TestInvalidDeathEventReturnsClassifiableError(t *testing.T) {
	store := NewStore(nil)
	_, err := store.AppendDeath(context.Background(), "bad-agent", "bad-character", "bad-session", AgentDeath{})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("invalid event error = %v, want ErrInvalidEvent", err)
	}
}

func intPointer(value int) *int           { return &value }
func floatPointer(value float64) *float64 { return &value }
func int64Pointer64(value int64) *int64   { return &value }

func newTestEventID(t *testing.T) string {
	t.Helper()
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	return credential.AgentID
}

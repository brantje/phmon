package events

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
)

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
	_, _, projectionErr := appendOne(ctx, projectionTx, credential.AgentID, invalidProjection)
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
		Payload: json.RawMessage(`{"slot":13,"success":true,"plus":5}`),
	}
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{alchemy})
	if err != nil || !changed || results[0].Status != "persisted" {
		t.Fatalf("alchemy attempt batch = %+v, changed=%v, err=%v", results, changed, err)
	}
	page, err = store.List(ctx, Filter{Server: server, Kind: "alchemy.attempt", Limit: 10})
	if err != nil || page.Total != 1 || page.Alchemy == nil || page.Alchemy.Attempts != 1 || page.Alchemy.Successes != 1 || page.Alchemy.HighestPlus == nil || *page.Alchemy.HighestPlus != 5 {
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

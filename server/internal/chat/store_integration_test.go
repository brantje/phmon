package chat_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/chat"
	"phmon/server/internal/database"
	"phmon/server/internal/events"
)

func TestChatProjectionPaginationUnreadAndPreferences(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run chat integration tests")
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
	server := "chat-" + credential.AgentID[:8]
	characterStore := characters.NewStore(pool)
	characterID, err := characterStore.Resolve(ctx, characters.Identity{Server: server, Name: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := characterStore.ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM activity_events WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE character_id=$1`, characterID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	var databaseNow time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&databaseNow); err != nil {
		t.Fatal(err)
	}
	store := events.NewStore(pool)
	chatStore := chat.NewStore(pool)
	first := makeChatEvent(credential.AgentID, characterID, sessionID, server, 1, databaseNow.Add(time.Second), "first")
	results, changed, err := store.AppendBatch(ctx, credential.AgentID, []events.AgentEvent{first})
	if err != nil || !changed || len(results) != 1 || results[0].Status != "persisted" {
		t.Fatalf("first message insert = %+v changed=%v err=%v", results, changed, err)
	}
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []events.AgentEvent{first})
	if err != nil || changed || len(results) != 1 || results[0].Status != "persisted" {
		t.Fatalf("event replay = %+v changed=%v err=%v", results, changed, err)
	}
	second := makeChatEvent(credential.AgentID, characterID, sessionID, server, 2, databaseNow.Add(2*time.Second), "second")
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []events.AgentEvent{second})
	if err != nil || !changed || results[0].Status != "persisted" {
		t.Fatalf("second message insert = %+v changed=%v err=%v", results, changed, err)
	}
	filter := chat.Filter{Server: server, CharacterID: characterID, Channel: "private", Peer: "Beta", Limit: 1}
	page, err := chatStore.Messages(ctx, filter)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].Text != "second" || !page.HasOlder || page.OlderCursor == "" {
		t.Fatalf("latest chat page = %+v err=%v", page, err)
	}
	filter.Before = page.OlderCursor
	page, err = chatStore.Messages(ctx, filter)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].Text != "first" || page.HasOlder {
		t.Fatalf("older chat page = %+v err=%v", page, err)
	}
	counts, err := chatStore.UnreadByChannel(ctx, server, characterID)
	if err != nil || counts["private"] != 2 {
		t.Fatalf("unread counts = %#v err=%v", counts, err)
	}
	if err := chatStore.MarkRead(ctx, server, characterID, "private", "Beta", page.Messages[0].MessageID); err != nil {
		t.Fatal(err)
	}
	counts, err = chatStore.UnreadByChannel(ctx, server, characterID)
	if err != nil || counts["private"] != 1 {
		t.Fatalf("unread after cursor = %#v err=%v", counts, err)
	}
	contacts, err := chatStore.Contacts(ctx, server, characterID, 20)
	if err != nil || len(contacts) != 1 || contacts[0].PeerKey != "beta" || contacts[0].LastMessageID == "" || contacts[0].Unread != 1 {
		t.Fatalf("private contacts = %+v err=%v", contacts, err)
	}
	prefs := chat.Preferences{BrowserNotifications: true, MessageSound: true}
	if err := chatStore.SavePreferences(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	gotPrefs, err := chatStore.Preferences(ctx)
	if err != nil || gotPrefs != prefs {
		t.Fatalf("chat preferences = %+v err=%v", gotPrefs, err)
	}
}

func makeChatEvent(agentID, characterID, sessionID, server string, sequence int64, at time.Time, message string) events.AgentEvent {
	eventID := agentID[:8] + "-" + agentID[9:13] + "-4000-8000-" + agentID[24:32] + fmt.Sprintf("%04d", sequence)
	return events.AgentEvent{
		ID: eventID, Schema: 1, Kind: "chat.message_received", Category: "chat",
		CharacterID: characterID, SessionID: sessionID, Server: server, Character: "Alpha",
		OccurredAt: at.UTC(), Sequence: &sequence, Source: "phbot.chat_callback", SourceRef: "handle_chat",
		Payload: json.RawMessage(fmt.Sprintf(`{"channel":"private","raw_type":"4","sender":"Beta","recipient":"Alpha","direction":"inbound","message":%q}`, message)),
	}
}

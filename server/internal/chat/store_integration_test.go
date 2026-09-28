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
	secondCharacterID, err := characterStore.Resolve(ctx, characters.Identity{Server: server, Name: "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	secondSessionID, err := characterStore.ClaimSessionID(ctx, credential.AgentID, secondCharacterID, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM activity_events WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE character_id IN ($1::uuid,$2::uuid)`, characterID, secondCharacterID)
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
	generalAlpha := makeChannelChatEvent(credential.AgentID, characterID, sessionID, server, "Alpha", 3, databaseNow.Add(3*time.Second), "general", "1", "Veyra", "hello Alpha")
	generalBeta := makeChannelChatEvent(credential.AgentID, secondCharacterID, secondSessionID, server, "Beta", 4, databaseNow.Add(4*time.Second), "general", "1", "Veyra", "hello Beta")
	globalBeta := makeChannelChatEvent(credential.AgentID, secondCharacterID, secondSessionID, server, "Beta", 5, databaseNow.Add(5*time.Second), "global", "6", "Veyra", "global message")
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []events.AgentEvent{generalAlpha, generalBeta, globalBeta})
	if err != nil || !changed || len(results) != 3 {
		t.Fatalf("server chat inserts = %+v changed=%v err=%v", results, changed, err)
	}
	serverPage, err := chatStore.Messages(ctx, chat.Filter{Server: server, CharacterID: characterID, Channel: "general", Limit: 20})
	if err != nil || len(serverPage.Messages) != 2 || serverPage.Messages[0].CharacterID != characterID || serverPage.Messages[1].CharacterID != secondCharacterID {
		t.Fatalf("General history should include every character on the selected server: %+v err=%v", serverPage.Messages, err)
	}
	counts, err = chatStore.UnreadByChannel(ctx, server, characterID)
	if err != nil || counts["general"] != 2 || counts["private"] != 1 || counts["global"] != 0 {
		t.Fatalf("server-wide General unread and ignored Global count = %#v err=%v", counts, err)
	}
	if err := chatStore.MarkRead(ctx, server, characterID, "general", "", serverPage.Messages[1].MessageID); err != nil {
		t.Fatal(err)
	}
	counts, err = chatStore.UnreadByChannel(ctx, server, secondCharacterID)
	if err != nil || counts["general"] != 0 || counts["global"] != 0 {
		t.Fatalf("General read on one character should clear the server for all characters: %#v err=%v", counts, err)
	}
	generalLater := makeChannelChatEvent(credential.AgentID, secondCharacterID, secondSessionID, server, "Beta", 6, databaseNow.Add(6*time.Second), "general", "1", "Veyra", "later")
	results, changed, err = store.AppendBatch(ctx, credential.AgentID, []events.AgentEvent{generalLater})
	if err != nil || !changed || results[0].Status != "persisted" {
		t.Fatalf("later General insert = %+v changed=%v err=%v", results, changed, err)
	}
	counts, err = chatStore.UnreadByChannel(ctx, server, characterID)
	if err != nil || counts["general"] != 1 || counts["global"] != 0 {
		t.Fatalf("later server-wide General unread = %#v err=%v", counts, err)
	}
	contacts, err := chatStore.Contacts(ctx, server, characterID, 20)
	if err != nil || len(contacts) != 1 || contacts[0].PeerKey != "beta" || contacts[0].LastMessageID == "" || contacts[0].Unread != 1 {
		t.Fatalf("private contacts = %+v err=%v", contacts, err)
	}
	readState, err := chatStore.ReadState(ctx, server, characterID)
	if err != nil || readState.UnreadByChannel["general"] != 1 || readState.UnreadByChannel["private"] != 1 ||
		len(readState.Contacts) != 1 || readState.Contacts[0].Unread != 1 {
		t.Fatalf("read state = %+v err=%v", readState, err)
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
	return makeChannelChatEvent(agentID, characterID, sessionID, server, "Alpha", sequence, at, "private", "2", "Beta", message)
}

func makeChannelChatEvent(agentID, characterID, sessionID, server, character string, sequence int64, at time.Time, channel, rawType, sender, message string) events.AgentEvent {
	eventID := agentID[:8] + "-" + agentID[9:13] + "-4000-8000-" + agentID[24:32] + fmt.Sprintf("%04d", sequence)
	return events.AgentEvent{
		ID: eventID, Schema: 1, Kind: "chat.message_received", Category: "chat",
		CharacterID: characterID, SessionID: sessionID, Server: server, Character: character,
		OccurredAt: at.UTC(), Sequence: &sequence, Source: "phbot.chat_callback", SourceRef: "handle_chat",
		Payload: json.RawMessage(fmt.Sprintf(`{"channel":%q,"raw_type":%q,"sender":%q,"recipient":%q,"direction":"inbound","message":%q}`, channel, rawType, sender, character, message)),
	}
}

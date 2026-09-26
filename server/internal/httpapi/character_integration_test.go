package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
)

func TestCharacterProtocolAndHTTPAPI(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run character API integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
	agentStore := agents.NewStore(pool)
	if err := agentStore.CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	serverName := "PhMonTest-" + credential.AgentID
	groupName := "phmontest-" + credential.AgentID
	t.Cleanup(func() {
		clean := context.Background()
		_, _ = pool.Exec(clean, `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(clean, `DELETE FROM characters WHERE server_key=$1`, strings.ToLower(serverName))
		_, _ = pool.Exec(clean, `DELETE FROM character_groups WHERE name=$1`, groupName)
		_, _ = pool.Exec(clean, `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	characterStore := characters.NewStore(pool)
	registry := agents.NewRegistry()
	web := httptest.NewServer(New(Dependencies{Agents: agentStore, Registry: registry, Characters: characterStore}))
	defer web.Close()
	conn := dialAgent(t, web.URL, credential.Token)
	defer conn.Close(websocket.StatusNormalClosure, "done")
	writeHello(t, conn, credential.AgentID)
	var ack helloAck
	if err := wsjson.Read(ctx, conn, &ack); err != nil {
		t.Fatal(err)
	}
	if ack.ProtocolVersion != 2 {
		t.Fatalf("unexpected protocol version: %d", ack.ProtocolVersion)
	}
	guild := "TestGuild"
	if err := wsjson.Write(ctx, conn, agentMessage{Type: "character.identify", ProtocolVersion: 2, Server: serverName, Name: "LiveAlpha", Guild: &guild, SentAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	var registration struct {
		Type            string `json:"type"`
		ProtocolVersion int    `json:"protocol_version"`
		CharacterID     string `json:"character_id"`
	}
	if err := wsjson.Read(ctx, conn, &registration); err != nil {
		t.Fatal(err)
	}
	if registration.Type != "character.registered" || registration.ProtocolVersion != 2 || !agents.ValidAgentID(registration.CharacterID) {
		t.Fatalf("invalid registration response: %+v", registration)
	}
	preResponse, err := http.Get(web.URL + "/api/characters?q=" + serverName)
	if err != nil {
		t.Fatal(err)
	}
	var preSnapshot struct {
		Characters []characters.Character `json:"characters"`
	}
	if err := json.NewDecoder(preResponse.Body).Decode(&preSnapshot); err != nil {
		preResponse.Body.Close()
		t.Fatal(err)
	}
	preResponse.Body.Close()
	if len(preSnapshot.Characters) != 1 || !preSnapshot.Characters[0].Online {
		t.Fatalf("explicit identity claim must establish the live session: %+v", preSnapshot.Characters)
	}
	level, hp, mp, exp, sp, gold, region, x, y := 110, int64(500), int64(250), int64(800), int64(42), int64(99), 25000, 10.5, 20.5
	state := characters.State{Level: &level, HP: &hp, MP: &mp, CurrentEXP: &exp, SP: &sp, Gold: &gold, Region: &region, X: &x, Y: &y}
	if err := wsjson.Write(ctx, conn, map[string]any{"type": "character.snapshot", "protocol_version": 2, "character_id": registration.CharacterID, "sent_at": time.Now().UTC().Format(time.RFC3339), "state": state}); err != nil {
		t.Fatal(err)
	}
	var listed struct {
		Characters []characters.Character `json:"characters"`
	}
	var responseCode int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response, requestErr := http.Get(web.URL + "/api/characters?q=" + serverName)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		listed = struct {
			Characters []characters.Character `json:"characters"`
		}{Characters: []characters.Character{}}
		responseCode = response.StatusCode
		decodeErr := json.NewDecoder(response.Body).Decode(&listed)
		response.Body.Close()
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if len(listed.Characters) == 1 && listed.Characters[0].Online && listed.Characters[0].Level != nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if responseCode != 200 || len(listed.Characters) != 1 || listed.Characters[0].ID != registration.CharacterID || !listed.Characters[0].Online || listed.Characters[0].Level == nil || *listed.Characters[0].Level != 110 {
		t.Fatalf("unexpected character API result: code=%d data=%+v", responseCode, listed)
	}
	groupRequest, err := http.NewRequest(http.MethodPost, web.URL+"/api/groups", strings.NewReader(`{"name":"`+groupName+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	groupRequest.Header.Set("Content-Type", "application/json")
	groupResponse, err := http.DefaultClient.Do(groupRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer groupResponse.Body.Close()
	var group characters.Group
	if err := json.NewDecoder(groupResponse.Body).Decode(&group); err != nil {
		t.Fatal(err)
	}
	if groupResponse.StatusCode != 201 {
		t.Fatalf("group create failed: %d", groupResponse.StatusCode)
	}
	memberRequest, err := http.NewRequest(http.MethodPut, web.URL+"/api/groups/"+group.ID+"/members/"+registration.CharacterID, nil)
	if err != nil {
		t.Fatal(err)
	}
	memberResponse, err := http.DefaultClient.Do(memberRequest)
	if err != nil {
		t.Fatal(err)
	}
	memberResponse.Body.Close()
	if memberResponse.StatusCode != 204 {
		t.Fatalf("member add failed: %d", memberResponse.StatusCode)
	}
	filtered, err := http.Get(web.URL + "/api/characters?group_id=" + group.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer filtered.Body.Close()
	var grouped struct {
		Characters []characters.Character `json:"characters"`
	}
	if err := json.NewDecoder(filtered.Body).Decode(&grouped); err != nil {
		t.Fatal(err)
	}
	if len(grouped.Characters) != 1 || grouped.Characters[0].ID != registration.CharacterID {
		t.Fatalf("group filter failed: %+v", grouped)
	}
	_ = conn.Close(websocket.StatusNormalClosure, "test complete")
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		records, listErr := agentStore.ListSeen(ctx)
		if listErr == nil {
			for _, record := range records {
				if record.AgentID == credential.AgentID && record.LastDisconnectedAt != nil {
					character, getErr := characterStore.Get(ctx, registration.CharacterID)
					if getErr != nil {
						t.Fatal(getErr)
					}
					if character.Online {
						t.Fatal("agent disconnect left character online")
					}
					return
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("agent disconnect was not recorded")
}

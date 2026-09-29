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
	"phmon/server/internal/mapanalytics"
	"phmon/server/internal/mapprofile"
	"phmon/server/internal/resources"
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
	t.Cleanup(pool.Close)
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
	otherServerName := "PhMonOtherTest-" + credential.AgentID
	groupName := "phmontest-" + credential.AgentID
	t.Cleanup(func() {
		clean := context.Background()
		_, _ = pool.Exec(clean, `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(clean, `DELETE FROM characters WHERE server_key=$1`, strings.ToLower(serverName))
		_, _ = pool.Exec(clean, `DELETE FROM characters WHERE server_key=$1`, strings.ToLower(otherServerName))
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
	otherCharacterID, err := characterStore.Resolve(ctx, characters.Identity{Server: otherServerName, Name: "OtherAlpha"})
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []struct {
		server string
		want   string
	}{
		{server: serverName, want: registration.CharacterID},
		{server: otherServerName, want: otherCharacterID},
	} {
		response, requestErr := http.Get(web.URL + "/api/characters?server=" + scope.server)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		var scoped struct {
			Characters []characters.Character `json:"characters"`
		}
		decodeErr := json.NewDecoder(response.Body).Decode(&scoped)
		response.Body.Close()
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if response.StatusCode != http.StatusOK || len(scoped.Characters) != 1 || scoped.Characters[0].ID != scope.want {
			t.Fatalf("character API crossed server scope %q: status=%d data=%+v", scope.server, response.StatusCode, scoped)
		}
	}
	detailResponse, err := http.Get(web.URL + "/api/characters/" + registration.CharacterID + "?server=" + otherServerName)
	if err != nil {
		t.Fatal(err)
	}
	detailResponse.Body.Close()
	if detailResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("character detail should be hidden outside selected server, got %d", detailResponse.StatusCode)
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
	if err := characterStore.SetMember(ctx, group.ID, otherCharacterID, true); err != nil {
		t.Fatal(err)
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
	var ownMember, otherMember bool
	for _, item := range grouped.Characters {
		ownMember = ownMember || item.ID == registration.CharacterID
		otherMember = otherMember || item.ID == otherCharacterID
	}
	if len(grouped.Characters) != 2 || !ownMember || !otherMember {
		t.Fatalf("group filter failed: %+v", grouped)
	}
	for _, scope := range []struct {
		server string
		want   string
	}{
		{server: serverName, want: registration.CharacterID},
		{server: otherServerName, want: otherCharacterID},
	} {
		response, requestErr := http.Get(web.URL + "/api/groups?server=" + scope.server)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		var scoped struct {
			Groups []characters.Group `json:"groups"`
		}
		decodeErr := json.NewDecoder(response.Body).Decode(&scoped)
		response.Body.Close()
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		var matched *characters.Group
		for index := range scoped.Groups {
			if scoped.Groups[index].ID == group.ID {
				matched = &scoped.Groups[index]
			}
		}
		if response.StatusCode != http.StatusOK || matched == nil || len(matched.Members) != 1 || matched.Members[0].ID != scope.want {
			t.Fatalf("group API crossed server scope %q: status=%d group=%+v", scope.server, response.StatusCode, matched)
		}
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

func TestStaleCharacterOperationsDoNotCloseSiblingCharacterSessions(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run character API integration tests")
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
	store := agents.NewStore(pool)
	if err := store.CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	charactersStore := characters.NewStore(pool)
	registry := agents.NewRegistry()
	server := httptest.NewServer(New(Dependencies{
		Database: pool, Agents: store, Registry: registry, Characters: charactersStore,
		AgentOptions: AgentOptions{HeartbeatTimeout: 5 * time.Second},
	}))
	defer server.Close()
	serverName := "PhMonStaleTest-" + credential.AgentID
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=$1`, strings.ToLower(serverName))
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})

	conn1 := dialAgent(t, server.URL, credential.Token)
	defer conn1.CloseNow()
	writeHello(t, conn1, credential.AgentID)
	var ack helloAck
	if err := wsjson.Read(ctx, conn1, &ack); err != nil {
		t.Fatal(err)
	}
	aID := identifyAndSnapshot(t, ctx, conn1, serverName, "Alpha", int64(10))
	bID := identifyAndSnapshot(t, ctx, conn1, serverName, "Beta", int64(20))

	conn2 := dialAgent(t, server.URL, credential.Token)
	defer conn2.CloseNow()
	writeHello(t, conn2, credential.AgentID)
	if err := wsjson.Read(ctx, conn2, &ack); err != nil {
		t.Fatal(err)
	}
	if got := identifyAndSnapshot(t, ctx, conn2, serverName, "Alpha", int64(30)); got != aID {
		t.Fatalf("takeover changed stable character ID: old=%s new=%s", aID, got)
	}

	for _, stale := range []struct {
		name string
		msg  agentMessage
	}{
		{name: "state", msg: agentMessage{Type: "character.state", CharacterID: aID, State: characters.State{HP: int64ptr(999)}}},
		{name: "snapshot", msg: agentMessage{Type: "character.snapshot", CharacterID: aID, State: characters.State{HP: int64ptr(888)}}},
		{name: "left", msg: agentMessage{Type: "character.left", CharacterID: aID}},
	} {
		t.Run(stale.name, func(t *testing.T) {
			stale.msg.ProtocolVersion = 2
			stale.msg.SentAt = time.Now().UTC().Format(time.RFC3339)
			if err := wsjson.Write(ctx, conn1, stale.msg); err != nil {
				t.Fatal(err)
			}
			var rejected struct {
				Type            string `json:"type"`
				ProtocolVersion int    `json:"protocol_version"`
				CharacterID     string `json:"character_id"`
				Reason          string `json:"reason"`
			}
			if err := wsjson.Read(ctx, conn1, &rejected); err != nil {
				t.Fatalf("stale operation closed multiplexed socket: %v", err)
			}
			if rejected.Type != "character.rejected" || rejected.ProtocolVersion != 2 || rejected.CharacterID != aID || rejected.Reason != "not_current_session" {
				t.Fatalf("unexpected character rejection: %+v", rejected)
			}
		})
	}

	// The first connection still owns B after each stale operation targeting A.
	if err := wsjson.Write(ctx, conn1, agentMessage{Type: "character.state", ProtocolVersion: 2, CharacterID: bID, State: characters.State{HP: int64ptr(40)}, SentAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatalf("conn1 could not update unrelated B: %v", err)
	}
	for _, conn := range []*websocket.Conn{conn1, conn2} {
		if err := wsjson.Write(ctx, conn, agentMessage{Type: "heartbeat", ProtocolVersion: 2, SentAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
			t.Fatalf("sibling socket was closed: %v", err)
		}
	}
	if registry.ConnectionCount(credential.AgentID) != 2 {
		t.Fatalf("stale character messages closed a connection: active=%d", registry.ConnectionCount(credential.AgentID))
	}
	alpha, err := charactersStore.Get(ctx, aID)
	if err != nil || !alpha.Online || alpha.HP == nil || *alpha.HP != 30 {
		t.Fatalf("stale conn1 operation changed conn2's A: character=%+v err=%v", alpha, err)
	}
	waitFor(t, time.Second, func() bool {
		beta, getErr := charactersStore.Get(ctx, bID)
		return getErr == nil && beta.Online && beta.HP != nil && *beta.HP == 40
	})
}

func identifyAndSnapshot(t *testing.T, ctx context.Context, conn *websocket.Conn, serverName, name string, hp int64) string {
	t.Helper()
	if err := wsjson.Write(ctx, conn, agentMessage{Type: "character.identify", ProtocolVersion: 2, Server: serverName, Name: name, SentAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	var registered struct {
		Type        string `json:"type"`
		CharacterID string `json:"character_id"`
	}
	if err := wsjson.Read(ctx, conn, &registered); err != nil {
		t.Fatal(err)
	}
	if registered.Type != "character.registered" || !agents.ValidAgentID(registered.CharacterID) {
		t.Fatalf("unexpected identify response: %+v", registered)
	}
	if err := wsjson.Write(ctx, conn, agentMessage{Type: "character.snapshot", ProtocolVersion: 2, CharacterID: registered.CharacterID, State: characters.State{HP: int64ptr(hp)}, SentAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	return registered.CharacterID
}

func int64ptr(value int64) *int64 { return &value }


func TestMovementAnalyticsFailureDoesNotRejectCanonicalState(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run character analytics integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
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
	serverName := "PhMonAnalyticsFailure-" + credential.AgentID
	characterStore := characters.NewStore(pool)
	resourceStore := resources.NewStore(nil)
	resourceStore.SetItemMetadata(&resources.ItemMetadata{Servers: map[string]string{strings.ToLower(serverName): mapprofile.GreatestDatasetID}})

	analyticsConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	analyticsConfig.MaxConns = 1
	analyticsPool, err := pgxpool.NewWithConfig(ctx, analyticsConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer analyticsPool.Close()
	held, err := analyticsPool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	t.Cleanup(func() {
		clean := context.Background()
		_, _ = pool.Exec(clean, `DELETE FROM character_position_samples WHERE lower(server_name)=lower($1)`, serverName)
		_, _ = pool.Exec(clean, `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(clean, `DELETE FROM characters WHERE server_key=$1`, strings.ToLower(serverName))
		_, _ = pool.Exec(clean, `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})

	web := httptest.NewServer(New(Dependencies{
		Database: pool, Agents: agentStore, Registry: agents.NewRegistry(), Characters: characterStore,
		Resources: resourceStore, MapAnalytics: mapanalytics.NewStore(analyticsPool),
		AgentOptions: AgentOptions{HeartbeatTimeout: 8 * time.Second},
	}))
	defer web.Close()

	conn := dialAgent(t, web.URL, credential.Token)
	defer conn.CloseNow()
	if err := wsjson.Write(ctx, conn, agentMessage{
		Type: "hello", ProtocolVersion: agentProtocolVersion, AgentID: credential.AgentID,
		PluginVersion: "fixture", PhBotVersion: "fixture", SentAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	var ack helloAck
	if err := wsjson.Read(ctx, conn, &ack); err != nil {
		t.Fatal(err)
	}
	if ack.ProtocolVersion != agentProtocolVersion {
		t.Fatalf("unexpected protocol version: %d", ack.ProtocolVersion)
	}
	if err := wsjson.Write(ctx, conn, agentMessage{
		Type: "character.identify", ProtocolVersion: agentProtocolVersion, Server: serverName,
		Name: "AnalyticsBlocked", SentAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	var registered struct {
		Type string `json:"type"`
		CharacterID string `json:"character_id"`
		SessionID string `json:"session_id"`
	}
	if err := wsjson.Read(ctx, conn, &registered); err != nil {
		t.Fatal(err)
	}
	if registered.Type != "character.registered" || !agents.ValidAgentID(registered.CharacterID) || !agents.ValidAgentID(registered.SessionID) {
		t.Fatalf("unexpected registration: %+v", registered)
	}

	region, x, y, firstHP := 25273, 10.0, 20.0, int64(111)
	if err := wsjson.Write(ctx, conn, agentMessage{
		Type: "character.state", ProtocolVersion: agentProtocolVersion, CharacterID: registered.CharacterID,
		SessionID: registered.SessionID, SentAt: time.Now().UTC().Format(time.RFC3339),
		State: characters.State{Region: &region, X: &x, Y: &y, HP: &firstHP},
	}); err != nil {
		t.Fatal(err)
	}
	secondHP := int64(222)
	if err := wsjson.Write(ctx, conn, agentMessage{
		Type: "character.state", ProtocolVersion: agentProtocolVersion, CharacterID: registered.CharacterID,
		SessionID: registered.SessionID, SentAt: time.Now().UTC().Format(time.RFC3339),
		State: characters.State{HP: &secondHP},
	}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		character, getErr := characterStore.Get(ctx, registered.CharacterID)
		if getErr == nil && character.HP != nil && *character.HP == secondHP && character.Region != nil && *character.Region == region {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	character, getErr := characterStore.Get(ctx, registered.CharacterID)
	t.Fatalf("analytics timeout blocked/rejected later canonical state: character=%+v err=%v", character, getErr)
}

package agents

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/database"
)

func TestAgentStoreLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run agent store integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal("cannot initialize test database pool")
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	credential, err := NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	if err := store.CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM agents WHERE agent_id = $1", credential.AgentID)
	})

	gotID, err := store.AuthenticateToken(ctx, credential.Token)
	if err != nil || gotID != credential.AgentID {
		t.Fatalf("authenticate id=%q err=%v", gotID, err)
	}
	if _, err := store.AuthenticateToken(ctx, "wrong-token"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected invalid token, got %v", err)
	}

	connectedAt := time.Now().UTC().Add(-time.Second)
	if err := store.MarkConnected(ctx, credential.AgentID, connectedAt, 1, "1.0.0", "fixture-phbot"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkSeen(ctx, credential.AgentID); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDisconnected(ctx, credential.AgentID, connectedAt); err != nil {
		t.Fatal(err)
	}

	records, err := store.ListSeen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.AgentID != credential.AgentID {
			continue
		}
		if record.ProtocolVersion == nil || *record.ProtocolVersion != 1 {
			t.Fatalf("unexpected protocol version: %+v", record.ProtocolVersion)
		}
		if record.PluginVersion == nil || *record.PluginVersion != "1.0.0" {
			t.Fatalf("unexpected plugin version: %+v", record.PluginVersion)
		}
		if record.LastDisconnectedAt == nil {
			t.Fatal("disconnect timestamp was not recorded")
		}
		return
	}
	t.Fatal("connected agent was not returned by ListSeen")
}

func TestStaleDisconnectDoesNotOverwriteNewConnection(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run agent store integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal("cannot initialize test database pool")
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	credential, err := NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	if err := store.CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM agents WHERE agent_id = $1", credential.AgentID)
	})

	firstConnectedAt := time.Now().UTC().Add(-2 * time.Second)
	secondConnectedAt := firstConnectedAt.Add(time.Second)
	if err := store.MarkConnected(ctx, credential.AgentID, firstConnectedAt, 1, "1.0.0", "fixture-phbot"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkConnected(ctx, credential.AgentID, secondConnectedAt, 1, "1.0.0", "fixture-phbot"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkDisconnected(ctx, credential.AgentID, firstConnectedAt); err != nil {
		t.Fatal(err)
	}

	records, err := store.ListSeen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.AgentID != credential.AgentID {
			continue
		}
		if record.LastDisconnectedAt != nil {
			t.Fatalf("stale disconnect overwrote newer session: %v", *record.LastDisconnectedAt)
		}
		if err := store.MarkDisconnected(ctx, credential.AgentID, secondConnectedAt); err != nil {
			t.Fatal(err)
		}
		records, err = store.ListSeen(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, updated := range records {
			if updated.AgentID == credential.AgentID {
				if updated.LastDisconnectedAt == nil {
					t.Fatal("current session disconnect was not recorded")
				}
				return
			}
		}
		t.Fatal("agent record not found after current disconnect")
	}
	t.Fatal("agent record not found")
}

func TestCredentialShape(t *testing.T) {
	first, err := NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	if !ValidAgentID(first.AgentID) {
		t.Fatalf("invalid generated id: %q", first.AgentID)
	}
	if len(first.Token) < 40 || first.Token[:4] != "phm_" {
		t.Fatal("generated token has an unexpected format")
	}
	if first.Token == second.Token {
		t.Fatal("tokens must be random")
	}
}

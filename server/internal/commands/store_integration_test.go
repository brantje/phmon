package commands

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
)

type allowCapabilities struct{}

func (allowCapabilities) CommandSupport(_ string, _ uint64, _ string) (bool, string) {
	return true, ""
}

func TestCommandAdmissionIdempotencyAndSessionFencing(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run command integration tests")
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
	characterStore := characters.NewStore(pool)
	characterID, err := characterStore.Resolve(ctx, characters.Identity{Server: "slice3-test", Name: credential.AgentID[:8]})
	if err != nil {
		t.Fatal(err)
	}
	if err := characterStore.ClaimSession(ctx, credential.AgentID, characterID, 77); err != nil {
		t.Fatal(err)
	}
	region := 25000
	if err := characterStore.Snapshot(ctx, credential.AgentID, characterID, 77, characters.State{Region: &region}); err != nil {
		t.Fatal(err)
	}

	store := NewStore(pool)
	target, err := store.ResolveTarget(ctx, characterID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM command_events WHERE command_id IN (SELECT command_id FROM commands WHERE agent_id=$1)`, credential.AgentID)
		_, _ = pool.Exec(cleanup, `DELETE FROM commands WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(cleanup, `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(cleanup, `DELETE FROM characters WHERE character_id=$1`, characterID)
		_, _ = pool.Exec(cleanup, `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})

	service := NewService(store, allowCapabilities{})
	input := SubmitInput{
		CharacterID:       characterID,
		ExpectedSessionID: target.SessionID,
		Name:              "bot.stop",
		Args:              json.RawMessage(`{}`),
		IdempotencyKey:    "same-request",
	}
	first, duplicate, _, err := service.Submit(ctx, "operator", input)
	if err != nil || duplicate || first.State != StateQueued {
		t.Fatalf("first submit = %+v duplicate=%v err=%v", first, duplicate, err)
	}

	var wg sync.WaitGroup
	ids := make(chan string, 6)
	errs := make(chan error, 6)
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			command, duplicate, _, submitErr := service.Submit(ctx, "operator", input)
			if submitErr != nil {
				errs <- submitErr
				return
			}
			if !duplicate {
				errs <- errors.New("retry was not marked duplicate")
				return
			}
			ids <- command.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for submitErr := range errs {
		t.Fatal(submitErr)
	}
	for id := range ids {
		if id != first.ID {
			t.Fatalf("idempotent retry returned %s, want %s", id, first.ID)
		}
	}

	conflict := input
	conflict.Name = "bot.start"
	if _, _, _, err := service.Submit(ctx, "operator", conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting idempotency error = %v", err)
	}

	second := input
	second.IdempotencyKey = "second-action"
	if _, _, _, err := service.Submit(ctx, "operator", second); !errors.Is(err, ErrInFlight) {
		t.Fatalf("second in-flight error = %v", err)
	}

	if err := characterStore.End(ctx, credential.AgentID, characterID, 77, "left"); err != nil {
		t.Fatal(err)
	}
	if err := characterStore.ClaimSession(ctx, credential.AgentID, characterID, 78); err != nil {
		t.Fatal(err)
	}
	stale := input
	stale.IdempotencyKey = "stale-session"
	if _, _, _, err := service.Submit(ctx, "operator", stale); !errors.Is(err, ErrStaleSession) {
		t.Fatalf("stale session error = %v", err)
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `UPDATE commands SET created_at=$2 - interval '2 seconds', expires_at=$2 - interval '1 second' WHERE command_id=$1`, first.ID, now); err != nil {
		t.Fatal(err)
	}
	changed, err := store.Reconcile(ctx, now, 30*time.Second, 6*time.Minute)
	if err != nil || !changed {
		t.Fatalf("reconcile expired queued command = %v, err=%v", changed, err)
	}
	var state State
	var eventCount int
	if err := pool.QueryRow(ctx, `SELECT state FROM commands WHERE command_id=$1`, first.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM command_events WHERE command_id=$1 AND kind='expired'`, first.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if state != StateExpired || eventCount != 1 {
		t.Fatalf("reconciled command state=%q expiry events=%d", state, eventCount)
	}
}

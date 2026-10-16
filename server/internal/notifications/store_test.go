package notifications

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/database"
)

func TestCatalogAndPreferences(t *testing.T) {
	catalog := Catalog()
	seen := map[string]bool{}
	for _, category := range catalog {
		for _, kind := range category.Kinds {
			if kind.Label == "" || seen[kind.ID] {
				t.Fatalf("missing or repeated catalog kind: %q", kind.ID)
			}
			seen[kind.ID] = true
		}
	}
	if len(seen) != 30 || !seen["trade.completed"] || !seen["trade.failed"] {
		t.Fatalf("catalog contains %d kinds, want 28 canonical and two trade outcomes", len(seen))
	}
	base := DefaultPreferences()
	if len(base.PopupKinds) != 0 || len(base.BrowserKinds) != 0 || len(base.ChatChannels) != 7 || base.Validate() != nil {
		t.Fatalf("unexpected defaults: %+v", base)
	}
	for name, change := range map[string]func(*Preferences){
		"unknown kind":          func(p *Preferences) { p.PopupKinds = []string{"other.event"} },
		"duplicate kind":        func(p *Preferences) { p.PopupKinds = []string{"character.died", "character.died"} },
		"native without popup":  func(p *Preferences) { p.BrowserKinds = []string{"character.died"} },
		"unknown channel":       func(p *Preferences) { p.ChatChannels = []string{"whisper"} },
		"duplicate channel":     func(p *Preferences) { p.ChatChannels = []string{"private", "private"} },
		"chat without channels": func(p *Preferences) { p.PopupKinds = []string{"chat.message_received"}; p.ChatChannels = []string{} },
	} {
		t.Run(name, func(t *testing.T) {
			p := DefaultPreferences()
			change(&p)
			if p.Validate() == nil {
				t.Fatalf("accepted %+v", p)
			}
		})
	}
	base.PopupKinds = []string{"trade.completed", "chat.message_received"}
	base.BrowserKinds = []string{"trade.completed"}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPreferencesPersist(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run notification persistence test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	store := NewStore(pool)
	previous, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer pool.Close()
		if err := store.Save(context.Background(), previous); err != nil {
			t.Errorf("restore notification preferences: %v", err)
		}
	})
	selected := DefaultPreferences()
	selected.PopupKinds = []string{"character.died", "trade.failed"}
	selected.BrowserKinds = []string{"trade.failed"}
	selected.ChatChannels = []string{"private", "guild"}
	selected.ChatSound = true
	if err := store.Save(ctx, selected); err != nil {
		t.Fatal(err)
	}
	stored, err := NewStore(pool).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.PopupKinds) != 2 || stored.PopupKinds[1] != "trade.failed" || len(stored.BrowserKinds) != 1 || len(stored.ChatChannels) != 2 || !stored.ChatSound {
		t.Fatalf("persisted preferences=%+v", stored)
	}
}

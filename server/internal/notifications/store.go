package notifications

import (
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/events"
)

var ErrInvalidPreferences = errors.New("invalid notification preferences")

var categoryLabels = []struct{ ID, Label string }{
	{"session", "Session"}, {"character", "Character"}, {"drop", "Drops"},
	{"world", "World"}, {"job", "Job"}, {"pet", "Pets"},
	{"alchemy", "Alchemy"}, {"chat", "Chat"}, {"party", "Party"},
	{"academy", "Academy"}, {"item", "Items"}, {"trade", "Trades"},
}

var kindLabels = map[string]string{
	"session.connected": "Connected", "session.disconnected": "Disconnected",
	"session.joined_game": "Joined game", "session.teleported": "Teleported",
	"character.died": "Died", "character.level_up": "Level up", "character.attacked": "Attacked",
	"drop.item": "Normal drop", "drop.rare": "Rare drop",
	"world.unique_spawned": "Unique notice", "world.gm_spawned": "GM spawned",
	"job.hunter_trader_seen": "Trader / Hunter seen", "job.thief_seen": "Thief seen",
	"pet.transport_died": "Transport died", "pet.summoned": "Pet summoned", "pet.dismissed": "Pet dismissed",
	"alchemy.finished": "Alchemy finished", "alchemy.attempt": "Alchemy attempt",
	"chat.message_received": "Message received",
	"party.member_joined":   "Member joined", "party.member_left": "Member left",
	"academy.member_joined": "Member joined", "academy.member_left": "Member left",
	"academy.member_graduated": "Member graduated",
	"item.acquired":            "Item acquired", "item.transferred": "Item transferred",
	"item.quantity_increased": "Quantity increased", "item.quantity_decreased": "Quantity decreased",
	"trade.completed": "Trade completed", "trade.failed": "Trade failed",
}

var AllChatChannels = []string{"general", "private", "party", "guild", "union", "global", "unknown"}

type Kind struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Category struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kinds []Kind `json:"kinds"`
}

func Catalog() []Category {
	known := SelectableKinds()
	result := make([]Category, 0, len(categoryLabels))
	for _, entry := range categoryLabels {
		category := Category{ID: entry.ID, Label: entry.Label, Kinds: []Kind{}}
		for kind, owner := range known {
			if owner == entry.ID {
				category.Kinds = append(category.Kinds, Kind{ID: kind, Label: kindLabels[kind]})
			}
		}
		sort.Slice(category.Kinds, func(i, j int) bool { return category.Kinds[i].ID < category.Kinds[j].ID })
		result = append(result, category)
	}
	return result
}

// Trade reports are a separate durable source, alongside canonical activity events.
func SelectableKinds() map[string]string {
	kinds := events.CanonicalKinds()
	kinds["trade.completed"] = "trade"
	kinds["trade.failed"] = "trade"
	return kinds
}

type Preferences struct {
	PopupKinds   []string `json:"popup_kinds"`
	BrowserKinds []string `json:"browser_kinds"`
	ChatChannels []string `json:"chat_channels"`
	ChatSound    bool     `json:"chat_sound"`
}

func DefaultPreferences() Preferences {
	return Preferences{PopupKinds: []string{}, BrowserKinds: []string{}, ChatChannels: append([]string(nil), AllChatChannels...)}
}

func ValidKinds(kinds []string) bool {
	known := SelectableKinds()
	if len(kinds) > len(known) {
		return false
	}
	seen := make(map[string]bool, len(kinds))
	for _, kind := range kinds {
		if _, ok := known[kind]; !ok || seen[kind] {
			return false
		}
		seen[kind] = true
	}
	return true
}

func (p Preferences) Validate() error {
	if p.PopupKinds == nil || p.BrowserKinds == nil || p.ChatChannels == nil ||
		!ValidKinds(p.PopupKinds) || !ValidKinds(p.BrowserKinds) {
		return ErrInvalidPreferences
	}
	enabled := make(map[string]bool, len(p.PopupKinds))
	for _, kind := range p.PopupKinds {
		enabled[kind] = true
	}
	for _, kind := range p.BrowserKinds {
		if !enabled[kind] {
			return ErrInvalidPreferences
		}
	}
	allowedChannels := make(map[string]bool, len(AllChatChannels))
	for _, channel := range AllChatChannels {
		allowedChannels[channel] = true
	}
	seenChannels := make(map[string]bool, len(p.ChatChannels))
	for _, channel := range p.ChatChannels {
		if !allowedChannels[channel] || seenChannels[channel] {
			return ErrInvalidPreferences
		}
		seenChannels[channel] = true
	}
	if enabled["chat.message_received"] && len(p.ChatChannels) == 0 {
		return ErrInvalidPreferences
	}
	return nil
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Get(ctx context.Context) (Preferences, error) {
	p := DefaultPreferences()
	err := s.pool.QueryRow(ctx, `SELECT popup_kinds,browser_kinds,chat_channels,chat_sound FROM notification_preferences WHERE operator_identity='operator'`).Scan(&p.PopupKinds, &p.BrowserKinds, &p.ChatChannels, &p.ChatSound)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil
	}
	return p, err
}

func (s *Store) Save(ctx context.Context, p Preferences) error {
	if err := p.Validate(); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO notification_preferences(operator_identity,popup_kinds,browser_kinds,chat_channels,chat_sound)
VALUES('operator',$1,$2,$3,$4) ON CONFLICT(operator_identity) DO UPDATE SET
popup_kinds=EXCLUDED.popup_kinds,browser_kinds=EXCLUDED.browser_kinds,
chat_channels=EXCLUDED.chat_channels,chat_sound=EXCLUDED.chat_sound,updated_at=now()`, p.PopupKinds, p.BrowserKinds, p.ChatChannels, p.ChatSound)
	return err
}

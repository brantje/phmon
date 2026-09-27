package commands

import (
	"context"
	"errors"
	"time"

	agentdomain "phmon/server/internal/agents"
)

const (
	commandResultWait = 30 * time.Second
	walkResultWait    = 6 * time.Minute
)

type CommandSender interface {
	Send(context.Context, string, uint64, any) error
}
type CommandInvalidator interface{ Invalidate() }

type Dispatcher struct {
	store  *Store
	sender CommandSender
	live   CommandInvalidator
	wake   chan struct{}
}

func NewDispatcher(store *Store, sender CommandSender, live CommandInvalidator) *Dispatcher {
	return &Dispatcher{store: store, sender: sender, live: live, wake: make(chan struct{}, 1)}
}
func (d *Dispatcher) Notify() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}
func (d *Dispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	reconcile := time.NewTicker(time.Second)
	defer reconcile.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.wake:
		case <-ticker.C:
		case <-reconcile.C:
			workCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			changed, _ := d.store.Reconcile(workCtx, time.Now().UTC(), commandResultWait, walkResultWait)
			cancel()
			if changed && d.live != nil {
				d.live.Invalidate()
			}
		}
		workCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		ids, err := d.store.Queued(workCtx, 32)
		cancel()
		if err != nil {
			continue
		}
		for _, id := range ids {
			d.dispatch(ctx, id)
		}
	}
}
func (d *Dispatcher) dispatch(ctx context.Context, id string) {
	workCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	command, claimed, err := d.store.ClaimDispatch(workCtx, id, time.Now().UTC())
	cancel()
	if err != nil || !claimed {
		return
	}
	checkCtx, checkCancel := context.WithTimeout(ctx, 2*time.Second)
	current, checkErr := d.store.CurrentTargetMatches(checkCtx, command)
	checkCancel()
	if checkErr != nil || !current {
		_ = d.store.FailBeforeSend(ctx, id, "target_session_changed", time.Now().UTC())
		if d.live != nil {
			d.live.Invalidate()
		}
		return
	}
	remaining := time.Until(command.ExpiresAt)
	if remaining <= 0 {
		_ = d.store.FailBeforeSend(ctx, id, "dispatch_deadline", time.Now().UTC())
		if d.live != nil {
			d.live.Invalidate()
		}
		return
	}
	payload := map[string]any{"type": "command.execute", "protocol_version": 3, "command_id": command.ID, "character_id": command.CharacterID, "session_id": command.SessionID, "name": command.Name, "args": command.Args, "expires_at": commandExpiryTimestamp(command.ExpiresAt), "ttl_ms": remaining.Milliseconds()}
	sendCtx, sendCancel := context.WithDeadline(ctx, command.ExpiresAt)
	err = d.sender.Send(sendCtx, command.AgentID, command.ConnectionGeneration, payload)
	sendCancel()
	if err != nil {
		if errors.Is(err, agentdomain.ErrConnectionUnavailable) || errors.Is(err, agentdomain.ErrNotSent) {
			_ = d.store.FailBeforeSend(ctx, id, "connection_unavailable", time.Now().UTC())
			if d.live != nil {
				d.live.Invalidate()
			}
			return
		}
		_ = d.store.MarkUnknown(ctx, id, "delivery_ambiguous", time.Now().UTC())
		if d.live != nil {
			d.live.Invalidate()
		}
		return
	}
	// A result may race this write bookkeeping. MarkSent preserves any later state.
	_ = d.store.MarkSent(ctx, id, time.Now().UTC())
	if d.live != nil {
		d.live.Invalidate()
	}
}

// commandExpiryTimestamp stays compatible with the whole-second UTC parser in
// previously deployed v3 plugins. Truncation can only shorten a command's
// effective lifetime; ttl_ms remains an additional upper bound.
func commandExpiryTimestamp(expiresAt time.Time) string {
	return expiresAt.UTC().Format(time.RFC3339)
}

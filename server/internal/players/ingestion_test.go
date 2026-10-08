package players

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPendingKeepsABAAndBoundsOutage(t *testing.T) {
	s := NewStore(nil)
	base := time.Now().Add(-time.Minute)
	for i, level := range []int{100, 101, 100} {
		o := fixtureObservation("Fixture Pending", "Observed", base.Add(time.Duration(i)*time.Second))
		o.Level = intRef(level)
		if !s.submit(o) {
			t.Fatal("admission failed")
		}
	}
	if s.Status().Pending != 3 {
		t.Fatal("A/B/A was collapsed")
	}
	if err := s.flush(context.Background()); err == nil || s.Status().Pending != 3 || s.Status().LastError == "" {
		t.Fatal("failed commit lost pending data or status")
	}
	for i := 0; i < maxPendingObservations+10; i++ {
		o := fixtureObservation("Fixture Pending", fmt.Sprintf("Player%d", i), base)
		o.RuntimeID = fmt.Sprint(i + 100)
		s.submit(o)
	}
	if s.Status().Pending > maxPendingObservations || s.pending.bytes > maxPendingBytes || s.Status().Overflow == 0 {
		t.Fatal("outage queue not bounded")
	}
}

func TestPendingDatabaseUnavailableThenRecoveryKeepsLiveIndependent(t *testing.T) {
	ctx, s, server := registryFixture(t)
	available := s.pool
	config := available.Config().Copy()
	config.ConnConfig.Host = "127.0.0.1"
	config.ConnConfig.Port = 1
	config.ConnConfig.ConnectTimeout = 100 * time.Millisecond
	unavailable, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer unavailable.Close()
	s.pool = unavailable
	at := time.Now().UTC()
	level := 110
	snapshot := LiveSnapshot{Server: server, SessionID: "fixture-live-session", AgentID: "fixture-agent", CharacterID: "fixture-character", Generation: 1, Status: "observed", Region: 25000, ObservedAt: at, Players: []Player{{PlayerID: "7", Name: "Outage Fixture", Level: &level, X: 10, Y: 20}}}
	live := NewLiveStore()
	live.Apply(snapshot)
	if !s.SubmitLive(snapshot) {
		t.Fatal("registry failed transient admission")
	}
	deadline, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	err = s.flush(deadline)
	cancel()
	if err == nil || s.Status().Pending != 1 || len(live.Snapshot(server, at)) != 1 {
		t.Fatal("database failure affected live state or lost pending evidence")
	}
	s.pool = available
	if err = s.flush(ctx); err != nil {
		t.Fatal(err)
	}
	if s.Status().Pending != 0 || s.Status().LastError != "" || len(registryList(t, ctx, s, server)) != 1 {
		t.Fatal("recovery failed")
	}
	if err = s.flush(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = available.QueryRow(ctx, `SELECT count(*) FROM player_observations WHERE server_key=$1`, ServerKey(server)).Scan(&count); err != nil || count != 1 {
		t.Fatal("retry duplicated committed evidence")
	}
}

func TestPendingCoalescesMovementAndCommitsConfigurations(t *testing.T) {
	ctx, s, server := registryFixture(t)
	base := time.Now().Add(-time.Minute).UTC().Truncate(time.Microsecond)
	for i := 0; i < 1000; i++ {
		o := fixtureObservation(server, "Moving", base.Add(time.Duration(i)*time.Millisecond))
		o.Location.X = float64(i)
		if !s.submit(o) {
			t.Fatal("admission failed")
		}
	}
	if s.Status().Pending != 1 {
		t.Fatal("unchanged queue not coalesced")
	}
	t.Logf("1000 unchanged moving snapshots coalesced to %d pending observation (%d bytes)", s.Status().Pending, s.pending.bytes)
	if err := s.flush(ctx); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	changes := []Observation{}
	for i := 0; i < 128; i++ {
		o := fixtureObservation(server, fmt.Sprintf("HighChange%d", i), base.Add(2*time.Second))
		o.RuntimeID = fmt.Sprint(i + 100)
		changes = append(changes, o)
	}
	if err := s.ApplyObservations(ctx, changes); err != nil {
		t.Fatal(err)
	}
	t.Logf("128 first/change observations committed in %s", time.Since(started))
	moving, _ := s.List(ctx, Filter{Server: server, Name: "Moving"})
	r := moving.Players[0]
	if r.Location.X != 999 || !r.Location.ObservedAt.Equal(base.Add(999*time.Millisecond)) {
		t.Fatal("latest location stamp lost")
	}
	for i, plus := range []int{5, 8, 5} {
		o := fixtureObservation(server, "Gear", base.Add(time.Duration(i)*time.Second))
		o.RuntimeID = "9"
		o.Equipment = fixtureEquipment(o.ObservedAt)
		o.Equipment.Slots[0].Plus = intRef(plus)
		if !s.submit(o) {
			t.Fatal("gear admission failed")
		}
	}
	if err := s.flush(ctx); err != nil {
		t.Fatal(err)
	}
	page, _ := s.List(ctx, Filter{Server: server, Name: "Gear"})
	history, err := s.EquipmentHistory(ctx, page.Players[0].ID, 25, "")
	if err != nil || len(history.Items) != 3 {
		t.Fatalf("backlogged A/B/A %d %v", len(history.Items), err)
	}
}

func TestPendingReversalAfterCommittedCheckpoint(t *testing.T) {
	ctx, s, server := registryFixture(t)
	base := time.Now().Add(-time.Minute).UTC().Truncate(time.Microsecond)
	for i, plus := range []int{5, 8, 5} {
		o := fixtureObservation(server, "Checkpoint reversal", base.Add(time.Duration(i)*time.Second))
		o.Equipment = fixtureEquipment(o.ObservedAt)
		o.Equipment.Slots[0].Plus = intRef(plus)
		if !s.submit(o) {
			t.Fatal("reversal admission failed")
		}
		if i == 0 {
			if err := s.flush(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.flush(ctx); err != nil {
		t.Fatal(err)
	}
	record := registryList(t, ctx, s, server)[0]
	history, err := s.EquipmentHistory(ctx, record.ID, 25, "")
	if err != nil || len(history.Items) != 3 || s.Status().Pending != 0 || gearHash(record.Gear) != history.Items[0].GearHash {
		t.Fatalf("30-second checkpoint suppressed a queued reversal: %+v %v", history, err)
	}
}

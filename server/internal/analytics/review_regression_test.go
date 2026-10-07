package analytics_test

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"math"
	"os"
	"phmon/server/internal/agents"
	"phmon/server/internal/analytics"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
	"strings"
	"testing"
	"time"
)

type reviewFixture struct {
	pool            *pgxpool.Pool
	agent, server   string
	chars, sessions []string
	now             time.Time
}

func newReviewFixture(t *testing.T, n int) reviewFixture {
	t.Helper()
	ctx := context.Background()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for Slice 12 PostgreSQL regression probes")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	cred, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	if err = agents.NewStore(pool).CreateCredential(ctx, cred); err != nil {
		t.Fatal(err)
	}
	f := reviewFixture{pool: pool, agent: cred.AgentID, server: "review-" + cred.AgentID[:8], now: time.Now().UTC().Truncate(time.Second)}
	store := characters.NewStore(pool)
	for i := 0; i < n; i++ {
		id, err := store.Resolve(ctx, characters.Identity{Server: f.server, Name: fmt.Sprintf("Char%02d", i)})
		if err != nil {
			t.Fatal(err)
		}
		sid, err := store.ClaimSessionID(ctx, f.agent, id, 1)
		if err != nil {
			t.Fatal(err)
		}
		f.chars = append(f.chars, id)
		f.sessions = append(f.sessions, sid)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM characters WHERE server_key=$1`, f.server)
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE agent_id=$1::uuid`, f.agent)
		pool.Close()
	})
	return f
}
func (f reviewFixture) sample(t *testing.T, index int, at time.Time, xp, maxxp, gold int64) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(), `INSERT INTO character_metric_samples(character_id,session_id,agent_id,server_key,sampled_at,level,current_exp,max_exp,sp,gold) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,10,$6,$7,10,$8)`, f.chars[index], f.sessions[index], f.agent, f.server, at, xp, maxxp, gold)
	if err != nil {
		t.Fatal(err)
	}
}
func (f reviewFixture) death(t *testing.T, index int) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(), `INSERT INTO activity_events(event_id,schema_version,kind,category,agent_id,character_id,session_id,server_name,occurred_at,source,source_ref,payload) VALUES(gen_random_uuid(),1,'character.died','character',$1::uuid,$2::uuid,$3::uuid,$4,$5,'phmon.test','review','{}')`, f.agent, f.chars[index], f.sessions[index], f.server, f.now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
}
func (f reviewFixture) query(t *testing.T, v analytics.View) analytics.Snapshot {
	t.Helper()
	id := ""
	if v == analytics.ViewPerformance {
		id = f.chars[0]
	}
	snap, err := analytics.NewStore(f.pool).Query(context.Background(), analytics.Filter{Server: f.server, CharacterID: id, View: v, From: f.now.Add(-23 * time.Hour), To: f.now.Add(time.Second), Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestReviewLeaderboardMustNotTreatOtherAsAnEntity(t *testing.T) {
	f := newReviewFixture(t, 50)
	for i := range f.chars {
		f.death(t, i)
	}
	s := f.query(t, analytics.ViewDeaths)
	for _, m := range s.Summary {
		if m.Key == "most_deaths_character" && m.Value == "Other" {
			t.Fatalf("leader is the aggregate Other bucket: %+v", m)
		}
	}
}
func TestReviewEconomyMustKeepLastDateForAllSeries(t *testing.T) {
	f := newReviewFixture(t, 10)
	start := f.now.Add(-59 * 24 * time.Hour)
	for c := range f.chars {
		for d := 0; d < 60; d++ {
			f.sample(t, c, start.Add(time.Duration(d)*24*time.Hour), 0, 1000, int64(c+100))
		}
	}
	s, err := analytics.NewStore(f.pool).Query(context.Background(), analytics.Filter{Server: f.server, View: analytics.ViewEconomy, From: start.Add(-time.Second), To: f.now.Add(time.Second), Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range s.TimeSeries {
		if strings.HasPrefix(p.Bucket, f.now.Format("2006-01-02")) {
			return
		}
	}
	t.Fatalf("latest day disappeared: 10 series x 60 buckets -> %d points, truncated=%v, last=%s", len(s.TimeSeries), s.Truncated, s.TimeSeries[len(s.TimeSeries)-1].Bucket)
}
func TestReviewRetentionMustContinueWhenOnlyOneTableHasBacklog(t *testing.T) {
	f := newReviewFixture(t, 1)
	_, err := f.pool.Exec(context.Background(), `INSERT INTO character_metric_samples(character_id,session_id,agent_id,server_key,sampled_at) SELECT $1::uuid,$2::uuid,$3::uuid,$4,$5::timestamptz-n*interval '1 second' FROM generate_series(1,2500) n`, f.chars[0], f.sessions[0], f.agent, f.server, f.now.Add(-100*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { analytics.NewStore(f.pool).RunRetention(ctx, 90, 6*time.Hour); close(done) }()
	var remaining int
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		err = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM character_metric_samples WHERE server_key=$1`, f.server).Scan(&remaining)
		if err != nil {
			t.Fatal(err)
		}
		if remaining == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	if remaining != 0 {
		t.Fatalf("after retention pass %d of 2500 expired character rows remain although guild table is empty", remaining)
	}
}
func TestReviewCurrentLevelPaceMustNotMixXPRequirements(t *testing.T) {
	f := newReviewFixture(t, 1)
	for i := 0; i < 18; i++ {
		xp, req := int64(100+i*10), int64(1000)
		if i >= 9 {
			xp, req = int64(1000+(i-9)*100), 10000
		}
		f.sample(t, 0, f.now.Add(time.Duration(i-17)*10*time.Second), xp, req, 1000)
	}
	p := f.query(t, analytics.ViewPerformance).Performance
	if p.XPPercentPerHour == nil || math.Abs(*p.XPPercentPerHour-360) > 0.01 {
		t.Fatalf("current requirement pace should be 360 percent/hour from latest compatible 80s; got %v percent/hour", *p.XPPercentPerHour)
	}
}
func TestReviewRollingEventTilesMustWorkWithoutNumericHistory(t *testing.T) {
	f := newReviewFixture(t, 1)
	f.death(t, 0)
	p := f.query(t, analytics.ViewPerformance).Performance
	if p.Deaths24h != 1 {
		t.Fatalf("one recorded death but no metric samples reports %d deaths", p.Deaths24h)
	}
}
func TestReviewResetMustInvalidateOldLiveWindow(t *testing.T) {
	f := newReviewFixture(t, 1)
	for i := 0; i < 7; i++ {
		f.sample(t, 0, f.now.Add(time.Duration(i-8)*10*time.Second), int64(i*10), 1000, int64(i*10))
	}
	store := analytics.NewStore(f.pool)
	_, err := store.ResetRateWindow(context.Background(), f.chars[0], "Char00", "operator", "review-reset-idempotency")
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Query(context.Background(), analytics.Filter{Server: f.server, CharacterID: f.chars[0], View: analytics.ViewPerformance, From: f.now.Add(-23 * time.Hour), To: f.now.Add(-time.Second), Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Performance.Rates["gold"].HasRate {
		t.Fatalf("live client whose fixed To precedes reset still returns pre-reset rate: %+v", s.Performance.Rates["gold"])
	}
}
func TestReviewLargeGoldDeltaMustKeepOneUnitGain(t *testing.T) {
	from := time.Now().UTC()
	base := int64(9007199254740992)
	samples := []analytics.MetricSample{}
	for i := 0; i < 7; i++ {
		gold := base
		if i == 6 {
			gold++
		}
		samples = append(samples, analytics.MetricSample{SessionID: "one", At: from.Add(time.Duration(i) * 10 * time.Second), Gold: &gold})
	}
	got := analytics.CalculateBalanceRate(samples, func(s analytics.MetricSample) *int64 { return s.Gold }, from, from.Add(time.Minute))
	if got.Delta != 1 || got.PerHour != 60 {
		t.Fatalf("one-unit gain became %+v", got)
	}
}

func TestReviewGuildLeaderMustSortGoldNumerically(t *testing.T) {
	f := newReviewFixture(t, 1)
	for guild, gold := range map[string]int64{"rich": 10000, "poor": 900} {
		_, err := f.pool.Exec(context.Background(), `INSERT INTO guild_gold_samples(server_key,guild_key,observer_character_id,session_id,resource_revision,sampled_at,gold) VALUES($1,$2,$3::uuid,$4::uuid,1,$5,$6)`, f.server, guild, f.chars[0], f.sessions[0], f.now, gold)
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err := analytics.NewStore(f.pool).Query(context.Background(), analytics.Filter{Server: f.server, View: analytics.ViewEconomy, From: f.now.Add(-time.Hour), To: f.now.Add(time.Second), Timezone: "UTC", BalanceScope: "guild_storage"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Breakdown[0].Label != "rich" {
		t.Fatalf("guild leaderboard sorted decimal strings instead of integers: %+v", s.Breakdown)
	}
}
func TestReviewDegreeBreakdownMustAccountForUnknownTaxonomy(t *testing.T) {
	f := newReviewFixture(t, 1)
	for _, model := range []int64{77, 99} {
		_, err := f.pool.Exec(context.Background(), `INSERT INTO activity_events(event_id,schema_version,kind,category,agent_id,character_id,session_id,server_name,occurred_at,source,source_ref,item_model,payload) VALUES(gen_random_uuid(),1,'drop.rare','drop',$1::uuid,$2::uuid,$3::uuid,$4,$5,'phmon.test','review',$6,'{}')`, f.agent, f.chars[0], f.sessions[0], f.server, f.now, model)
		if err != nil {
			t.Fatal(err)
		}
	}
	store := analytics.NewStore(f.pool)
	store.SetItemTaxonomy(func(server string, model int64, code string) (string, string, bool) {
		if model == 77 {
			return "Armor", "10", true
		}
		return "", "", false
	}, nil, nil)
	s, err := store.Query(context.Background(), analytics.Filter{Server: f.server, View: analytics.ViewRareDrops, From: f.now.Add(-time.Hour), To: f.now.Add(time.Second), Timezone: "UTC", GroupBy: "degree"})
	if err != nil {
		t.Fatal(err)
	}
	sum := 0
	for _, p := range s.Breakdown {
		n := 0
		fmt.Sscan(p.Value, &n)
		sum += n
	}
	if sum != 2 {
		t.Fatalf("total=%s but degree breakdown accounts for %d: %+v", s.Total, sum, s.Breakdown)
	}
}

func TestReviewVisibleProgressMustBatchFiftyCharacters(t *testing.T) {
	f := newReviewFixture(t, 50)
	snapshot, err := analytics.NewStore(f.pool).Query(context.Background(), analytics.Filter{
		Server: f.server, CharacterIDs: f.chars, View: analytics.ViewPerformance,
		From: f.now.Add(-24 * time.Hour), To: f.now, Timezone: "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.PerformanceBatch) != 50 {
		t.Fatalf("single visible-character batch returned %d of 50 progress records", len(snapshot.PerformanceBatch))
	}
	if snapshot.Status != "limited" {
		t.Fatalf("missing per-character samples should remain explicit as limited/insufficient, got %q", snapshot.Status)
	}
}

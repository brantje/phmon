package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pingFunc func(context.Context) error

func (f pingFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestHealth(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		dbErr              error
		code               int
		want               Health
		calls              int
	}{
		{"live independent of database", "GET", "/healthz", errors.New("secret"), 200, Health{Status: "ok"}, 0},
		{"ready", "GET", "/readyz", nil, 200, Health{Status: "ok", Database: "ok"}, 1},
		{"not ready", "GET", "/readyz", errors.New("password=secret"), 503, Health{Status: "unavailable", Database: "unavailable"}, 1},
		{"wrong method", "POST", "/readyz", nil, 405, Health{}, 0},
		{"unknown route", "GET", "/missing", nil, 404, Health{}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := New(Dependencies{Database: pingFunc(func(ctx context.Context) error {
				calls++
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 2*time.Second {
					t.Fatal("missing bounded database timeout")
				}
				return tc.dbErr
			})})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.code || calls != tc.calls {
				t.Fatalf("status=%d calls=%d", w.Code, calls)
			}
			if tc.code == 200 || tc.code == 503 {
				var got Health
				if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got != tc.want {
					t.Fatalf("got %+v want %+v", got, tc.want)
				}
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("health must not be cached")
				}
			}
		})
	}
}

func TestReadinessPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := New(Dependencies{Database: pingFunc(func(ctx context.Context) error { return ctx.Err() })})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil).WithContext(ctx))
	if w.Code != 503 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestPostgresReadiness(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run the real PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal("cannot initialize test database pool")
	}
	defer pool.Close()
	w := httptest.NewRecorder()
	New(Dependencies{Database: pool}).ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil).WithContext(ctx))
	if w.Code != http.StatusOK {
		t.Fatalf("PostgreSQL readiness failed: %d", w.Code)
	}
}

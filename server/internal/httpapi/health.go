package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Pinger interface{ Ping(context.Context) error }

type Health struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
}

func New(db Pinger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		respond(w, http.StatusOK, Health{Status: "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			respond(w, http.StatusServiceUnavailable, Health{Status: "unavailable", Database: "unavailable"})
			return
		}
		respond(w, http.StatusOK, Health{Status: "ok", Database: "ok"})
	})
	return mux
}

func respond(w http.ResponseWriter, code int, health Health) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(health)
}

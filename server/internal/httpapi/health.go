package httpapi

import (
	"context"
	"net/http"
	"time"
)

type Pinger interface{ Ping(context.Context) error }

type Health struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
}

func registerHealth(mux *http.ServeMux, db Pinger) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, Health{Status: "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			respondJSON(w, http.StatusServiceUnavailable, Health{Status: "unavailable", Database: "unavailable"})
			return
		}
		respondJSON(w, http.StatusOK, Health{Status: "ok", Database: "ok"})
	})
}

package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Health(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			WriteError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database connection failed")
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

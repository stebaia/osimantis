package server

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// statusRecorder cattura lo status code scritto dall'handler per poterlo loggare.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// auth è un middleware che richiede l'header "Authorization: Bearer <token>" e
// lo confronta con il token configurato. /health è esente: serve a probe e
// orchestratori che non hanno (e non devono avere) il token.
//
// Il confronto usa subtle.ConstantTimeCompare per non rivelare il token tramite
// timing. Token mancante o errato → 401.
func auth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			const prefix = "Bearer "
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, prefix) {
				writeError(w, http.StatusUnauthorized, "autenticazione richiesta")
				return
			}
			got := strings.TrimPrefix(h, prefix)
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				writeError(w, http.StatusUnauthorized, "token non valido")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// logging è un middleware che registra metodo, path, status e durata di ogni
// richiesta con slog.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"durata", time.Since(start),
		)
	})
}

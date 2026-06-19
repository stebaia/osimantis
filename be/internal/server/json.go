package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeJSON serializza v come JSON con lo status dato e il Content-Type corretto.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Header e status sono già stati scritti: possiamo solo loggare.
		slog.Error("encoding risposta JSON fallito", "err", err)
	}
}

// writeError risponde con un oggetto JSON {"error": "..."} e lo status dato.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

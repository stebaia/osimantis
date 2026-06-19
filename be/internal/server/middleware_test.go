package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler risponde 200: serve a verificare che auth lasci passare.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthMiddleware(t *testing.T) {
	const token = "segreto-123"
	h := auth(token)(okHandler())

	cases := []struct {
		name       string
		path       string
		authHeader string
		want       int
	}{
		{"health esente senza token", "/health", "", http.StatusOK},
		{"chat senza header", "/chat", "", http.StatusUnauthorized},
		{"chat con token errato", "/chat", "Bearer sbagliato", http.StatusUnauthorized},
		{"chat senza prefisso Bearer", "/chat", token, http.StatusUnauthorized},
		{"chat con token corretto", "/chat", "Bearer " + token, http.StatusOK},
		{"graph con token corretto", "/graph", "Bearer " + token, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, atteso %d", rec.Code, tc.want)
			}
		})
	}
}

package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test della mappatura e del retry usando un server HTTP fittizio: nessuna
// chiamata all'API reale, deterministici.

func TestGeminiChatTextResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verifica che la chiave sia passata nell'header corretto.
		if got := r.Header.Get("x-goog-api-key"); got != "test-key" {
			t.Errorf("header api key = %q", got)
		}
		// Verifica che il body contenga il testo dell'utente.
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "ciao") {
			t.Errorf("body senza 'ciao': %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"ciao a te"}]}}]}`)
	}))
	defer srv.Close()

	g := NewGemini("test-key", WithBaseURL(srv.URL))
	turn, err := g.Chat(context.Background(), []Turn{{Role: RoleUser, Text: "ciao"}}, nil)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if turn.Role != RoleModel || turn.Text != "ciao a te" {
		t.Errorf("turn = %+v", turn)
	}
	if len(turn.ToolCalls) != 0 {
		t.Errorf("attese 0 tool call, %d", len(turn.ToolCalls))
	}
}

func TestGeminiChatFunctionCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verifica che i toolDefs vengano inviati.
		var req geminiRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		if len(req.Tools) != 1 || len(req.Tools[0].FunctionDeclarations) != 1 {
			t.Errorf("tool definitions non inviate correttamente: %+v", req.Tools)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"find_node","args":{"query":"Mura"}}}]}}]}`)
	}))
	defer srv.Close()

	g := NewGemini("k", WithBaseURL(srv.URL))
	toolDefs := []map[string]any{{"name": "find_node", "description": "x"}}
	turn, err := g.Chat(context.Background(), []Turn{{Role: RoleUser, Text: "chi è Mura?"}}, toolDefs)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("attesa 1 tool call, %d", len(turn.ToolCalls))
	}
	tc := turn.ToolCalls[0]
	if tc.Name != "find_node" || tc.Args["query"] != "Mura" {
		t.Errorf("tool call = %+v", tc)
	}
}

func TestGeminiRetryOn429(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// Primo tentativo: 429 con Retry-After breve per non rallentare il test.
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"code":429,"message":"quota"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]}}]}`)
	}))
	defer srv.Close()

	g := NewGemini("k", WithBaseURL(srv.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	turn, err := g.Chat(ctx, []Turn{{Role: RoleUser, Text: "ciao"}}, nil)
	if err != nil {
		t.Fatalf("Chat dopo retry: %v", err)
	}
	if turn.Text != "ok" {
		t.Errorf("text = %q", turn.Text)
	}
	if calls != 2 {
		t.Errorf("chiamate = %d, attese 2 (1 fallita + 1 ok)", calls)
	}
}

func TestGeminiBadResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":400,"message":"API key invalida"}}`)
	}))
	defer srv.Close()

	g := NewGemini("k", WithBaseURL(srv.URL))
	_, err := g.Chat(context.Background(), []Turn{{Role: RoleUser, Text: "x"}}, nil)
	if err == nil {
		t.Fatal("atteso errore")
	}
	var bad *BadResponseError
	if !errors.As(err, &bad) {
		t.Fatalf("atteso *BadResponseError, ottenuto %T: %v", err, err)
	}
	if bad.StatusCode != http.StatusBadRequest || !strings.Contains(bad.Reason, "API key") {
		t.Errorf("errore = %+v", bad)
	}
}

// Package server espone l'API HTTP: router, middleware e handler.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"relazioni-server/internal/agent"
	"relazioni-server/internal/llm"
	"relazioni-server/internal/tools"

	"github.com/jackc/pgx/v5/pgxpool"
)

// requestTimeout limita la durata di una richiesta. Generoso perché /chat può
// fare più giri LLM↔tool, e l'LLM è lento.
const requestTimeout = 60 * time.Second

// Server raccoglie le dipendenze degli handler HTTP.
type Server struct {
	pool     *pgxpool.Pool
	llm      llm.LLM
	exec     agent.ToolExecutor
	toolDefs []map[string]any
	apiToken string
}

// New costruisce il Server con le sue dipendenze. apiToken è il bearer token
// richiesto dal middleware di autenticazione.
func New(pool *pgxpool.Pool, client llm.LLM, apiToken string) *Server {
	return &Server{
		pool:     pool,
		llm:      client,
		exec:     agent.NewToolExecutor(tools.NewRegistry(), pool),
		toolDefs: tools.Definitions,
		apiToken: apiToken,
	}
}

// Handler costruisce il router con middleware e rotte (pattern per-metodo,
// net/http, niente framework). I middleware avvolgono il mux dall'interno verso
// l'esterno: logging copre tutto (incluse le risposte 401 di auth), auth fa da
// guardia prima degli handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /chat", s.handleChat)
	mux.HandleFunc("GET /graph", s.handleGraph)
	mux.HandleFunc("GET /wiki", s.handleWikiList)
	mux.HandleFunc("GET /wiki/{id}", s.handleWikiPage)
	return logging(auth(s.apiToken)(mux))
}

// handleHealth verifica la connessione al DB.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database non disponibile")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// chatRequest / chatResponse sono i contratti dell'endpoint /chat.
type chatRequest struct {
	Text string `json:"text"`
}
type chatResponse struct {
	Reply string `json:"reply"`
}

// handleChat esegue l'agente sul testo dell'utente e ritorna la risposta.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	// Limitiamo la dimensione del body per non leggere payload abnormi.
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB
	if err != nil {
		writeError(w, http.StatusBadRequest, "lettura body fallita")
		return
	}
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON non valido")
		return
	}
	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "il campo 'text' è obbligatorio")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	// Pseudonimizzazione per-richiesta: i nomi reali estratti dal grafo vengono
	// sostituiti con pseudonimi stabili prima di raggiungere l'LLM. La mappa è
	// fresca a ogni conversazione, così nulla sfugge tra richieste diverse.
	exec := agent.NewPseudonymizingExecutor(s.exec)
	reply, err := agent.RunAgent(ctx, s.llm, exec, s.toolDefs, req.Text)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, "timeout nell'elaborazione della richiesta")
			return
		}
		writeError(w, http.StatusInternalServerError, "errore nell'elaborazione: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, chatResponse{Reply: reply})
}

// handleGraph restituisce l'intero grafo (nodi + archi) per l'app Flutter.
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	graph, err := tools.GraphDump(ctx, s.pool)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lettura del grafo fallita")
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

// handleWikiList restituisce l'elenco delle persone (per il frontend). È una
// vista calcolata sul DB, non salva nulla.
func (s *Server) handleWikiList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestCtx(r, 10*time.Second)
	defer cancel()

	people, err := tools.WikiList(ctx, s.pool)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lettura elenco fallita")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"people": people})
}

// handleWikiPage restituisce la scheda di una persona (dati, legami, eventi
// recenti) come JSON, dato il suo id. La presentazione la fa il frontend.
func (s *Server) handleWikiPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id non valido")
		return
	}

	ctx, cancel := requestCtx(r, 10*time.Second)
	defer cancel()

	page, err := tools.WikiPage(ctx, s.pool, id)
	if errors.Is(err, tools.ErrNodeNotFound) {
		writeError(w, http.StatusNotFound, "persona non trovata")
		return
	}
	if err != nil {
		slog.Error("wiki page", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "generazione scheda fallita")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

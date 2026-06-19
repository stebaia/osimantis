package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"relazioni-server/internal/tools"
)

// Endpoint REST per il frontend: ricerca, dettaglio, creazione/modifica manuale
// di persone e legami, e il feed cronologico. Tutti dietro lo stesso token.

// decodeBody legge e deserializza il body JSON con un limite di dimensione.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB
	if err != nil {
		writeError(w, http.StatusBadRequest, "lettura body fallita")
		return false
	}
	if err := json.Unmarshal(body, dst); err != nil {
		writeError(w, http.StatusBadRequest, "JSON non valido")
		return false
	}
	return true
}

// pathID estrae e valida un id intero dal path. Scrive 400 e ritorna false se
// non valido.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "id non valido")
		return 0, false
	}
	return id, true
}

// handleSearchPeople: GET /people?q=...  ricerca persone (q opzionale).
func (s *Server) handleSearchPeople(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestCtx(r, 10*time.Second)
	defer cancel()

	q := r.URL.Query().Get("q")
	people, err := tools.SearchPeople(ctx, s.pool, q, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ricerca fallita")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"people": people})
}

// handlePersonDetail: GET /people/{id}  scheda completa (= vista wiki).
func (s *Server) handlePersonDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
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
		writeError(w, http.StatusInternalServerError, "lettura fallita")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// handleCreatePerson: POST /people  crea una persona manualmente.
func (s *Server) handleCreatePerson(w http.ResponseWriter, r *http.Request) {
	var in tools.PersonInput
	if !decodeBody(w, r, &in) {
		return
	}
	ctx, cancel := requestCtx(r, 10*time.Second)
	defer cancel()

	node, err := tools.CreatePerson(ctx, s.pool, in)
	if errors.Is(err, tools.ErrInvalid) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "creazione fallita")
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

// handleUpdatePerson: PATCH /people/{id}  aggiorna name/aliases/data (parziale).
func (s *Server) handleUpdatePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in tools.PersonInput
	if !decodeBody(w, r, &in) {
		return
	}
	ctx, cancel := requestCtx(r, 10*time.Second)
	defer cancel()

	node, err := tools.UpdatePerson(ctx, s.pool, id, in)
	if errors.Is(err, tools.ErrNodeNotFound) {
		writeError(w, http.StatusNotFound, "persona non trovata")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "aggiornamento fallito")
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// handleDeletePerson: DELETE /people/{id}  rimuove una persona (e i suoi archi).
func (s *Server) handleDeletePerson(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx, cancel := requestCtx(r, 10*time.Second)
	defer cancel()

	err := tools.DeletePerson(ctx, s.pool, id)
	if errors.Is(err, tools.ErrNodeNotFound) {
		writeError(w, http.StatusNotFound, "persona non trovata")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "eliminazione fallita")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

// handleCreateLink: POST /links  crea o aggiorna un legame.
func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	var in tools.LinkInput
	if !decodeBody(w, r, &in) {
		return
	}
	ctx, cancel := requestCtx(r, 10*time.Second)
	defer cancel()

	edge, err := tools.CreateLink(ctx, s.pool, in)
	if errors.Is(err, tools.ErrInvalid) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "creazione legame fallita")
		return
	}
	writeJSON(w, http.StatusCreated, edge)
}

// handleDeleteLink: DELETE /links/{id}  rimuove un legame.
func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx, cancel := requestCtx(r, 10*time.Second)
	defer cancel()

	err := tools.DeleteLink(ctx, s.pool, id)
	if errors.Is(err, tools.ErrNodeNotFound) {
		writeError(w, http.StatusNotFound, "legame non trovato")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rimozione legame fallita")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

// handleFeed: GET /feed?limit=...  changelog cronologico.
func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestCtx(r, 10*time.Second)
	defer cancel()

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	items, err := tools.FeedList(ctx, s.pool, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lettura feed fallita")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"feed": items})
}

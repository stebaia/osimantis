package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// La "wiki" è una VISTA calcolata al volo dal DB: non salva file. Queste funzioni
// sono esportate (non ToolFn) perché servono direttamente agli endpoint HTTP, come
// GraphDump. Restituiscono dati strutturati (JSON): la presentazione (scheda
// persona) la fa il frontend Flutter, il backend espone solo dati.

// WikiNeighbor è un legame della persona verso un altro nodo, con i metadati
// dell'arco (inclusa l'eventuale nota in edges.data.note).
type WikiNeighbor struct {
	NodeID    int64   `json:"node_id"`
	Name      string  `json:"name"`
	NodeType  string  `json:"node_type"`
	Relation  string  `json:"relation"`
	Direction string  `json:"direction"` // "out" | "in"
	Weight    float64 `json:"weight"`
	LastSeen  *string `json:"last_seen"`
	Note      string  `json:"note,omitempty"`
}

// WikiEvent è un evento recente che coinvolge la persona.
type WikiEvent struct {
	ID         int64   `json:"id"`
	RawText    string  `json:"raw_text"`
	Summary    *string `json:"summary"`
	OccurredAt string  `json:"occurred_at"`
}

// WikiPageData raccoglie la scheda completa di una persona (per il frontend).
type WikiPageData struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	Aliases   []string       `json:"aliases"`
	Data      map[string]any `json:"data"`
	Neighbors []WikiNeighbor `json:"neighbors"`
	Events    []WikiEvent    `json:"events"`
}

// WikiListItem è una voce dell'elenco persone.
type WikiListItem struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

// ErrNodeNotFound segnala che l'id richiesto non esiste (o non è una persona).
var ErrNodeNotFound = errors.New("nodo non trovato")

// WikiList restituisce tutte le persone in ordine alfabetico, per l'indice.
func WikiList(ctx context.Context, pool *pgxpool.Pool) ([]WikiListItem, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, name, aliases FROM nodes WHERE type = 'person' ORDER BY lower(name)`)
	if err != nil {
		return nil, fmt.Errorf("wiki list: %w", err)
	}
	defer rows.Close()

	out := []WikiListItem{}
	for rows.Next() {
		var it WikiListItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Aliases); err != nil {
			return nil, fmt.Errorf("wiki list scan: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("wiki list rows: %w", err)
	}
	return out, nil
}

// WikiPage compone la scheda di una persona: dati del nodo, legami ed eventi
// recenti. Restituisce ErrNodeNotFound se l'id non esiste o non è una persona.
func WikiPage(ctx context.Context, pool *pgxpool.Pool, id int64) (WikiPageData, error) {
	page := WikiPageData{ID: id, Aliases: []string{}, Data: map[string]any{}}

	// Nodo persona.
	var data []byte
	err := pool.QueryRow(ctx,
		`SELECT name, aliases, data FROM nodes WHERE id = $1 AND type = 'person'`, id).
		Scan(&page.Name, &page.Aliases, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return WikiPageData{}, ErrNodeNotFound
	}
	if err != nil {
		return WikiPageData{}, fmt.Errorf("wiki page nodo: %w", err)
	}
	if err := json.Unmarshal(data, &page.Data); err != nil {
		return WikiPageData{}, fmt.Errorf("wiki page decode data: %w", err)
	}

	// Legami (stessa logica di get_neighbors), con la nota dall'arco.
	page.Neighbors, err = wikiNeighbors(ctx, pool, id)
	if err != nil {
		return WikiPageData{}, err
	}

	// Eventi recenti che coinvolgono la persona.
	page.Events, err = wikiEvents(ctx, pool, id)
	if err != nil {
		return WikiPageData{}, err
	}

	return page, nil
}

func wikiNeighbors(ctx context.Context, pool *pgxpool.Pool, id int64) ([]WikiNeighbor, error) {
	const sql = `
SELECT e.type, e.weight, e.last_seen, e.data,
       CASE WHEN e.from_id = $1 THEN 'out' ELSE 'in' END AS direction,
       n.id, n.name, n.type
FROM edges e
JOIN nodes n ON n.id = CASE WHEN e.from_id = $1 THEN e.to_id ELSE e.from_id END
WHERE e.from_id = $1 OR e.to_id = $1
ORDER BY e.weight DESC, e.last_seen DESC NULLS LAST`

	rows, err := pool.Query(ctx, sql, id)
	if err != nil {
		return nil, fmt.Errorf("wiki neighbors: %w", err)
	}
	defer rows.Close()

	out := []WikiNeighbor{}
	for rows.Next() {
		var nb WikiNeighbor
		var edgeData []byte
		var lastSeen *time.Time // timestamptz: scansiona in time.Time, non string
		if err := rows.Scan(&nb.Relation, &nb.Weight, &lastSeen, &edgeData,
			&nb.Direction, &nb.NodeID, &nb.Name, &nb.NodeType); err != nil {
			return nil, fmt.Errorf("wiki neighbors scan: %w", err)
		}
		if lastSeen != nil {
			s := lastSeen.Format(time.RFC3339)
			nb.LastSeen = &s
		}
		// Estrai la nota dell'arco, se presente.
		var d map[string]any
		if err := json.Unmarshal(edgeData, &d); err == nil {
			if note, ok := d["note"].(string); ok {
				nb.Note = note
			}
		}
		out = append(out, nb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("wiki neighbors rows: %w", err)
	}
	return out, nil
}

func wikiEvents(ctx context.Context, pool *pgxpool.Pool, id int64) ([]WikiEvent, error) {
	const sql = `
SELECT ev.id, ev.raw_text, ev.summary, ev.occurred_at
FROM events ev
WHERE ev.id IN (SELECT event_id FROM event_participants WHERE node_id = $1)
ORDER BY ev.occurred_at DESC
LIMIT 20`

	rows, err := pool.Query(ctx, sql, id)
	if err != nil {
		return nil, fmt.Errorf("wiki events: %w", err)
	}
	defer rows.Close()

	out := []WikiEvent{}
	for rows.Next() {
		var ev WikiEvent
		var occurredAt time.Time // timestamptz: scansiona in time.Time, non string
		if err := rows.Scan(&ev.ID, &ev.RawText, &ev.Summary, &occurredAt); err != nil {
			return nil, fmt.Errorf("wiki events scan: %w", err)
		}
		ev.OccurredAt = occurredAt.Format(time.RFC3339)
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("wiki events rows: %w", err)
	}
	return out, nil
}

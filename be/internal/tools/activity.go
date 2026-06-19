package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// activity_log: changelog append-only di tutto ciò che cambia nel grafo. Lo
// alimentano sia gli endpoint REST manuali (actor 'user') sia, in futuro, i tool
// dell'agente (actor 'agent'). Il feed (GET /feed) lo legge in ordine cronologico.

// Actor distingue chi ha prodotto l'attività.
type Actor string

const (
	ActorUser  Actor = "user"  // scrittura manuale dal frontend
	ActorAgent Actor = "agent" // scrittura fatta dall'LLM via tool
)

// Querier è ciò che serve per scrivere il log: lo soddisfano sia *pgxpool.Pool
// sia pgx.Tx, così LogActivity può girare dentro la stessa transazione della
// scrittura che descrive (atomicità: o entrambe o nessuna).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// LogActivity inserisce una riga nel changelog. data può essere nil.
func LogActivity(ctx context.Context, q Querier, action string, entityType string, entityID int64, actor Actor, summary string, data map[string]any) error {
	payload := []byte("{}")
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("activity log marshal: %w", err)
		}
		payload = b
	}
	const sql = `
INSERT INTO activity_log (action, entity_type, entity_id, actor, summary, data)
VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := q.Exec(ctx, sql, action, entityType, entityID, string(actor), summary, payload); err != nil {
		return fmt.Errorf("activity log insert: %w", err)
	}
	return nil
}

// FeedItem è una voce del changelog (per GET /feed).
type FeedItem struct {
	ID         int64          `json:"id"`
	Action     string         `json:"action"`
	EntityType string         `json:"entity_type"`
	EntityID   *int64         `json:"entity_id"`
	Actor      string         `json:"actor"`
	Summary    string         `json:"summary"`
	Data       map[string]any `json:"data"`
	CreatedAt  string         `json:"created_at"`
}

// FeedList restituisce le ultime attività in ordine cronologico inverso
// (più recenti prima). limit limita il numero di voci (default 50).
func FeedList(ctx context.Context, pool *pgxpool.Pool, limit int) ([]FeedItem, error) {
	if limit <= 0 {
		limit = 50
	}
	const sql = `
SELECT id, action, entity_type, entity_id, actor, summary, data, created_at
FROM activity_log
ORDER BY created_at DESC, id DESC
LIMIT $1`

	rows, err := pool.Query(ctx, sql, limit)
	if err != nil {
		return nil, fmt.Errorf("feed list: %w", err)
	}
	defer rows.Close()

	out := []FeedItem{}
	for rows.Next() {
		var it FeedItem
		var data []byte
		var createdAt time.Time
		if err := rows.Scan(&it.ID, &it.Action, &it.EntityType, &it.EntityID,
			&it.Actor, &it.Summary, &data, &createdAt); err != nil {
			return nil, fmt.Errorf("feed list scan: %w", err)
		}
		if err := json.Unmarshal(data, &it.Data); err != nil {
			return nil, fmt.Errorf("feed list decode data: %w", err)
		}
		it.CreatedAt = createdAt.Format(time.RFC3339)
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("feed list rows: %w", err)
	}
	return out, nil
}

package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInvalid segnala un input non valido lato client (→ HTTP 400). errInvalid lo
// avvolge con un messaggio specifico, mantenendo errors.Is(err, ErrInvalid).
var ErrInvalid = errors.New("input non valido")

func errInvalid(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalid, msg)
}

// CRUD manuale (REST) su persone e legami. A differenza dei ToolFn (pensati per
// l'LLM, args map[string]any), queste funzioni hanno firme tipizzate per gli
// handler HTTP. Ogni scrittura registra una voce nel changelog (activity_log).

// PersonInput sono i campi modificabili di una persona dal frontend.
type PersonInput struct {
	Name    string         `json:"name"`
	Aliases []string       `json:"aliases"`
	Data    map[string]any `json:"data"`
}

// SearchPeople cerca persone per nome o alias (case-insensitive + fuzzy), come
// find_node ma limitato a type='person'. q vuoto restituisce le prime persone.
func SearchPeople(ctx context.Context, pool *pgxpool.Pool, q string, limit int) ([]WikiListItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var (
		sql  string
		args []any
	)
	if q == "" {
		sql = `SELECT id, name, aliases FROM nodes WHERE type='person' ORDER BY lower(name) LIMIT $1`
		args = []any{limit}
	} else {
		sql = `
SELECT id, name, aliases
FROM nodes
WHERE type='person'
  AND (
        lower(name) = lower($1)
        OR EXISTS (SELECT 1 FROM unnest(aliases) a WHERE lower(a) = lower($1))
        OR lower(name) % lower($1)
        OR lower(name) LIKE '%' || lower($1) || '%'
      )
ORDER BY
    CASE
        WHEN lower(name) = lower($1) THEN 3
        WHEN EXISTS (SELECT 1 FROM unnest(aliases) a WHERE lower(a) = lower($1)) THEN 2
        ELSE 1
    END DESC,
    similarity(lower(name), lower($1)) DESC
LIMIT $2`
		args = []any{q, limit}
	}

	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("search people: %w", err)
	}
	defer rows.Close()

	out := []WikiListItem{}
	for rows.Next() {
		var it WikiListItem
		if err := rows.Scan(&it.ID, &it.Name, &it.Aliases); err != nil {
			return nil, fmt.Errorf("search people scan: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search people rows: %w", err)
	}
	return out, nil
}

// CreatePerson crea una persona manualmente e logga l'attività.
func CreatePerson(ctx context.Context, pool *pgxpool.Pool, in PersonInput) (nodeResult, error) {
	if in.Name == "" {
		return nodeResult{}, errInvalid("il campo 'name' è obbligatorio")
	}
	if in.Aliases == nil {
		in.Aliases = []string{}
	}
	data, err := jsonObject(coalesceData(in.Data))
	if err != nil {
		return nodeResult{}, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nodeResult{}, fmt.Errorf("create person begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const sql = `
INSERT INTO nodes (type, name, aliases, data)
VALUES ('person', $1, $2, $3)
RETURNING id, type, name, aliases, data`
	node, err := scanNode(tx.QueryRow(ctx, sql, in.Name, in.Aliases, data))
	if err != nil {
		return nodeResult{}, fmt.Errorf("create person: %w", err)
	}

	if err := LogActivity(ctx, tx, "person_created", "node", node.ID, ActorUser,
		"Creata persona "+node.Name, nil); err != nil {
		return nodeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nodeResult{}, fmt.Errorf("create person commit: %w", err)
	}
	return node, nil
}

// UpdatePerson aggiorna name, aliases e/o data di una persona. I campi a zero
// (name vuoto, aliases/data nil) non vengono toccati: è un PATCH parziale, con
// merge di data e degli alias (dedup). Restituisce ErrNodeNotFound se manca.
func UpdatePerson(ctx context.Context, pool *pgxpool.Pool, id int64, in PersonInput) (nodeResult, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nodeResult{}, fmt.Errorf("update person begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// COALESCE/merge fatto in SQL: name solo se non vuoto; aliases uniti e
	// deduplicati se forniti; data merge JSONB se fornito.
	var nameArg any
	if in.Name != "" {
		nameArg = in.Name
	}
	dataArg := []byte("{}")
	if in.Data != nil {
		b, err := jsonObject(in.Data)
		if err != nil {
			return nodeResult{}, err
		}
		dataArg = b
	}
	var aliasesArg any
	if in.Aliases != nil {
		aliasesArg = in.Aliases
	}

	const sql = `
UPDATE nodes
SET name       = COALESCE($2, name),
    aliases    = CASE WHEN $3::text[] IS NULL THEN aliases
                      ELSE ARRAY(SELECT DISTINCT unnest(aliases || $3::text[])) END,
    data       = data || $4::jsonb,
    updated_at = now()
WHERE id = $1 AND type = 'person'
RETURNING id, type, name, aliases, data`
	node, err := scanNode(tx.QueryRow(ctx, sql, id, nameArg, aliasesArg, dataArg))
	if errors.Is(err, pgx.ErrNoRows) {
		return nodeResult{}, ErrNodeNotFound
	}
	if err != nil {
		return nodeResult{}, fmt.Errorf("update person: %w", err)
	}

	if err := LogActivity(ctx, tx, "person_updated", "node", node.ID, ActorUser,
		"Aggiornata persona "+node.Name, in.Data); err != nil {
		return nodeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nodeResult{}, fmt.Errorf("update person commit: %w", err)
	}
	return node, nil
}

// LinkInput descrive un legame da creare/aggiornare.
type LinkInput struct {
	FromID int64    `json:"from_id"`
	ToID   int64    `json:"to_id"`
	Type   string   `json:"type"`
	Weight *float64 `json:"weight"`
	Note   string   `json:"note"`
}

// CreateLink crea o aggiorna (upsert) un legame tra due nodi e logga l'attività.
func CreateLink(ctx context.Context, pool *pgxpool.Pool, in LinkInput) (edgeResult, error) {
	if in.Type == "" {
		return edgeResult{}, errInvalid("il campo 'type' è obbligatorio")
	}
	if in.FromID == in.ToID {
		return edgeResult{}, errInvalid("un legame deve collegare due nodi diversi")
	}

	data := []byte("{}")
	if in.Note != "" {
		b, err := jsonObject(map[string]any{"note": in.Note})
		if err != nil {
			return edgeResult{}, err
		}
		data = b
	}
	var weightArg any
	if in.Weight != nil {
		weightArg = *in.Weight
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return edgeResult{}, fmt.Errorf("create link begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const sql = `
INSERT INTO edges (from_id, to_id, type, weight, data)
VALUES ($1, $2, $3, COALESCE($4, 1.0), $5)
ON CONFLICT (from_id, to_id, type) DO UPDATE
SET weight     = COALESCE($4, edges.weight),
    data       = edges.data || $5::jsonb,
    updated_at = now()
RETURNING id, from_id, to_id, type, weight, last_seen, data`
	edge, err := scanEdge(tx.QueryRow(ctx, sql, in.FromID, in.ToID, in.Type, weightArg, data))
	if err != nil {
		// Violazione di foreign key (23503) = from_id o to_id inesistente.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return edgeResult{}, errInvalid("from_id o to_id inesistente")
		}
		return edgeResult{}, fmt.Errorf("create link: %w", err)
	}

	if err := LogActivity(ctx, tx, "link_created", "edge", edge.ID, ActorUser,
		fmt.Sprintf("Creato legame %q", edge.Type), nil); err != nil {
		return edgeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return edgeResult{}, fmt.Errorf("create link commit: %w", err)
	}
	return edge, nil
}

// DeleteLink rimuove un legame per id e logga l'attività. Restituisce
// ErrNodeNotFound se l'arco non esiste.
func DeleteLink(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("delete link begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `DELETE FROM edges WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNodeNotFound
	}

	if err := LogActivity(ctx, tx, "link_deleted", "edge", id, ActorUser,
		"Rimosso un legame", nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeletePerson rimuove una persona per id e logga l'attività. Gli archi e le
// partecipazioni a eventi collegati cadono per ON DELETE CASCADE. Restituisce
// ErrNodeNotFound se l'id non esiste o non è una persona.
func DeletePerson(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("delete person begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var name string
	err = tx.QueryRow(ctx,
		`DELETE FROM nodes WHERE id = $1 AND type = 'person' RETURNING name`, id).
		Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNodeNotFound
	}
	if err != nil {
		return fmt.Errorf("delete person: %w", err)
	}

	if err := LogActivity(ctx, tx, "person_deleted", "node", id, ActorUser,
		"Eliminata persona "+name, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// coalesceData restituisce una mappa non-nil per le colonne data.
func coalesceData(d map[string]any) map[string]any {
	if d == nil {
		return map[string]any{}
	}
	return d
}

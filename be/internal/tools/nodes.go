package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// nodeResult è la rappresentazione di un nodo restituita ai tool.
type nodeResult struct {
	ID      int64          `json:"id"`
	Type    string         `json:"type"`
	Name    string         `json:"name"`
	Aliases []string       `json:"aliases"`
	Data    map[string]any `json:"data"`
}

// findNode cerca un nodo per nome o alias (case-insensitive + fuzzy trigram).
// Restituisce i candidati ordinati per rilevanza, così l'agente può disambiguare.
// Query 1 di predizioni.sql.
func findNode(ctx context.Context, pool *pgxpool.Pool, args map[string]any) (any, error) {
	query, err := argString(args, "query")
	if err != nil {
		return nil, err
	}
	typeFilter, err := argStringOpt(args, "type")
	if err != nil {
		return nil, err
	}

	const sql = `
SELECT id, type, name, aliases, data
FROM nodes
WHERE ($2::text IS NULL OR type = $2)
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
LIMIT 10`

	var typeArg any
	if typeFilter != "" {
		typeArg = typeFilter
	}

	rows, err := pool.Query(ctx, sql, query, typeArg)
	if err != nil {
		return nil, fmt.Errorf("find_node: %w", err)
	}
	defer rows.Close()

	results, err := scanNodes(rows)
	if err != nil {
		return nil, fmt.Errorf("find_node scan: %w", err)
	}
	return map[string]any{"candidates": results}, nil
}

// upsertPerson crea o aggiorna una persona. Vedi upsertNode.
func upsertPerson(ctx context.Context, pool *pgxpool.Pool, args map[string]any) (any, error) {
	return upsertNode(ctx, pool, "person", args)
}

// upsertPlace crea o aggiorna un luogo. Vedi upsertNode.
func upsertPlace(ctx context.Context, pool *pgxpool.Pool, args map[string]any) (any, error) {
	return upsertNode(ctx, pool, "place", args)
}

// upsertNode crea un nodo del tipo dato, oppure aggiorna quello con `id`:
// merge degli alias senza duplicati e merge del campo data. Query 2 di predizioni.sql.
func upsertNode(ctx context.Context, pool *pgxpool.Pool, nodeType string, args map[string]any) (any, error) {
	aliases, err := argStringSlice(args, "aliases")
	if err != nil {
		return nil, err
	}
	if aliases == nil {
		aliases = []string{}
	}
	data, err := argJSONOpt(args, "data")
	if err != nil {
		return nil, err
	}

	id, hasID, err := argInt64Opt(args, "id")
	if err != nil {
		return nil, err
	}

	var node nodeResult
	if hasID {
		// UPDATE: merge alias (dedup) + merge data.
		const sql = `
UPDATE nodes
SET aliases    = ARRAY(SELECT DISTINCT unnest(aliases || $2::text[])),
    data       = data || $3::jsonb,
    updated_at = now()
WHERE id = $1
RETURNING id, type, name, aliases, data`
		row := pool.QueryRow(ctx, sql, id, aliases, data)
		node, err = scanNode(row)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("upsert: nodo id=%d non trovato", id)
		}
		if err != nil {
			return nil, fmt.Errorf("upsert update: %w", err)
		}
		return node, nil
	}

	// INSERT: nuovo nodo.
	name, err := argString(args, "name")
	if err != nil {
		return nil, err
	}
	const sql = `
INSERT INTO nodes (type, name, aliases, data)
VALUES ($1, $2, $3, $4)
RETURNING id, type, name, aliases, data`
	row := pool.QueryRow(ctx, sql, nodeType, name, aliases, data)
	node, err = scanNode(row)
	if err != nil {
		return nil, fmt.Errorf("upsert insert: %w", err)
	}
	return node, nil
}

// linkNodes crea o aggiorna una relazione tra due nodi (upsert su from,to,type).
// La nota opzionale finisce in data.note. Query 3 di predizioni.sql.
func linkNodes(ctx context.Context, pool *pgxpool.Pool, args map[string]any) (any, error) {
	fromID, err := argInt64(args, "from_id")
	if err != nil {
		return nil, err
	}
	toID, err := argInt64(args, "to_id")
	if err != nil {
		return nil, err
	}
	relType, err := argString(args, "type")
	if err != nil {
		return nil, err
	}
	weight, hasWeight, err := argFloat64Opt(args, "weight")
	if err != nil {
		return nil, err
	}
	note, err := argStringOpt(args, "note")
	if err != nil {
		return nil, err
	}

	// La nota viene incapsulata in un JSONB {"note": "..."} per il merge in data.
	data := []byte("{}")
	if note != "" {
		nb, err := jsonObject(map[string]any{"note": note})
		if err != nil {
			return nil, err
		}
		data = nb
	}

	var weightArg any
	if hasWeight {
		weightArg = weight
	}

	const sql = `
INSERT INTO edges (from_id, to_id, type, weight, data)
VALUES ($1, $2, $3, COALESCE($4, 1.0), $5)
ON CONFLICT (from_id, to_id, type) DO UPDATE
SET weight     = COALESCE($4, edges.weight),
    data       = edges.data || $5::jsonb,
    updated_at = now()
RETURNING id, from_id, to_id, type, weight, last_seen, data`

	row := pool.QueryRow(ctx, sql, fromID, toID, relType, weightArg, data)
	edge, err := scanEdge(row)
	if err != nil {
		return nil, fmt.Errorf("link_nodes: %w", err)
	}
	return edge, nil
}

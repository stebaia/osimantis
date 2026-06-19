package tools

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// getNeighbors restituisce il vicinato di un nodo: relazioni uscenti ed entranti
// con il nodo collegato e i metadati dell'arco. Query 4 di predizioni.sql.
func getNeighbors(ctx context.Context, pool *pgxpool.Pool, args map[string]any) (any, error) {
	nodeID, err := argInt64(args, "node_id")
	if err != nil {
		return nil, err
	}

	const sql = `
SELECT e.id, e.type, e.weight, e.last_seen, e.data,
       CASE WHEN e.from_id = $1 THEN 'out' ELSE 'in' END AS direction,
       n.id, n.type, n.name, n.aliases
FROM edges e
JOIN nodes n ON n.id = CASE WHEN e.from_id = $1 THEN e.to_id ELSE e.from_id END
WHERE e.from_id = $1 OR e.to_id = $1
ORDER BY e.weight DESC, e.last_seen DESC NULLS LAST`

	rows, err := pool.Query(ctx, sql, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get_neighbors: %w", err)
	}
	defer rows.Close()

	type neighbor struct {
		EdgeID          int64    `json:"edge_id"`
		Relation        string   `json:"relation"`
		Weight          float64  `json:"weight"`
		LastSeen        *string  `json:"last_seen"`
		Direction       string   `json:"direction"`
		NeighborID      int64    `json:"neighbor_id"`
		NeighborType    string   `json:"neighbor_type"`
		NeighborName    string   `json:"neighbor_name"`
		NeighborAliases []string `json:"neighbor_aliases"`
	}

	out := []neighbor{}
	for rows.Next() {
		var nb neighbor
		var edgeData []byte // letto ma non esposto: i metadati dell'arco (es. note)
		if err := rows.Scan(&nb.EdgeID, &nb.Relation, &nb.Weight, &nb.LastSeen, &edgeData,
			&nb.Direction, &nb.NeighborID, &nb.NeighborType, &nb.NeighborName, &nb.NeighborAliases); err != nil {
			return nil, fmt.Errorf("get_neighbors scan: %w", err)
		}
		out = append(out, nb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get_neighbors rows: %w", err)
	}
	return map[string]any{"neighbors": out}, nil
}

// recentEvents restituisce gli eventi più recenti, opzionalmente filtrati per
// nodo coinvolto. Query 6 di predizioni.sql.
func recentEvents(ctx context.Context, pool *pgxpool.Pool, args map[string]any) (any, error) {
	nodeID, hasNode, err := argInt64Opt(args, "node_id")
	if err != nil {
		return nil, err
	}
	limit, hasLimit, err := argInt64Opt(args, "limit")
	if err != nil {
		return nil, err
	}
	if !hasLimit || limit <= 0 {
		limit = 10
	}

	const sql = `
SELECT ev.id, ev.raw_text, ev.summary, ev.occurred_at, ev.place_id, ev.data,
       COALESCE(
           array_agg(DISTINCT p.node_id) FILTER (WHERE p.node_id IS NOT NULL),
           '{}'
       ) AS participant_ids
FROM events ev
LEFT JOIN event_participants p ON p.event_id = ev.id
WHERE $1::bigint IS NULL
   OR ev.id IN (SELECT event_id FROM event_participants WHERE node_id = $1)
GROUP BY ev.id
ORDER BY ev.occurred_at DESC
LIMIT $2`

	var nodeArg any
	if hasNode {
		nodeArg = nodeID
	}

	rows, err := pool.Query(ctx, sql, nodeArg, limit)
	if err != nil {
		return nil, fmt.Errorf("recent_events: %w", err)
	}
	defer rows.Close()

	type event struct {
		ID             int64   `json:"id"`
		RawText        string  `json:"raw_text"`
		Summary        *string `json:"summary"`
		OccurredAt     string  `json:"occurred_at"`
		PlaceID        *int64  `json:"place_id"`
		ParticipantIDs []int64 `json:"participant_ids"`
	}

	out := []event{}
	for rows.Next() {
		var ev event
		var data []byte // metadati evento, non esposti qui
		if err := rows.Scan(&ev.ID, &ev.RawText, &ev.Summary, &ev.OccurredAt, &ev.PlaceID, &data, &ev.ParticipantIDs); err != nil {
			return nil, fmt.Errorf("recent_events scan: %w", err)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recent_events rows: %w", err)
	}
	return map[string]any{"events": out}, nil
}

// predictionFeatures restituisce segnali aggregati su una persona, come base
// fattuale per ipotesi/predizioni. Query 7 di predizioni.sql.
func predictionFeatures(ctx context.Context, pool *pgxpool.Pool, args map[string]any) (any, error) {
	nodeID, err := argInt64(args, "node_id")
	if err != nil {
		return nil, err
	}

	const sql = `
SELECT
    n.id, n.name, n.aliases,
    (SELECT count(*) FROM edges e WHERE e.from_id = n.id OR e.to_id = n.id),
    (SELECT avg(weight) FROM edges e WHERE e.from_id = n.id OR e.to_id = n.id),
    (SELECT max(last_seen) FROM edges e WHERE e.from_id = n.id OR e.to_id = n.id),
    (SELECT count(*) FROM event_participants ep
        JOIN events ev ON ev.id = ep.event_id
        WHERE ep.node_id = n.id AND ev.occurred_at > now() - interval '90 days'),
    (SELECT count(*) FROM edges e
        WHERE (e.from_id = n.id OR e.to_id = n.id)
          AND e.last_seen IS NOT NULL
          AND e.last_seen < now() - interval '180 days')
FROM nodes n
WHERE n.id = $1`

	var (
		id            int64
		name          string
		aliases       []string
		degree        int64
		avgWeight     *float64
		lastContact   *string
		eventsLast90d int64
		staleEdges    int64
	)
	err = pool.QueryRow(ctx, sql, nodeID).Scan(
		&id, &name, &aliases, &degree, &avgWeight, &lastContact, &eventsLast90d, &staleEdges)
	if err != nil {
		return nil, fmt.Errorf("prediction_features: %w", err)
	}

	return map[string]any{
		"id":              id,
		"name":            name,
		"aliases":         aliases,
		"degree":          degree,
		"avg_weight":      avgWeight,
		"last_contact":    lastContact,
		"events_last_90d": eventsLast90d,
		"stale_edges":     staleEdges,
	}, nil
}

// searchSemantic è il tool di ricerca semantica (Step 9). L'embedding della
// query richiede il client LLM, non ancora cablato qui: per ora restituisce un
// errore esplicito invece di fingere un risultato.
func searchSemantic(ctx context.Context, pool *pgxpool.Pool, args map[string]any) (any, error) {
	if _, err := argString(args, "query"); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("search_semantic non ancora disponibile: richiede gli embedding (Step 9)")
}

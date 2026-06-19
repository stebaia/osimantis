package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GraphNode è un nodo nel dump del grafo (per l'endpoint GET /graph).
type GraphNode struct {
	ID   int64          `json:"id"`
	Type string         `json:"type"`
	Name string         `json:"name"`
	Data map[string]any `json:"data"`
}

// GraphEdge è un arco nel dump del grafo.
type GraphEdge struct {
	ID     int64   `json:"id"`
	From   int64   `json:"from"`
	To     int64   `json:"to"`
	Type   string  `json:"type"`
	Weight float64 `json:"weight"`
}

// Graph è l'intero grafo, consumato dall'app Flutter.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphDump legge tutti i nodi e gli archi. Query 9 di predizioni.sql.
// È una funzione esportata (non un ToolFn) perché serve direttamente
// all'endpoint HTTP, non all'LLM.
func GraphDump(ctx context.Context, pool *pgxpool.Pool) (Graph, error) {
	g := Graph{Nodes: []GraphNode{}, Edges: []GraphEdge{}}

	nodeRows, err := pool.Query(ctx, `SELECT id, type, name, data FROM nodes ORDER BY id`)
	if err != nil {
		return Graph{}, fmt.Errorf("graph dump nodi: %w", err)
	}
	defer nodeRows.Close()
	for nodeRows.Next() {
		var n GraphNode
		var data []byte
		if err := nodeRows.Scan(&n.ID, &n.Type, &n.Name, &data); err != nil {
			return Graph{}, fmt.Errorf("graph dump scan nodo: %w", err)
		}
		if err := json.Unmarshal(data, &n.Data); err != nil {
			return Graph{}, fmt.Errorf("graph dump decode data: %w", err)
		}
		g.Nodes = append(g.Nodes, n)
	}
	if err := nodeRows.Err(); err != nil {
		return Graph{}, fmt.Errorf("graph dump nodi rows: %w", err)
	}

	edgeRows, err := pool.Query(ctx, `SELECT id, from_id, to_id, type, weight FROM edges ORDER BY id`)
	if err != nil {
		return Graph{}, fmt.Errorf("graph dump archi: %w", err)
	}
	defer edgeRows.Close()
	for edgeRows.Next() {
		var e GraphEdge
		if err := edgeRows.Scan(&e.ID, &e.From, &e.To, &e.Type, &e.Weight); err != nil {
			return Graph{}, fmt.Errorf("graph dump scan arco: %w", err)
		}
		g.Edges = append(g.Edges, e)
	}
	if err := edgeRows.Err(); err != nil {
		return Graph{}, fmt.Errorf("graph dump archi rows: %w", err)
	}

	return g, nil
}

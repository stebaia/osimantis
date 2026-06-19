package tools

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// scanNode legge una riga (id, type, name, aliases, data) in un nodeResult.
func scanNode(row pgx.Row) (nodeResult, error) {
	var n nodeResult
	var data []byte
	if err := row.Scan(&n.ID, &n.Type, &n.Name, &n.Aliases, &data); err != nil {
		return nodeResult{}, err
	}
	if err := json.Unmarshal(data, &n.Data); err != nil {
		return nodeResult{}, fmt.Errorf("decode node.data: %w", err)
	}
	return n, nil
}

// scanNodes legge tutte le righe restituite da una query su nodi.
func scanNodes(rows pgx.Rows) ([]nodeResult, error) {
	var out []nodeResult
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []nodeResult{}
	}
	return out, nil
}

// edgeResult rappresenta una relazione restituita ai tool.
type edgeResult struct {
	ID       int64          `json:"id"`
	FromID   int64          `json:"from_id"`
	ToID     int64          `json:"to_id"`
	Type     string         `json:"type"`
	Weight   float64        `json:"weight"`
	LastSeen *string        `json:"last_seen"`
	Data     map[string]any `json:"data"`
}

// scanEdge legge una riga (id, from, to, type, weight, last_seen, data).
func scanEdge(row pgx.Row) (edgeResult, error) {
	var e edgeResult
	var data []byte
	if err := row.Scan(&e.ID, &e.FromID, &e.ToID, &e.Type, &e.Weight, &e.LastSeen, &data); err != nil {
		return edgeResult{}, err
	}
	if err := json.Unmarshal(data, &e.Data); err != nil {
		return edgeResult{}, fmt.Errorf("decode edge.data: %w", err)
	}
	return e, nil
}

// jsonObject serializza una mappa in JSON ([]byte) per le colonne JSONB.
func jsonObject(m map[string]any) ([]byte, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("serializzazione JSON: %w", err)
	}
	return b, nil
}

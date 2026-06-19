// Package tools implementa i tool che l'LLM può invocare per leggere e scrivere
// nel knowledge graph. Ogni tool è una ToolFn registrata in un registry per nome;
// le definizioni JSON (function declarations) sono esposte all'LLM separatamente.
//
// Tutte le query sono parametrizzate (vedi predizioni.sql). add_event gira in una
// singola transazione.
package tools

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ToolFn è la firma di un tool: riceve gli argomenti deserializzati dall'LLM e
// restituisce un risultato serializzabile in JSON, oppure un errore.
type ToolFn func(ctx context.Context, pool *pgxpool.Pool, args map[string]any) (any, error)

// Registry mappa il nome del tool alla sua implementazione.
type Registry map[string]ToolFn

// NewRegistry costruisce il registry con tutti i tool del contratto LLM.
func NewRegistry() Registry {
	return Registry{
		"find_node":           findNode,
		"upsert_person":       upsertPerson,
		"upsert_place":        upsertPlace,
		"link_nodes":          linkNodes,
		"add_event":           addEvent,
		"get_neighbors":       getNeighbors,
		"recent_events":       recentEvents,
		"prediction_features": predictionFeatures,
		"search_semantic":     searchSemantic,
	}
}

// Call esegue il tool richiesto. Restituisce un errore se il tool non esiste.
func (r Registry) Call(ctx context.Context, pool *pgxpool.Pool, name string, args map[string]any) (any, error) {
	fn, ok := r[name]
	if !ok {
		return nil, fmt.Errorf("tool sconosciuto: %q", name)
	}
	return fn(ctx, pool, args)
}

package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// addEvent registra un evento in UNA transazione: o tutte le scritture o nessuna.
// Inserisce l'evento, i partecipanti, e aggiorna last_seen SOLO sugli archi che
// collegano tra loro i nodi coinvolti in QUESTO evento (partecipanti + luogo),
// non su tutti gli archi delle persone. Query 5 di predizioni.sql.
func addEvent(ctx context.Context, pool *pgxpool.Pool, args map[string]any) (any, error) {
	rawText, err := argString(args, "raw_text")
	if err != nil {
		return nil, err
	}
	participantIDs, err := argInt64Slice(args, "participant_ids")
	if err != nil {
		return nil, err
	}
	if len(participantIDs) == 0 {
		return nil, fmt.Errorf("add_event: serve almeno un partecipante in participant_ids")
	}
	summary, err := argStringOpt(args, "summary")
	if err != nil {
		return nil, err
	}
	placeID, hasPlace, err := argInt64Opt(args, "place_id")
	if err != nil {
		return nil, err
	}
	data, err := argJSONOpt(args, "data")
	if err != nil {
		return nil, err
	}

	var occurredAt any // nil → COALESCE($3, now()) nel SQL
	if s, err := argStringOpt(args, "occurred_at"); err != nil {
		return nil, err
	} else if s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("add_event: occurred_at deve essere in formato ISO 8601 (RFC3339): %w", err)
		}
		occurredAt = t
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("add_event begin: %w", err)
	}
	// Rollback è no-op dopo un Commit riuscito: garantisce il cleanup su ogni errore.
	defer func() { _ = tx.Rollback(ctx) }()

	// 5a. inserisci evento.
	var (
		eventID    int64
		occurredTS time.Time
	)
	var placeArg any
	if hasPlace {
		placeArg = placeID
	}
	const insEvent = `
INSERT INTO events (raw_text, summary, occurred_at, place_id, data)
VALUES ($1, $2, COALESCE($3, now()), $4, $5)
RETURNING id, occurred_at`
	var summaryArg any
	if summary != "" {
		summaryArg = summary
	}
	if err := tx.QueryRow(ctx, insEvent, rawText, summaryArg, occurredAt, placeArg, data).
		Scan(&eventID, &occurredTS); err != nil {
		return nil, fmt.Errorf("add_event insert evento: %w", err)
	}

	// 5b. inserisci i partecipanti (batch).
	const insParticipant = `
INSERT INTO event_participants (event_id, node_id)
VALUES ($1, $2)
ON CONFLICT (event_id, node_id) DO NOTHING`
	batch := &pgx.Batch{}
	for _, nodeID := range participantIDs {
		batch.Queue(insParticipant, eventID, nodeID)
	}
	br := tx.SendBatch(ctx, batch)
	for range participantIDs {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return nil, fmt.Errorf("add_event insert partecipante: %w", err)
		}
	}
	if err := br.Close(); err != nil {
		return nil, fmt.Errorf("add_event chiusura batch: %w", err)
	}

	// 5c. aggiorna last_seen sugli archi tra i nodi coinvolti (partecipanti + luogo).
	involved := append([]int64{}, participantIDs...)
	if hasPlace {
		involved = append(involved, placeID)
	}
	const updLastSeen = `
UPDATE edges
SET last_seen  = GREATEST(COALESCE(last_seen, $2::timestamptz), $2::timestamptz),
    updated_at = now()
WHERE from_id = ANY($1::bigint[])
  AND to_id   = ANY($1::bigint[])`
	tag, err := tx.Exec(ctx, updLastSeen, involved, occurredTS)
	if err != nil {
		return nil, fmt.Errorf("add_event update last_seen: %w", err)
	}

	// Changelog (stessa transazione: o l'evento + il log, o niente).
	summaryText := rawText
	if summary != "" {
		summaryText = summary
	}
	if err := LogActivity(ctx, tx, "event_added", "event", eventID, ActorAgent, summaryText, nil); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("add_event commit: %w", err)
	}

	return map[string]any{
		"event_id":      eventID,
		"occurred_at":   occurredTS.Format(time.RFC3339),
		"participants":  participantIDs,
		"edges_touched": tag.RowsAffected(),
	}, nil
}

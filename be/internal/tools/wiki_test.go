package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WikiList elenca solo le persone, in ordine alfabetico.
func TestWikiList(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	ctx := context.Background()

	mustUpsert(t, ctx, pool, upsertPerson, map[string]any{"name": "Erik Muratori", "aliases": []any{"Mura"}})
	mustUpsert(t, ctx, pool, upsertPerson, map[string]any{"name": "Anna Bianchi"})
	mustUpsert(t, ctx, pool, upsertPlace, map[string]any{"name": "Bar Basso"}) // place: NON deve comparire

	list, err := WikiList(ctx, pool)
	if err != nil {
		t.Fatalf("WikiList: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("attese 2 persone, ho %d: %+v", len(list), list)
	}
	if list[0].Name != "Anna Bianchi" || list[1].Name != "Erik Muratori" {
		t.Errorf("ordine alfabetico sbagliato: %+v", list)
	}
}

// WikiPage compone dati, legami (con nota) ed eventi recenti di una persona.
// È anche il test di regressione del bug timestamptz→string in occurred_at/last_seen.
func TestWikiPage(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	ctx := context.Background()

	person := mustUpsert(t, ctx, pool, upsertPerson, map[string]any{
		"name": "Erik Muratori", "aliases": []any{"Mura"},
		"data": map[string]any{"lavoro": "data scientist"},
	})
	place := mustUpsert(t, ctx, pool, upsertPlace, map[string]any{"name": "Bar Basso"})

	// Legame con nota.
	if _, err := linkNodes(ctx, pool, map[string]any{
		"from_id": person, "to_id": place, "type": "frequenta", "note": "dopo lavoro",
	}); err != nil {
		t.Fatalf("link_nodes: %v", err)
	}
	// Evento che coinvolge la persona.
	if _, err := addEvent(ctx, pool, map[string]any{
		"raw_text": "aperitivo al Bar Basso", "participant_ids": []any{person}, "place_id": place,
	}); err != nil {
		t.Fatalf("add_event: %v", err)
	}

	page, err := WikiPage(ctx, pool, person)
	if err != nil {
		t.Fatalf("WikiPage: %v", err)
	}

	if page.Name != "Erik Muratori" || page.Data["lavoro"] != "data scientist" {
		t.Errorf("dati persona errati: %+v", page)
	}
	if len(page.Neighbors) != 1 || page.Neighbors[0].Note != "dopo lavoro" {
		t.Errorf("legami/nota errati: %+v", page.Neighbors)
	}
	// occurred_at deve essere una stringa RFC3339 non vuota (regressione scan).
	if len(page.Events) != 1 || page.Events[0].OccurredAt == "" {
		t.Errorf("eventi errati: %+v", page.Events)
	}
}

// Un id inesistente (o non-persona) dà ErrNodeNotFound.
func TestWikiPageNotFound(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	ctx := context.Background()

	_, err := WikiPage(ctx, pool, 99999)
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("atteso ErrNodeNotFound, ho %v", err)
	}
}

// mustUpsert esegue un upsert e restituisce l'id del nodo creato.
func mustUpsert(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fn func(context.Context, *pgxpool.Pool, map[string]any) (any, error), args map[string]any) int64 {
	t.Helper()
	res, err := fn(ctx, pool, args)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	node, ok := res.(nodeResult)
	if !ok {
		t.Fatalf("tipo risultato inatteso: %T", res)
	}
	return node.ID
}

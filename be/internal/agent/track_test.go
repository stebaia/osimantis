package agent

import (
	"context"
	"testing"
)

// fakeExec ritorna un output fisso, ignorando name/args. Serve a guidare lo scan
// del trackingExecutor con risultati di forma controllata.
type fakeExec struct {
	out any
	err error
}

func (f *fakeExec) Call(context.Context, string, map[string]any) (any, error) {
	return f.out, f.err
}

func TestTrackingExecutorCollectsPersons(t *testing.T) {
	// upsert_person ritorna direttamente il nodo (forma struct → mappa via JSON).
	upsert := &fakeExec{out: map[string]any{
		"id": 7, "type": "person", "name": "Erik Muratori", "aliases": []string{"Mura"},
	}}
	// find_node ritorna {"candidates":[...]} con nodi annidati, anche un luogo.
	find := &fakeExec{out: map[string]any{"candidates": []any{
		map[string]any{"id": 9, "type": "person", "name": "Lucia"},
		map[string]any{"id": 3, "type": "place", "name": "Bar Basso"},
	}}}

	tr := NewTrackingExecutor(upsert)
	if _, err := tr.Call(context.Background(), "upsert_person", nil); err != nil {
		t.Fatalf("call upsert: %v", err)
	}
	// Sostituiamo l'inner per simulare un secondo tool nello stesso turno.
	tr.inner = find
	if _, err := tr.Call(context.Background(), "find_node", nil); err != nil {
		t.Fatalf("call find: %v", err)
	}

	got := tr.Touched()
	if len(got) != 2 {
		t.Fatalf("attesi 2 nodi persona, ottenuti %d: %+v", len(got), got)
	}
	// Ordine di prima comparsa: Erik (7) poi Lucia (9). Il luogo è escluso.
	if got[0].ID != 7 || got[0].Name != "Erik Muratori" {
		t.Errorf("primo nodo inatteso: %+v", got[0])
	}
	if got[1].ID != 9 || got[1].Name != "Lucia" {
		t.Errorf("secondo nodo inatteso: %+v", got[1])
	}
}

func TestTrackingExecutorDeprioritizesUser(t *testing.T) {
	// Un turno che tocca l'utente (is_user) e un amico: la scheda da aprire è
	// quella dell'amico, non quella dell'utente.
	tr := NewTrackingExecutor(&fakeExec{out: map[string]any{
		"id": 2, "type": "person", "name": "Stefano",
		"data": map[string]any{"is_user": true},
	}})
	if _, err := tr.Call(context.Background(), "get_user", nil); err != nil {
		t.Fatalf("call get_user: %v", err)
	}
	tr.inner = &fakeExec{out: map[string]any{
		"id": 1, "type": "person", "name": "Erik",
	}}
	if _, err := tr.Call(context.Background(), "upsert_person", nil); err != nil {
		t.Fatalf("call upsert: %v", err)
	}

	got := tr.Touched()
	if len(got) != 1 || got[0].Name != "Erik" {
		t.Fatalf("atteso solo Erik (utente escluso), ho %+v", got)
	}
}

func TestTrackingExecutorUserAloneIsKept(t *testing.T) {
	// "chi sono?": l'unico nodo toccato è l'utente → va mostrato comunque.
	tr := NewTrackingExecutor(&fakeExec{out: map[string]any{
		"user": map[string]any{
			"id": 2, "type": "person", "name": "Stefano",
			"data": map[string]any{"is_user": true},
		},
	}})
	if _, err := tr.Call(context.Background(), "get_user", nil); err != nil {
		t.Fatalf("call: %v", err)
	}
	got := tr.Touched()
	if len(got) != 1 || got[0].Name != "Stefano" {
		t.Fatalf("atteso Stefano, ho %+v", got)
	}
}

func TestTrackingExecutorDedupes(t *testing.T) {
	tr := NewTrackingExecutor(&fakeExec{out: map[string]any{
		"id": 7, "type": "person", "name": "Erik",
	}})
	for i := 0; i < 3; i++ {
		if _, err := tr.Call(context.Background(), "upsert_person", nil); err != nil {
			t.Fatalf("call: %v", err)
		}
	}
	if got := tr.Touched(); len(got) != 1 {
		t.Fatalf("atteso 1 nodo deduplicato, ottenuti %d", len(got))
	}
}

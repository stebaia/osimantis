package agent

import (
	"context"
	"strings"
	"testing"
)

// recordingExecutor cattura gli args che riceve e restituisce un risultato fisso.
type recordingExecutor struct {
	gotArgs map[string]any
	result  any
	err     error
}

func (e *recordingExecutor) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	e.gotArgs = args
	return e.result, e.err
}

// Il risultato di find_node con un nome reale deve uscire pseudonimizzato, e lo
// pseudonimo deve essere stabile (derivato dall'id).
func TestPseudonymizerMasksResult(t *testing.T) {
	inner := &recordingExecutor{result: map[string]any{
		"candidates": []any{
			map[string]any{"id": int64(7), "type": "person", "name": "Erik Muratori", "aliases": []string{"Mura"}},
		},
	}}
	exec := NewPseudonymizingExecutor(inner)

	out, err := exec.Call(context.Background(), "find_node", map[string]any{"query": "Mura"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	cands := out.(map[string]any)["candidates"].([]any)
	got := cands[0].(map[string]any)["name"].(string)
	if got == "Erik Muratori" {
		t.Fatal("il nome reale è uscito in chiaro verso l'LLM")
	}
	if !strings.HasPrefix(got, "Persona_") {
		t.Errorf("pseudonimo inatteso: %q", got)
	}
}

// Round-trip: dopo aver imparato un nodo, se il modello rimette lo pseudonimo in
// un campo testuale degli args, il tool deve ricevere il nome reale.
func TestPseudonymizerResolvesArgs(t *testing.T) {
	inner := &recordingExecutor{result: map[string]any{
		"id": int64(7), "type": "person", "name": "Erik Muratori",
	}}
	exec := NewPseudonymizingExecutor(inner)

	// 1° giro: impara il nodo e produce lo pseudonimo.
	out, err := exec.Call(context.Background(), "find_node", map[string]any{"query": "Erik"})
	if err != nil {
		t.Fatalf("Call 1: %v", err)
	}
	pseudo := out.(map[string]any)["name"].(string)
	if !strings.HasPrefix(pseudo, "Persona_") {
		t.Fatalf("atteso pseudonimo, ho %q", pseudo)
	}

	// 2° giro: il modello usa lo pseudonimo in un campo libero (raw_text).
	if _, err := exec.Call(context.Background(), "add_event", map[string]any{
		"raw_text": "aperitivo con " + pseudo,
	}); err != nil {
		t.Fatalf("Call 2: %v", err)
	}
	gotRaw := inner.gotArgs["raw_text"].(string)
	if gotRaw != "aperitivo con Erik Muratori" {
		t.Errorf("args non risolti: %q", gotRaw)
	}
}

// Lo pseudonimo deve dipendere solo da (type,id): due mappe diverse generano lo
// stesso pseudonimo per lo stesso nodo.
func TestPseudonymStableAcrossInstances(t *testing.T) {
	a := newPseudonymizer()
	b := newPseudonymizer()
	if a.pseudoFor(42, "person", "Tizio") != b.pseudoFor(42, "person", "Tizio") {
		t.Error("pseudonimo non stabile tra istanze")
	}
	if a.pseudoFor(42, "person", "X") == a.pseudoFor(43, "person", "X") {
		t.Error("id diversi devono dare pseudonimi diversi")
	}
}

// Un errore del tool che contiene un nome reale appreso va mascherato.
func TestPseudonymizerMasksError(t *testing.T) {
	p := newPseudonymizer()
	pseudo := p.pseudoFor(7, "person", "Erik Muratori")

	e := &pseudoExecutor{inner: &recordingExecutor{}, p: p}
	masked := e.p.maskError(errString("nodo Erik Muratori non trovato"))
	if strings.Contains(masked.Error(), "Erik Muratori") {
		t.Errorf("nome reale non mascherato nell'errore: %q", masked)
	}
	if !strings.Contains(masked.Error(), pseudo) {
		t.Errorf("pseudonimo assente nell'errore: %q", masked)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

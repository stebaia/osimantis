package agent

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"relazioni-server/internal/llm"
)

// mockLLM restituisce, a ogni Chat, il Turn successivo della sequenza
// preconfigurata. Registra gli storici ricevuti per le asserzioni.
type mockLLM struct {
	responses []llm.Turn
	calls     int
	histories [][]llm.Turn
}

func (m *mockLLM) Chat(ctx context.Context, history []llm.Turn, toolDefs []map[string]any) (llm.Turn, error) {
	m.histories = append(m.histories, append([]llm.Turn(nil), history...))
	if m.calls >= len(m.responses) {
		return llm.Turn{}, errors.New("mockLLM: nessuna risposta residua")
	}
	r := m.responses[m.calls]
	m.calls++
	return r, nil
}

// mockExecutor registra le chiamate ed esegue una funzione configurabile.
type mockExecutor struct {
	called []string
	fn     func(name string, args map[string]any) (any, error)
}

func (m *mockExecutor) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	m.called = append(m.called, name)
	if m.fn != nil {
		return m.fn(name, args)
	}
	return map[string]any{"ok": true}, nil
}

// Il loop: prima il modello chiede un tool, poi (visto il risultato) risponde
// testualmente. Verifichiamo, in modo deterministico con synctest, che il tool
// sia eseguito UNA volta e che la risposta finale sia quella attesa.
func TestRunAgentToolThenText(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := &mockLLM{responses: []llm.Turn{
			{Role: llm.RoleModel, ToolCalls: []llm.ToolCall{{Name: "find_node", Args: map[string]any{"query": "Mura"}}}},
			{Role: llm.RoleModel, Text: "Ho trovato Erik Muratori (Mura)."},
		}}
		exec := &mockExecutor{fn: func(name string, args map[string]any) (any, error) {
			return map[string]any{"candidates": []any{map[string]any{"id": 1, "name": "Erik Muratori"}}}, nil
		}}

		reply, err := RunAgent(context.Background(), client, exec, nil, "chi è Mura?", nil)
		if err != nil {
			t.Fatalf("RunAgent: %v", err)
		}
		if reply != "Ho trovato Erik Muratori (Mura)." {
			t.Errorf("reply = %q", reply)
		}
		if len(exec.called) != 1 || exec.called[0] != "find_node" {
			t.Errorf("tool eseguiti = %v, atteso [find_node] una volta", exec.called)
		}
		if client.calls != 2 {
			t.Errorf("chiamate LLM = %d, attese 2", client.calls)
		}
		// Allo storico della 2ª chiamata devono comparire: system, user, model(toolcall), tool(result).
		second := client.histories[1]
		if len(second) != 4 {
			t.Fatalf("storico 2ª chiamata len = %d, atteso 4: %+v", len(second), second)
		}
		if second[3].Role != llm.RoleTool || len(second[3].ToolResults) != 1 {
			t.Errorf("ultimo turn non è il risultato del tool: %+v", second[3])
		}
	})
}

// Lo storico (prior) passato a RunAgent deve finire nel contesto inviato
// all'LLM, tra il system prompt e il messaggio corrente.
func TestRunAgentIncludesHistory(t *testing.T) {
	client := &mockLLM{responses: []llm.Turn{
		{Role: llm.RoleModel, Text: "ok"},
	}}
	prior := []llm.Turn{
		{Role: llm.RoleUser, Text: "io sono Stefano"},
		{Role: llm.RoleModel, Text: "Ciao Stefano"},
	}

	_, err := RunAgent(context.Background(), client, &mockExecutor{}, nil, "chi sono?", prior)
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	sent := client.histories[0]
	// system, prior(2), messaggio corrente = 4 turni.
	if len(sent) != 4 {
		t.Fatalf("storico inviato len = %d, atteso 4: %+v", len(sent), sent)
	}
	if sent[1].Text != "io sono Stefano" || sent[2].Text != "Ciao Stefano" {
		t.Errorf("prior non incluso correttamente: %+v", sent)
	}
	if sent[3].Text != "chi sono?" {
		t.Errorf("messaggio corrente in posizione errata: %+v", sent[3])
	}
}

// Risposta diretta senza tool.
func TestRunAgentTextOnly(t *testing.T) {
	client := &mockLLM{responses: []llm.Turn{
		{Role: llm.RoleModel, Text: "Ciao!"},
	}}
	exec := &mockExecutor{}

	reply, err := RunAgent(context.Background(), client, exec, nil, "ciao", nil)
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	if reply != "Ciao!" {
		t.Errorf("reply = %q", reply)
	}
	if len(exec.called) != 0 {
		t.Errorf("nessun tool atteso, eseguiti %v", exec.called)
	}
}

// Un tool che fallisce non deve interrompere il loop: l'errore va all'LLM come
// risultato e il modello produce comunque una risposta finale.
func TestRunAgentToolErrorContinues(t *testing.T) {
	client := &mockLLM{responses: []llm.Turn{
		{Role: llm.RoleModel, ToolCalls: []llm.ToolCall{{Name: "find_node", Args: map[string]any{"query": "x"}}}},
		{Role: llm.RoleModel, Text: "Non ho trovato nulla."},
	}}
	exec := &mockExecutor{fn: func(name string, args map[string]any) (any, error) {
		return nil, errors.New("boom")
	}}

	reply, err := RunAgent(context.Background(), client, exec, nil, "cerca x", nil)
	if err != nil {
		t.Fatalf("RunAgent non deve fallire per errore tool: %v", err)
	}
	if reply != "Non ho trovato nulla." {
		t.Errorf("reply = %q", reply)
	}
	// L'errore del tool deve essere stato re-iniettato nello storico.
	last := client.histories[len(client.histories)-1]
	tr := last[len(last)-1].ToolResults[0]
	content, _ := tr.Content.(map[string]any)
	if content["error"] != "boom" {
		t.Errorf("errore del tool non propagato all'LLM: %+v", tr.Content)
	}
}

// SAFETY NET — promessa vuota: il modello conferma di aver salvato ("Ho segnato!")
// senza aver eseguito alcuna scrittura. Il loop deve reiniettare il richiamo e
// dare al modello un altro giro, in cui esegue davvero il tool di scrittura.
func TestRunAgentNudgesOnEmptySavePromise(t *testing.T) {
	client := &mockLLM{responses: []llm.Turn{
		// 1° giro: promette senza scrivere.
		{Role: llm.RoleModel, Text: "Ho segnato! Me lo ricordo."},
		// 2° giro (dopo il nudge): esegue la scrittura.
		{Role: llm.RoleModel, ToolCalls: []llm.ToolCall{{Name: "upsert_person", Args: map[string]any{"name": "Michela"}}}},
		// 3° giro: risposta finale.
		{Role: llm.RoleModel, Text: "Fatto davvero, Michela è salvata."},
	}}
	exec := &mockExecutor{}

	reply, err := RunAgent(context.Background(), client, exec, nil, "segna michela", nil)
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	if reply != "Fatto davvero, Michela è salvata." {
		t.Errorf("reply = %q", reply)
	}
	if len(exec.called) != 1 || exec.called[0] != "upsert_person" {
		t.Errorf("tool eseguiti = %v, atteso [upsert_person]", exec.called)
	}
	// Il nudge deve essere comparso nello storico come messaggio user.
	last := client.histories[len(client.histories)-1]
	foundNudge := false
	for _, turn := range last {
		if turn.Role == llm.RoleUser && turn.Text == saveNudge {
			foundNudge = true
		}
	}
	if !foundNudge {
		t.Errorf("il nudge non è stato reiniettato nello storico: %+v", last)
	}
}

// SAFETY NET — nessun falso richiamo se il modello HA scritto: conferma + tool di
// scrittura eseguito → la risposta è accettata subito, niente nudge.
func TestRunAgentNoNudgeWhenWroteAndPromised(t *testing.T) {
	client := &mockLLM{responses: []llm.Turn{
		{Role: llm.RoleModel, ToolCalls: []llm.ToolCall{{Name: "upsert_person", Args: map[string]any{"name": "Michela"}}}},
		{Role: llm.RoleModel, Text: "Ho segnato Michela!"},
	}}
	exec := &mockExecutor{}

	reply, err := RunAgent(context.Background(), client, exec, nil, "segna michela", nil)
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	if reply != "Ho segnato Michela!" {
		t.Errorf("reply = %q", reply)
	}
	if client.calls != 2 {
		t.Errorf("chiamate LLM = %d, attese 2 (nessun nudge)", client.calls)
	}
}

// SAFETY NET — nessun richiamo su una risposta di sola lettura che non promette
// salvataggi: una domanda ("chi è Mura?") non deve mai innescare il nudge.
func TestRunAgentNoNudgeOnReadOnlyReply(t *testing.T) {
	client := &mockLLM{responses: []llm.Turn{
		{Role: llm.RoleModel, Text: "Mura è Erik Muratori, un tuo amico."},
	}}
	exec := &mockExecutor{}

	reply, err := RunAgent(context.Background(), client, exec, nil, "chi è Mura?", nil)
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	if reply != "Mura è Erik Muratori, un tuo amico." {
		t.Errorf("reply = %q", reply)
	}
	if client.calls != 1 {
		t.Errorf("chiamate LLM = %d, attesa 1 (nessun nudge)", client.calls)
	}
}

// SAFETY NET — evento mancante: il modello crea le persone e il legame (quindi
// HA scritto), dice di aver registrato l'evento, ma non chiama add_event. La rete
// deve scattare comunque (regola specifica sull'evento) e dare un altro giro in
// cui esegue add_event.
func TestRunAgentNudgesOnMissingEvent(t *testing.T) {
	client := &mockLLM{responses: []llm.Turn{
		// 1° giro: scrive le persone + il legame.
		{Role: llm.RoleModel, ToolCalls: []llm.ToolCall{
			{Name: "upsert_person", Args: map[string]any{"name": "Federica"}},
			{Name: "upsert_person", Args: map[string]any{"name": "Michela"}},
			{Name: "link_nodes", Args: map[string]any{"from_id": float64(9), "to_id": float64(3), "type": "partner"}},
		}},
		// 2° giro: promette l'evento ma non lo registra → deve scattare il nudge.
		{Role: llm.RoleModel, Text: "Ho registrato l'evento al Rift e le persone."},
		// 3° giro (dopo il nudge): esegue add_event.
		{Role: llm.RoleModel, ToolCalls: []llm.ToolCall{
			{Name: "add_event", Args: map[string]any{"raw_text": "al rift c'erano...", "participant_ids": []any{float64(9), float64(10)}}},
		}},
		// 4° giro: risposta finale.
		{Role: llm.RoleModel, Text: "Fatto, evento al Rift salvato."},
	}}
	exec := &mockExecutor{}

	reply, err := RunAgent(context.Background(), client, exec, nil, "ieri al rift c'erano...", nil)
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	if reply != "Fatto, evento al Rift salvato." {
		t.Errorf("reply = %q", reply)
	}
	// add_event deve essere stato eseguito grazie al nudge.
	sawEvent := false
	for _, c := range exec.called {
		if c == "add_event" {
			sawEvent = true
		}
	}
	if !sawEvent {
		t.Errorf("add_event non eseguito: tool chiamati = %v", exec.called)
	}
	last := client.histories[len(client.histories)-1]
	foundNudge := false
	for _, turn := range last {
		if turn.Role == llm.RoleUser && turn.Text == eventNudge {
			foundNudge = true
		}
	}
	if !foundNudge {
		t.Errorf("eventNudge non reiniettato: %+v", last)
	}
}

// La rete scatta UNA sola volta: se anche dopo il nudge il modello continua a
// promettere senza eseguire, la risposta viene comunque accettata (no loop).
func TestRunAgentNudgesAtMostOnce(t *testing.T) {
	client := &mockLLM{responses: []llm.Turn{
		{Role: llm.RoleModel, Text: "Ho segnato! Me lo ricordo."},
		// Dopo il nudge promette di nuovo senza scrivere: deve essere accettata.
		{Role: llm.RoleModel, Text: "Ho segnato davvero stavolta."},
	}}
	exec := &mockExecutor{}

	reply, err := RunAgent(context.Background(), client, exec, nil, "segna x", nil)
	if err != nil {
		t.Fatalf("RunAgent: %v", err)
	}
	if reply != "Ho segnato davvero stavolta." {
		t.Errorf("reply = %q", reply)
	}
	if client.calls != 2 {
		t.Errorf("chiamate LLM = %d, attese 2 (un solo nudge)", client.calls)
	}
}

func TestPromisesEvent(t *testing.T) {
	yes := []string{"Ho registrato l'evento al Rift.", "Ho segnato l'incontro di ieri.", "Ho salvato la serata."}
	for _, s := range yes {
		if !promisesEvent(s) {
			t.Errorf("promisesEvent(%q) = false, atteso true", s)
		}
	}
	no := []string{"Ho segnato Michela.", "Mura è un tuo amico.", "Ho aggiornato il lavoro di Erik."}
	for _, s := range no {
		if promisesEvent(s) {
			t.Errorf("promisesEvent(%q) = true, atteso false", s)
		}
	}
}

func TestPromisesSave(t *testing.T) {
	saves := []string{"Ho segnato!", "Perfetto, me lo ricordo.", "Ok, ho aggiornato i dati.", "ora so che Mura è Erik"}
	for _, s := range saves {
		if !promisesSave(s) {
			t.Errorf("promisesSave(%q) = false, atteso true", s)
		}
	}
	reads := []string{"Mura è Erik Muratori.", "Chi intendi?", "Non ho trovato nulla."}
	for _, s := range reads {
		if promisesSave(s) {
			t.Errorf("promisesSave(%q) = true, atteso false", s)
		}
	}
}

// Se il modello continua a chiedere tool all'infinito, il cap interviene.
func TestRunAgentIterationCap(t *testing.T) {
	loop := make([]llm.Turn, maxIterations+2)
	for i := range loop {
		loop[i] = llm.Turn{Role: llm.RoleModel, ToolCalls: []llm.ToolCall{{Name: "find_node", Args: map[string]any{"query": "x"}}}}
	}
	client := &mockLLM{responses: loop}
	exec := &mockExecutor{}

	_, err := RunAgent(context.Background(), client, exec, nil, "loop", nil)
	if err == nil {
		t.Fatal("atteso errore per cap iterazioni")
	}
	if client.calls != maxIterations {
		t.Errorf("chiamate LLM = %d, attese %d (cap)", client.calls, maxIterations)
	}
}

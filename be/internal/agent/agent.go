// Package agent orchestra il ciclo conversazionale: invia il contesto all'LLM,
// esegue i tool che il modello richiede, re-inietta i risultati e ripete finché
// il modello produce una risposta testuale finale.
package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"relazioni-server/internal/llm"
)

// maxIterations limita i giri LLM↔tool per evitare loop infiniti. Il playbook
// suggeriva 6, ma i test reali mostrano che un singolo messaggio può toccare più
// entità (find_node + upsert per ciascuna + link/add_event), avvicinandosi al
// limite prima della risposta finale. 10 dà margine senza rinunciare alla guardia.
const maxIterations = 10

// ToolExecutor esegue un tool per nome. Lo introduciamo come interfaccia (invece
// di dipendere direttamente da tools.Registry + *pgxpool.Pool) così l'agente è
// testabile con un executor finto, senza database.
type ToolExecutor interface {
	Call(ctx context.Context, name string, args map[string]any) (any, error)
}

// RunAgent esegue il loop dell'agente per un singolo messaggio utente e
// restituisce la risposta testuale finale del modello.
//
// Logica:
//  1. storico = system prompt + messaggio utente;
//  2. chiama l'LLM con le definizioni dei tool;
//  3. finché il modello chiede tool: eseguili, logga (nome, durata, esito) e
//     re-inietta i risultati, poi richiama l'LLM;
//  4. quando il modello produce testo (nessun tool), restituiscilo.
//
// Cap di maxIterations. Se un tool fallisce, l'errore viene passato all'LLM come
// risultato (non si interrompe il loop).
// prior è lo storico della conversazione (memoria a breve termine), che il
// chiamante passa come turni user/model già pronti. Può essere nil: in quel caso
// l'agente vede solo system prompt + messaggio corrente.
func RunAgent(ctx context.Context, client llm.LLM, exec ToolExecutor, toolDefs []map[string]any, userText string, prior []llm.Turn) (string, error) {
	history := make([]llm.Turn, 0, len(prior)+2)
	history = append(history, llm.Turn{Role: llm.RoleSystem, Text: SystemPrompt})
	history = append(history, prior...)
	history = append(history, llm.Turn{Role: llm.RoleUser, Text: userText})

	emptyResponses := 0
	// called tiene traccia di quali tool sono stati eseguiti nel turno: la safety
	// net confronta ciò che la risposta PROMETTE con ciò che è stato fatto davvero.
	called := map[string]bool{}
	// nudged: la safety net può scattare UNA sola volta per turno (qualunque sia il
	// motivo), così non si rischiano richiami a catena.
	nudged := false
	for iter := 0; iter < maxIterations; iter++ {
		turn, err := client.Chat(ctx, history, toolDefs)
		if err != nil {
			// Gemini a volte restituisce un candidato vuoto (né testo né tool):
			// è transitorio. Ritentiamo qualche volta lo stesso contesto prima di
			// arrenderci con un messaggio gentile, invece di propagare un 500.
			var bad *llm.BadResponseError
			if errors.As(err, &bad) && bad.StatusCode == 0 {
				emptyResponses++
				if emptyResponses <= 2 {
					continue
				}
				return "Non sono riuscito a elaborare la risposta. Puoi riprovare o riformulare?", nil
			}
			return "", fmt.Errorf("chiamata LLM (iterazione %d): %w", iter, err)
		}

		// Nessun tool richiesto: è la risposta finale.
		if len(turn.ToolCalls) == 0 {
			if turn.Text == "" {
				return "", fmt.Errorf("il modello ha restituito una risposta vuota")
			}
			// SAFETY NET: il modello a volte CONFERMA a parole ciò che non ha fatto
			// con i tool, e l'utente crede che il dato sia salvato. Confrontiamo la
			// promessa della risposta con i tool effettivamente eseguiti e, se manca
			// l'azione, reiniettiamo UNA volta un richiamo perché la esegua davvero.
			// Due casi distinti:
			//   - promessa di salvataggio MA nessuna scrittura affatto (promessa vuota);
			//   - promessa di aver registrato un EVENTO/incontro MA add_event non
			//     chiamato (caso tipico: crea le persone ma tronca prima dell'evento).
			// Un solo nudge per turno (nudged) per non rischiare richiami a catena.
			if !nudged {
				if nudge := missingActionNudge(turn.Text, called); nudge != "" {
					nudged = true
					history = append(history, turn)
					history = append(history, llm.Turn{Role: llm.RoleUser, Text: nudge})
					continue
				}
			}
			return turn.Text, nil
		}

		// Il turn del modello (con le richieste di tool) va nello storico prima
		// dei risultati, così il contesto resta coerente.
		history = append(history, turn)

		results := make([]llm.ToolResult, 0, len(turn.ToolCalls))
		for _, call := range turn.ToolCalls {
			called[call.Name] = true
			results = append(results, executeTool(ctx, exec, call))
		}
		history = append(history, llm.Turn{Role: llm.RoleTool, ToolResults: results})
	}

	return "", fmt.Errorf("raggiunto il limite di %d iterazioni senza una risposta finale", maxIterations)
}

// writeTools sono i tool che modificano il grafo. I tool di sola lettura
// (find_node, get_user, get_neighbors, recent_events, prediction_features,
// search_semantic) non ne fanno parte.
var writeTools = map[string]bool{
	"upsert_person": true,
	"upsert_place":  true,
	"link_nodes":    true,
	"add_event":     true,
}

// I richiami reiniettati dalla safety net, uno per ciascun caso.
const (
	// saveNudge: la risposta conferma un salvataggio ma non è stato scritto nulla.
	saveNudge = `Hai confermato all'utente di aver salvato qualcosa, ma non hai ` +
		`eseguito nessun tool di scrittura in questo turno. Una conferma senza azione ` +
		`è una promessa vuota: l'utente crede che il dato sia salvato e non lo è. ` +
		`Esegui ORA i tool necessari (find_node, upsert_person/upsert_place, link_nodes, ` +
		`add_event) per tutte le persone, i luoghi, le relazioni e gli eventi del ` +
		`messaggio, poi rispondi.`
	// eventNudge: la risposta dice di aver registrato un evento ma add_event manca.
	eventNudge = `Hai detto all'utente di aver registrato l'evento/incontro, ma non hai ` +
		`chiamato add_event in questo turno: l'evento NON è stato salvato. Chiama ORA ` +
		`add_event con il testo grezzo, tutti i partecipanti (gli id delle persone ` +
		`coinvolte) e l'eventuale luogo, poi rispondi.`
)

// missingActionNudge confronta ciò che la risposta finale PROMETTE con i tool
// effettivamente eseguiti (called) e, se manca l'azione corrispondente,
// restituisce il richiamo da reiniettare. Stringa vuota = nessuna incoerenza.
//
// L'ordine conta: il caso "evento mancante" è più specifico e va controllato
// prima di quello generico "non ha scritto nulla".
func missingActionNudge(text string, called map[string]bool) string {
	if promisesEvent(text) && !called["add_event"] {
		return eventNudge
	}
	if promisesSave(text) && !hasWrite(called) {
		return saveNudge
	}
	return ""
}

// hasWrite dice se nel turno è stato eseguito almeno un tool di scrittura.
func hasWrite(called map[string]bool) bool {
	for name := range called {
		if writeTools[name] {
			return true
		}
	}
	return false
}

// promisesSave riconosce, in modo volutamente conservativo, le risposte che
// AFFERMANO un salvataggio già avvenuto (es. "ho segnato", "me lo ricordo"). Non
// deve scattare su domande o intenzioni future: un falso positivo costa solo un
// giro LLM in più, ma teniamo l'insieme stretto sui modi di dire effettivi
// dell'assistente (vedi gli esempi di stile nel system prompt).
func promisesSave(text string) bool {
	t := strings.ToLower(text)
	markers := []string{
		"ho segnato", "segnato!", "ho salvato", "ho registrato", "ho aggiornato",
		"ho aggiunto", "me lo ricordo", "ora so", "ora ricordo", "annotato",
	}
	for _, m := range markers {
		if strings.Contains(t, m) {
			return true
		}
	}
	return false
}

// promisesEvent riconosce le risposte che affermano di aver registrato un EVENTO
// o un incontro (es. "ho registrato l'incontro", "ho segnato la serata"). È più
// specifica di promisesSave: cerca un verbo di registrazione vicino a un termine
// che indica un fatto accaduto.
func promisesEvent(text string) bool {
	t := strings.ToLower(text)
	hasNoun := false
	for _, n := range []string{"event", "incontro", "serata", "uscita", "aperitivo", "cena", "ritrovo"} {
		if strings.Contains(t, n) {
			hasNoun = true
			break
		}
	}
	if !hasNoun {
		return false
	}
	for _, v := range []string{"ho registrato", "ho segnato", "ho salvato", "ho annotato", "registrato l", "segnato l"} {
		if strings.Contains(t, v) {
			return true
		}
	}
	return false
}

// executeTool esegue una singola tool call, la logga, e impacchetta l'esito (o
// l'errore) in un ToolResult da re-iniettare. Un errore del tool NON interrompe
// il loop: viene comunicato al modello come contenuto del risultato.
func executeTool(ctx context.Context, exec ToolExecutor, call llm.ToolCall) llm.ToolResult {
	start := time.Now()
	out, err := exec.Call(ctx, call.Name, call.Args)
	dur := time.Since(start)

	if err != nil {
		slog.Error("tool eseguito", "tool", call.Name, "durata", dur, "esito", "errore", "err", err)
		return llm.ToolResult{
			Name:    call.Name,
			Content: map[string]any{"error": err.Error()},
		}
	}

	slog.Info("tool eseguito", "tool", call.Name, "durata", dur, "esito", "ok")
	return llm.ToolResult{Name: call.Name, Content: out}
}

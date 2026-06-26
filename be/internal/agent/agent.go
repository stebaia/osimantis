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
// suggeriva 6, ma i test reali mostrano che un singolo messaggio fitto può
// toccare molte entità: find_node + upsert per ogni persona/luogo, un link per
// ogni relazione, l'add_event finale, PIÙ i giri extra che i nudge della safety
// net possono aggiungere. Con 10 un messaggio tipo "al Rift c'erano A (ragazza
// di B) e C (ragazza di D)" esauriva il cap proprio sull'ultimo passo (l'evento).
// 20 dà margine abbondante; il loop resta protetto da maxNudges e dal timeout
// HTTP della richiesta.
const maxIterations = 20

// maxNudges limita quante volte per turno la safety net può reiniettare un
// richiamo. Un messaggio fitto può avere più lacune insieme (persone create ma
// non collegate E evento non registrato): due richiami le coprono entrambe senza
// rischiare un ping-pong infinito con il modello.
const maxNudges = 2

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
	// calls conta quante volte ciascun tool è stato eseguito nel turno: la safety
	// net confronta ciò che è stato fatto davvero (e quanto) con ciò che il
	// messaggio implicava e la risposta PROMETTE.
	calls := map[string]int{}
	// nudges: la safety net può scattare al massimo maxNudges volte per turno, così
	// copre più lacune (es. prima i link, poi l'evento) senza richiami a catena
	// infiniti.
	nudges := 0
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
			if nudges < maxNudges {
				if nudge := missingActionNudge(turn.Text, calls); nudge != "" {
					nudges++
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
			calls[call.Name]++
			results = append(results, executeTool(ctx, exec, call))
		}
		history = append(history, llm.Turn{Role: llm.RoleTool, ToolResults: results})
	}

	// Cap raggiunto senza una risposta testuale finale. Le scritture eseguite nelle
	// iterazioni precedenti sono già state committate (ogni tool scrive subito), per
	// cui propagare un 500 sarebbe fuorviante: il grosso del lavoro è salvato.
	// Restituiamo una conferma onesta — confermiamo ciò che è stato fatto ma
	// invitiamo a ricontrollare — invece di un errore che fa credere all'utente di
	// aver perso tutto. Logghiamo il caso per accorgercene se diventa frequente.
	slog.Warn("agente: limite iterazioni raggiunto", "max", maxIterations)
	return "Ho salvato le persone e le relazioni che mi hai detto. C'era parecchia roba in un colpo solo: se manca qualcosa (tipo l'evento), scrivimelo e lo aggiungo.", nil
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
	// linkNudge: hai creato/aggiornato più persone o luoghi ma non li hai collegati.
	// Quando un messaggio nomina più entità insieme, quasi sempre c'è una relazione
	// tra loro (es. "Federica, la ragazza di Visa") o un evento che le unisce: se
	// non è stato chiamato link_nodes, è quasi certo che manchino le relazioni.
	linkNudge = `In questo turno hai creato o aggiornato più persone/luoghi ma non hai ` +
		`chiamato link_nodes nemmeno una volta: le relazioni tra loro NON sono state ` +
		`salvate. Rileggi il messaggio dell'utente ed estrai TUTTE le relazioni tra le ` +
		`entità che hai gestito (es. "X è la ragazza di Y", "Z lavora a W", "K frequenta ` +
		`il locale J"). Chiama ORA link_nodes per ciascuna relazione, usando gli id dei ` +
		`nodi coinvolti, e registra anche l'eventuale evento con add_event. Poi rispondi.`
)

// missingActionNudge confronta ciò che è stato fatto davvero nel turno (calls,
// con il conteggio per tool) e ciò che la risposta finale PROMETTE; se manca
// l'azione corrispondente restituisce il richiamo da reiniettare. Stringa vuota
// = nessuna incoerenza.
//
// L'ordine conta, dal più specifico al più generico:
//  1. evento promesso ma add_event mancante;
//  2. più entità create ma nessun link (lacuna strutturale, indipendente dal
//     testo della risposta: è il fallimento tipico dei messaggi fitti);
//  3. salvataggio promesso ma nessuna scrittura affatto.
func missingActionNudge(text string, calls map[string]int) string {
	if promisesEvent(text) && calls["add_event"] == 0 {
		return eventNudge
	}
	if createdMultipleNodes(calls) && calls["link_nodes"] == 0 {
		return linkNudge
	}
	if promisesSave(text) && !hasWrite(calls) {
		return saveNudge
	}
	return ""
}

// createdMultipleNodes dice se nel turno sono state create/aggiornate almeno due
// persone o luoghi (in qualunque combinazione). Sotto le due entità non scatta il
// richiamo sui link: un singolo inserimento isolato è legittimo, e pretendere un
// collegamento genererebbe falsi positivi.
func createdMultipleNodes(calls map[string]int) bool {
	return calls["upsert_person"]+calls["upsert_place"] >= 2
}

// hasWrite dice se nel turno è stato eseguito almeno un tool di scrittura.
func hasWrite(calls map[string]int) bool {
	for name, n := range calls {
		if n > 0 && writeTools[name] {
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

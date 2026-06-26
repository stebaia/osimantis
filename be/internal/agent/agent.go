// Package agent orchestra il ciclo conversazionale: invia il contesto all'LLM,
// esegue i tool che il modello richiede, re-inietta i risultati e ripete finché
// il modello produce una risposta testuale finale.
package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
			return turn.Text, nil
		}

		// Il turn del modello (con le richieste di tool) va nello storico prima
		// dei risultati, così il contesto resta coerente.
		history = append(history, turn)

		results := make([]llm.ToolResult, 0, len(turn.ToolCalls))
		for _, call := range turn.ToolCalls {
			results = append(results, executeTool(ctx, exec, call))
		}
		history = append(history, llm.Turn{Role: llm.RoleTool, ToolResults: results})
	}

	return "", fmt.Errorf("raggiunto il limite di %d iterazioni senza una risposta finale", maxIterations)
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

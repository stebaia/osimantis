// Command llmcheck è un test manuale del client LLM: manda un messaggio a Gemini
// e stampa la risposta. Serve a verificare la connettività e la chiave reale.
//
//	GEMINI_API_KEY=... go run ./cmd/llmcheck
//	GEMINI_API_KEY=... go run ./cmd/llmcheck "scrivi una frase sul mare"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"relazioni-server/internal/llm"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "GEMINI_API_KEY non impostata")
		os.Exit(1)
	}

	prompt := "Ciao! Rispondi con una sola frase per confermare che funzioni."
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}

	client := llm.NewGemini(apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	turn, err := client.Chat(ctx, []llm.Turn{{Role: llm.RoleUser, Text: prompt}}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "errore: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Prompt: %s\n\nRisposta Gemini:\n%s\n", prompt, turn.Text)
}

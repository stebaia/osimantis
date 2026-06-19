// Package llm definisce un'interfaccia provider-agnostica verso un LLM con
// supporto al function calling, e una sua implementazione per Gemini via REST.
//
// L'agente (internal/agent) dipende SOLO dall'interfaccia LLM e dai tipi di
// questo file: aggiungere un nuovo provider (Groq OpenAI-compatibile, Ollama
// locale) significa scrivere una nuova implementazione, senza toccare il resto.
package llm

import "context"

// Role è il ruolo di un messaggio nella conversazione.
type Role string

const (
	RoleUser  Role = "user"  // messaggio dell'utente
	RoleModel Role = "model" // risposta del modello
	RoleTool  Role = "tool"  // risultato di un tool re-iniettato nel contesto
)

// ToolCall è una richiesta del modello di invocare un tool, con i suoi argomenti.
type ToolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// ToolResult è l'esito dell'esecuzione di un tool, da re-iniettare nel contesto.
type ToolResult struct {
	Name    string `json:"name"`
	Content any    `json:"content"`
}

// Turn è un messaggio della conversazione, in forma neutra rispetto al provider.
//   - role=user/model: Text contiene il testo.
//   - role=model con tool: ToolCalls contiene le invocazioni richieste.
//   - role=tool: ToolResults contiene gli esiti dei tool richiesti prima.
type Turn struct {
	Role        Role         `json:"role"`
	Text        string       `json:"text,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}

// LLM è l'astrazione su cui si appoggia l'agente.
//
// Chat invia lo storico della conversazione e le definizioni dei tool, e
// restituisce il prossimo Turn del modello: o testo finale, o una o più
// ToolCalls da eseguire (che l'agente eseguirà e re-inietterà).
type LLM interface {
	Chat(ctx context.Context, history []Turn, toolDefs []map[string]any) (Turn, error)
}

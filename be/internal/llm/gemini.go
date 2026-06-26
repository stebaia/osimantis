package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultGeminiModel = "gemini-2.5-flash"
	defaultBaseURL     = "https://generativelanguage.googleapis.com/v1beta"
	maxRetries         = 4 // backoff: 1s, 2s, 4s, 8s
)

// Gemini è l'implementazione di LLM per l'API REST di Google Gemini.
type Gemini struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

// GeminiOption personalizza il client (modello, baseURL, http.Client).
type GeminiOption func(*Gemini)

// WithModel imposta il modello (default gemini-2.5-flash).
func WithModel(m string) GeminiOption { return func(g *Gemini) { g.model = m } }

// WithBaseURL imposta l'endpoint base (utile per i test).
func WithBaseURL(u string) GeminiOption { return func(g *Gemini) { g.baseURL = u } }

// WithHTTPClient inietta un http.Client custom.
func WithHTTPClient(c *http.Client) GeminiOption { return func(g *Gemini) { g.http = c } }

// NewGemini costruisce il client. La chiave arriva dalla config (GEMINI_API_KEY).
func NewGemini(apiKey string, opts ...GeminiOption) *Gemini {
	g := &Gemini{
		apiKey:  apiKey,
		model:   defaultGeminiModel,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Chat implementa LLM: mappa lo storico in una richiesta Gemini, gestisce il
// retry su rate limit, e mappa la risposta nel Turn del modello.
func (g *Gemini) Chat(ctx context.Context, history []Turn, toolDefs []map[string]any) (Turn, error) {
	reqBody := g.buildRequest(history, toolDefs)
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return Turn{}, fmt.Errorf("marshal richiesta gemini: %w", err)
	}

	resp, err := g.doWithRetry(ctx, payload)
	if err != nil {
		return Turn{}, err
	}

	return g.parseResponse(resp)
}

// buildRequest mappa i Turn neutri nel formato Gemini. Il Turn con role=system
// viene instradato nel canale dedicato systemInstruction, che Gemini tratta come
// istruzione forte e persistente: le regole (es. creazione proattiva) non si
// diluiscono dopo qualche giro di tool, come accadeva quando il prompt era un
// normale messaggio user nello storico.
func (g *Gemini) buildRequest(history []Turn, toolDefs []map[string]any) geminiRequest {
	req := geminiRequest{}
	if len(toolDefs) > 0 {
		req.Tools = []geminiTool{{FunctionDeclarations: toolDefs}}
	}

	for _, t := range history {
		switch t.Role {
		case RoleSystem:
			// systemInstruction è single-shot: se per qualche motivo arrivassero più
			// turni system, l'ultimo vince (gli altri sono ignorati). In pratica ce
			// n'è sempre uno solo, in testa.
			req.SystemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: t.Text}},
			}
		case RoleUser:
			req.Contents = append(req.Contents, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: t.Text}},
			})
		case RoleModel:
			parts := []geminiPart{}
			if t.Text != "" {
				parts = append(parts, geminiPart{Text: t.Text})
			}
			for _, tc := range t.ToolCalls {
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{Name: tc.Name, Args: tc.Args},
				})
			}
			req.Contents = append(req.Contents, geminiContent{Role: "model", Parts: parts})
		case RoleTool:
			// I risultati dei tool vanno come parts functionResponse con role "user".
			parts := make([]geminiPart, 0, len(t.ToolResults))
			for _, tr := range t.ToolResults {
				parts = append(parts, geminiPart{
					FunctionResponse: &geminiFunctionResponse{
						Name:     tr.Name,
						Response: wrapResponse(tr.Content),
					},
				})
			}
			req.Contents = append(req.Contents, geminiContent{Role: "user", Parts: parts})
		}
	}
	return req
}

// wrapResponse garantisce che il payload di functionResponse sia un oggetto JSON
// (Gemini richiede un object), anche quando il tool restituisce un valore scalare
// o un array.
func wrapResponse(content any) map[string]any {
	if m, ok := content.(map[string]any); ok {
		return m
	}
	return map[string]any{"result": content}
}

// doWithRetry esegue la POST con retry a backoff esponenziale sui 429.
func (g *Gemini) doWithRetry(ctx context.Context, payload []byte) (*geminiResponse, error) {
	url := fmt.Sprintf("%s/models/%s:generateContent", g.baseURL, g.model)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Backoff: 1s, 2s, 4s, 8s — sovrascritto da Retry-After se presente.
			wait := time.Duration(1<<(attempt-1)) * time.Second
			if rl, ok := lastErr.(*RateLimitedError); ok && rl.RetryAfter > 0 {
				wait = rl.RetryAfter
			}
			slog.Warn("gemini rate limited, retry", "attempt", attempt, "wait", wait)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		resp, err := g.doOnce(ctx, url, payload)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		// Riprova solo sui rate limit; gli altri errori sono definitivi.
		if _, ok := err.(*RateLimitedError); !ok {
			return nil, err
		}
	}
	return nil, fmt.Errorf("gemini: superati i %d retry: %w", maxRetries, lastErr)
}

// doOnce esegue una singola chiamata HTTP e classifica l'esito.
func (g *Gemini) doOnce(ctx context.Context, url string, payload []byte) (*geminiResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creazione richiesta: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", g.apiKey)

	httpResp, err := g.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chiamata gemini: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, &BadResponseError{StatusCode: httpResp.StatusCode, Reason: "lettura body fallita", Err: err}
	}

	switch {
	case httpResp.StatusCode == http.StatusTooManyRequests:
		return nil, &RateLimitedError{RetryAfter: parseRetryAfter(httpResp.Header.Get("Retry-After"))}
	case httpResp.StatusCode != http.StatusOK:
		return nil, &BadResponseError{StatusCode: httpResp.StatusCode, Reason: extractAPIError(body)}
	}

	var parsed geminiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, &BadResponseError{StatusCode: httpResp.StatusCode, Reason: "JSON non interpretabile", Err: err}
	}
	return &parsed, nil
}

// parseResponse mappa la risposta Gemini in un Turn neutro (testo o tool calls).
func (g *Gemini) parseResponse(resp *geminiResponse) (Turn, error) {
	if resp.PromptFeedback != nil && resp.PromptFeedback.BlockReason != "" {
		return Turn{}, &BadResponseError{Reason: "prompt bloccato: " + resp.PromptFeedback.BlockReason}
	}
	if len(resp.Candidates) == 0 {
		return Turn{}, &BadResponseError{Reason: "nessun candidato nella risposta"}
	}

	out := Turn{Role: RoleModel}
	for _, part := range resp.Candidates[0].Content.Parts {
		switch {
		case part.FunctionCall != nil:
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				Name: part.FunctionCall.Name,
				Args: part.FunctionCall.Args,
			})
		case part.Text != "":
			out.Text += part.Text
		}
	}

	if out.Text == "" && len(out.ToolCalls) == 0 {
		return Turn{}, &BadResponseError{Reason: "risposta senza testo né tool call"}
	}
	return out, nil
}

// parseRetryAfter interpreta l'header Retry-After (solo formato secondi).
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

// extractAPIError estrae il messaggio d'errore dall'envelope Gemini, con fallback
// al body grezzo.
func extractAPIError(body []byte) string {
	var env geminiErrorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
		return env.Error.Message
	}
	if len(body) > 300 {
		body = body[:300]
	}
	return string(body)
}

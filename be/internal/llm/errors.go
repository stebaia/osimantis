package llm

import (
	"fmt"
	"time"
)

// Errori tipizzati: l'agente può distinguere i casi con errors.As invece di
// confrontare stringhe.

// RateLimitedError indica un 429 dal provider, con l'eventuale Retry-After.
type RateLimitedError struct {
	RetryAfter time.Duration // 0 se l'header non era presente
	Err        error         // errore sottostante, se presente
}

func (e *RateLimitedError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limited (riprovare tra %s)", e.RetryAfter)
	}
	return "rate limited"
}

func (e *RateLimitedError) Unwrap() error { return e.Err }

// BadResponseError indica una risposta non interpretabile (status inatteso,
// JSON malformato, struttura mancante).
type BadResponseError struct {
	StatusCode int
	Reason     string
	Err        error
}

func (e *BadResponseError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("risposta LLM non valida (HTTP %d): %s", e.StatusCode, e.Reason)
	}
	return fmt.Sprintf("risposta LLM non valida: %s", e.Reason)
}

func (e *BadResponseError) Unwrap() error { return e.Err }

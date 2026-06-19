// Package config carica la configurazione dell'applicazione esclusivamente da
// variabili d'ambiente. Nessun valore sensibile è hardcoded nel sorgente.
package config

import (
	"fmt"
	"os"
)

// Config raccoglie tutti i parametri di runtime del server.
type Config struct {
	// DatabaseURL è la stringa di connessione Postgres (formato pgx/libpq).
	DatabaseURL string
	// GeminiAPIKey è la chiave per l'LLM. Obbligatoria già da ora così
	// l'avvio fallisce subito se l'ambiente non è configurato correttamente.
	GeminiAPIKey string
	// APIToken è il bearer token richiesto per autenticare le chiamate.
	APIToken string
	// Port è la porta TCP su cui il server HTTP è in ascolto.
	Port string
}

// Load legge la configurazione dall'ambiente. Restituisce un errore esplicito
// che elenca ogni variabile obbligatoria mancante, così il problema è chiaro
// al primo avvio invece di emergere in un secondo momento.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		APIToken:     os.Getenv("API_TOKEN"),
		Port:         os.Getenv("PORT"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.GeminiAPIKey == "" {
		missing = append(missing, "GEMINI_API_KEY")
	}
	if cfg.APIToken == "" {
		missing = append(missing, "API_TOKEN")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("variabili d'ambiente obbligatorie mancanti: %v", missing)
	}

	// PORT è opzionale: default a 8080.
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return cfg, nil
}

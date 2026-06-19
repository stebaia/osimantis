// Package config carica la configurazione dell'applicazione esclusivamente da
// variabili d'ambiente. Nessun valore sensibile è hardcoded nel sorgente.
package config

import (
	"fmt"
	"os"
	"strings"
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
		DatabaseURL:  databaseURL(),
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		APIToken:     os.Getenv("API_TOKEN"),
		Port:         os.Getenv("PORT"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL (oppure POSTGRES_USER/PASSWORD/DB)")
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

// databaseURL determina la stringa di connessione a Postgres.
//
// Preferisce DATABASE_URL se impostata. Altrimenti, se ci sono i componenti
// POSTGRES_USER/PASSWORD/DB, costruisce un DSN in formato key=value (libpq)
// invece di un URL postgres://. Questo è VOLUTO: nel formato URL una password con
// caratteri speciali (@ : / ? # } \ ...) andrebbe percent-encodata, e dimenticarlo
// rompe il parsing ("invalid port" & simili). Il formato key=value richiede solo
// di quotare/escapare gli spazi e gli apici, cosa che facciamo qui.
func databaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	user := os.Getenv("POSTGRES_USER")
	pass := os.Getenv("POSTGRES_PASSWORD")
	dbName := os.Getenv("POSTGRES_DB")
	if user == "" || dbName == "" {
		return ""
	}

	host := getenvDefault("POSTGRES_HOST", "db")
	port := getenvDefault("POSTGRES_PORT", "5432")

	// DSN key=value: ogni valore è quotato per gestire spazi e caratteri speciali
	// nella password senza alcun percent-encoding.
	parts := []string{
		"host=" + quoteDSN(host),
		"port=" + quoteDSN(port),
		"user=" + quoteDSN(user),
		"password=" + quoteDSN(pass),
		"dbname=" + quoteDSN(dbName),
		"sslmode=disable", // rete interna del progetto, non esposta
	}
	return strings.Join(parts, " ")
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// quoteDSN racchiude un valore tra apici singoli se contiene spazi o caratteri
// che il parser DSN di libpq/pgx tratterebbe come separatori, facendo l'escape
// di backslash e apici interni.
func quoteDSN(v string) string {
	if v == "" {
		return "''"
	}
	if !strings.ContainsAny(v, " '\\") {
		return v
	}
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(v) + "'"
}

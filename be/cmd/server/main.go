// Command server è il punto di ingresso del backend del knowledge graph delle
// relazioni. Carica la configurazione dall'ambiente, apre la connessione a
// Postgres, espone l'API HTTP e gestisce lo spegnimento controllato.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"relazioni-server/internal/config"
	"relazioni-server/internal/db"
	"relazioni-server/internal/llm"
	"relazioni-server/internal/logging"
	"relazioni-server/internal/server"
)

func main() {
	// Qualsiasi errore in fase di avvio è fatale: usciamo con codice 1 dopo
	// aver loggato la causa, così l'orchestratore (systemd, container, ecc.)
	// rileva il fallimento.
	if err := run(); err != nil {
		slog.Error("avvio fallito", "err", err)
		os.Exit(1)
	}
}

func run() error {
	logger := logging.Setup()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Context legato ai segnali di terminazione: alla ricezione di SIGINT o
	// SIGTERM viene cancellato, sbloccando l'attesa più in basso e innescando
	// lo shutdown controllato.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// La connessione iniziale ha un timeout dedicato: se il DB non risponde
	// non vogliamo restare appesi indefinitamente all'avvio.
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := db.Connect(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connesso al database")

	// Applichiamo lo schema (idempotente) all'avvio: il deploy non dipende più dal
	// bind-mount in /docker-entrypoint-initdb.d, che su alcune PaaS (Coolify) è
	// inaffidabile. Al primo avvio crea tabelle ed estensioni; ai successivi è no-op.
	if err := db.ApplySchema(connectCtx, pool); err != nil {
		return err
	}
	logger.Info("schema applicato")

	// Client LLM (Gemini) e server HTTP con le sue dipendenze.
	llmClient := llm.NewGemini(cfg.GeminiAPIKey)
	apiServer := server.New(pool, llmClient, cfg.APIToken)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Il server gira in una goroutine così il main può rimanere in attesa del
	// segnale di stop. Eventuali errori di ListenAndServe vengono inoltrati
	// sul canale.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server in ascolto", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Attendiamo o un errore del server o il segnale di terminazione.
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("ricevuto segnale di stop, avvio shutdown controllato")
	}

	// Diamo al server un tempo massimo per terminare le richieste in corso.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	logger.Info("shutdown completato")
	return nil
}

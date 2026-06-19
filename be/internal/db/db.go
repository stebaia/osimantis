// Package db gestisce la connessione al database Postgres tramite pgxpool.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect crea un pool di connessioni verso Postgres e verifica con Ping che il
// database risponda. Se il DB non è raggiungibile l'errore è esplicito e il
// chiamante (l'avvio del server) deve interrompersi.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("creazione pool pgx: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		// Il pool va chiuso: senza Close le connessioni già aperte resterebbero
		// appese in caso di Ping fallito.
		pool.Close()
		return nil, fmt.Errorf("ping al database fallito: %w", err)
	}

	return pool, nil
}

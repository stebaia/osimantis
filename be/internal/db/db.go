// Package db gestisce la connessione al database Postgres tramite pgxpool e
// l'applicazione idempotente dello schema all'avvio.
package db

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// schemaSQL è il DDL del grafo, embeddato nel binario. È la fonte di verità dello
// schema: applicarlo all'avvio rende il deploy indipendente dal meccanismo
// /docker-entrypoint-initdb.d (fragile con i bind-mount di alcune PaaS come
// Coolify) e idempotente grazie agli IF NOT EXISTS / CREATE EXTENSION IF NOT EXISTS.
//
//go:embed schema.sql
var schemaSQL string

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

// ApplySchema esegue il DDL embeddato. È idempotente (tutto IF NOT EXISTS), quindi
// può girare a ogni avvio: crea le tabelle/estensioni al primo deploy e non fa
// nulla di distruttivo nei successivi. Va chiamato dopo Connect, prima di servire.
func ApplySchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("applicazione schema: %w", err)
	}
	return nil
}

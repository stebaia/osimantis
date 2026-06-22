package tools

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// I test girano contro un Postgres reale (con pgvector + pg_trgm e lo schema
// applicato). Imposta TEST_DATABASE_URL per eseguirli, es. con lo stack Colima:
//   TEST_DATABASE_URL="postgres://relazioni:changeme@localhost:5432/relazioni?sslmode=disable" go test ./internal/tools/...
// Se la variabile non c'è, i test vengono saltati (non falliti).

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL non impostata: salto i test di integrazione")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connessione al DB di test: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("DB di test non raggiungibile: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// cleanGraph azzera le tabelle prima di ogni test per isolarli. activity_log non
// ha FK verso nodes, quindi il CASCADE non la tocca: va troncata a parte.
func cleanGraph(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE nodes, activity_log RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestUpsertPersonWithAlias(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	ctx := context.Background()

	res, err := upsertPerson(ctx, pool, map[string]any{
		"name":    "Erik Muratori",
		"aliases": []any{"Mura"},
		"data":    map[string]any{"lavoro": "ingegnere"},
	})
	if err != nil {
		t.Fatalf("upsert_person: %v", err)
	}
	node, ok := res.(nodeResult)
	if !ok {
		t.Fatalf("tipo risultato inatteso: %T", res)
	}
	if node.Name != "Erik Muratori" {
		t.Errorf("name = %q, atteso 'Erik Muratori'", node.Name)
	}
	if len(node.Aliases) != 1 || node.Aliases[0] != "Mura" {
		t.Errorf("aliases = %v, atteso [Mura]", node.Aliases)
	}
	if node.Data["lavoro"] != "ingegnere" {
		t.Errorf("data.lavoro = %v, atteso 'ingegnere'", node.Data["lavoro"])
	}

	// Update: aggiungo un alias e un campo; verifico merge senza duplicati.
	res2, err := upsertPerson(ctx, pool, map[string]any{
		"id":      float64(node.ID), // l'LLM manda numeri come float64
		"aliases": []any{"Mura", "Eriko"},
		"data":    map[string]any{"interessi": "vela"},
	})
	if err != nil {
		t.Fatalf("upsert_person update: %v", err)
	}
	node2 := res2.(nodeResult)
	if len(node2.Aliases) != 2 {
		t.Errorf("dopo merge aliases = %v, attesi 2 senza duplicati", node2.Aliases)
	}
	if node2.Data["lavoro"] != "ingegnere" || node2.Data["interessi"] != "vela" {
		t.Errorf("merge data fallito: %v", node2.Data)
	}
	// Update SENZA name: il name esistente va preservato.
	if node2.Name != "Erik Muratori" {
		t.Errorf("update senza name ha alterato il name: %q", node2.Name)
	}
}

// L'update di upsert_person deve poter CAMBIARE il name canonico (correzione),
// preservando alias e data. Regressione del bug "non riesco a correggere il nome".
func TestUpsertPersonUpdatesName(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	ctx := context.Background()

	res, err := upsertPerson(ctx, pool, map[string]any{
		"name":    "Stefano Baluardi",
		"aliases": []any{"Ste"},
		"data":    map[string]any{"is_user": true},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := res.(nodeResult).ID

	upd, err := upsertPerson(ctx, pool, map[string]any{
		"id":   float64(id),
		"name": "Stefano Baiardi",
	})
	if err != nil {
		t.Fatalf("update name: %v", err)
	}
	node := upd.(nodeResult)
	if node.Name != "Stefano Baiardi" {
		t.Errorf("name non aggiornato: %q", node.Name)
	}
	// alias e data preservati.
	if len(node.Aliases) != 1 || node.Aliases[0] != "Ste" {
		t.Errorf("alias non preservati: %v", node.Aliases)
	}
	if node.Data["is_user"] != true {
		t.Errorf("data non preservato: %v", node.Data)
	}
}

func TestFindNodeResolvesAlias(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	ctx := context.Background()

	if _, err := upsertPerson(ctx, pool, map[string]any{
		"name":    "Erik Muratori",
		"aliases": []any{"Mura"},
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	res, err := findNode(ctx, pool, map[string]any{"query": "mura"})
	if err != nil {
		t.Fatalf("find_node: %v", err)
	}
	candidates := res.(map[string]any)["candidates"].([]nodeResult)
	if len(candidates) == 0 {
		t.Fatal("nessun candidato trovato per l'alias 'mura'")
	}
	if candidates[0].Name != "Erik Muratori" {
		t.Errorf("primo candidato = %q, atteso 'Erik Muratori'", candidates[0].Name)
	}
}

func TestAddEventTransaction(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	ctx := context.Background()

	// Due persone + un luogo, con un arco tra le persone.
	erik := mustUpsertPerson(t, pool, "Erik Muratori", "Mura")
	lucia := mustUpsertPerson(t, pool, "Lucia", "")
	bar := mustUpsertPlace(t, pool, "Bar Basso")

	if _, err := linkNodes(ctx, pool, map[string]any{
		"from_id": float64(erik), "to_id": float64(lucia), "type": "amico",
	}); err != nil {
		t.Fatalf("link: %v", err)
	}

	res, err := addEvent(ctx, pool, map[string]any{
		"raw_text":        "aperitivo con Mura e Lucia al Bar Basso",
		"participant_ids": []any{float64(erik), float64(lucia)},
		"place_id":        float64(bar),
	})
	if err != nil {
		t.Fatalf("add_event: %v", err)
	}
	out := res.(map[string]any)

	// Evento creato.
	var nEvents, nParticipants int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events").Scan(&nEvents); err != nil {
		t.Fatal(err)
	}
	if nEvents != 1 {
		t.Errorf("eventi = %d, atteso 1", nEvents)
	}
	// Partecipanti inseriti.
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM event_participants").Scan(&nParticipants); err != nil {
		t.Fatal(err)
	}
	if nParticipants != 2 {
		t.Errorf("partecipanti = %d, atteso 2", nParticipants)
	}
	// last_seen aggiornato sull'arco erik↔lucia.
	var lastSeen *time.Time
	if err := pool.QueryRow(ctx, "SELECT last_seen FROM edges WHERE from_id=$1 AND to_id=$2", erik, lucia).Scan(&lastSeen); err != nil {
		t.Fatal(err)
	}
	if lastSeen == nil {
		t.Error("last_seen non aggiornato sull'arco tra i partecipanti")
	}
	if got, _ := out["edges_touched"].(int64); got < 1 {
		t.Errorf("edges_touched = %v, atteso >= 1", out["edges_touched"])
	}
}

func mustUpsertPerson(t *testing.T, pool *pgxpool.Pool, name, alias string) int64 {
	t.Helper()
	args := map[string]any{"name": name}
	if alias != "" {
		args["aliases"] = []any{alias}
	}
	res, err := upsertPerson(context.Background(), pool, args)
	if err != nil {
		t.Fatalf("setup persona %q: %v", name, err)
	}
	return res.(nodeResult).ID
}

func mustUpsertPlace(t *testing.T, pool *pgxpool.Pool, name string) int64 {
	t.Helper()
	res, err := upsertPlace(context.Background(), pool, map[string]any{"name": name})
	if err != nil {
		t.Fatalf("setup luogo %q: %v", name, err)
	}
	return res.(nodeResult).ID
}

package tools

import (
	"context"
	"errors"
	"testing"
)

// CreatePerson + UpdatePerson: merge di data e alias, e logging nel changelog.
func TestCreateAndUpdatePerson(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	ctx := context.Background()

	node, err := CreatePerson(ctx, pool, PersonInput{
		Name: "Anna Bianchi", Aliases: []string{"Anni"},
		Data: map[string]any{"lavoro": "avvocata"},
	})
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	// PATCH parziale: aggiunge un alias e un campo, non tocca il resto.
	upd, err := UpdatePerson(ctx, pool, node.ID, PersonInput{
		Aliases: []string{"Anna B."},
		Data:    map[string]any{"interessi": "vela"},
	})
	if err != nil {
		t.Fatalf("UpdatePerson: %v", err)
	}
	if len(upd.Aliases) != 2 {
		t.Errorf("alias non uniti: %+v", upd.Aliases)
	}
	if upd.Data["lavoro"] != "avvocata" || upd.Data["interessi"] != "vela" {
		t.Errorf("merge data errato: %+v", upd.Data)
	}

	// Il changelog deve avere registrato creazione + aggiornamento (actor user).
	feed, err := FeedList(ctx, pool, 10)
	if err != nil {
		t.Fatalf("FeedList: %v", err)
	}
	if len(feed) != 2 || feed[0].Action != "person_updated" || feed[1].Action != "person_created" {
		t.Errorf("changelog inatteso: %+v", feed)
	}
	if feed[0].Actor != "user" {
		t.Errorf("actor atteso 'user', ho %q", feed[0].Actor)
	}
}

// CreatePerson senza nome è input non valido.
func TestCreatePersonInvalid(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	_, err := CreatePerson(context.Background(), pool, PersonInput{})
	if !errors.Is(err, ErrInvalid) {
		t.Errorf("atteso ErrInvalid, ho %v", err)
	}
}

// CreateLink + DeleteLink, con validazione e logging.
func TestCreateAndDeleteLink(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	ctx := context.Background()

	a, _ := CreatePerson(ctx, pool, PersonInput{Name: "A"})
	b, _ := CreatePerson(ctx, pool, PersonInput{Name: "B"})

	edge, err := CreateLink(ctx, pool, LinkInput{FromID: a.ID, ToID: b.ID, Type: "collega", Note: "studio"})
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if edge.Data["note"] != "studio" {
		t.Errorf("nota non salvata: %+v", edge.Data)
	}

	// from_id == to_id → invalido.
	if _, err := CreateLink(ctx, pool, LinkInput{FromID: a.ID, ToID: a.ID, Type: "x"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("atteso ErrInvalid per self-link, ho %v", err)
	}
	// nodo inesistente → invalido (FK).
	if _, err := CreateLink(ctx, pool, LinkInput{FromID: a.ID, ToID: 99999, Type: "x"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("atteso ErrInvalid per FK, ho %v", err)
	}

	if err := DeleteLink(ctx, pool, edge.ID); err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	if err := DeleteLink(ctx, pool, edge.ID); !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("atteso ErrNodeNotFound per delete ripetuto, ho %v", err)
	}
}

// SearchPeople trova per alias e ignora i luoghi.
func TestSearchPeople(t *testing.T) {
	pool := testPool(t)
	cleanGraph(t, pool)
	ctx := context.Background()

	if _, err := CreatePerson(ctx, pool, PersonInput{Name: "Erik Muratori", Aliases: []string{"Mura"}}); err != nil {
		t.Fatal(err)
	}
	mustUpsert(t, ctx, pool, upsertPlace, map[string]any{"name": "Mura Café"}) // place, da ignorare

	res, err := SearchPeople(ctx, pool, "mura", 10)
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}
	if len(res) != 1 || res[0].Name != "Erik Muratori" {
		t.Errorf("ricerca alias errata: %+v", res)
	}
}

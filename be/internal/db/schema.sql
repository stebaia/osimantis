-- ============================================================================
-- Osimantis — schema del knowledge graph personale delle relazioni.
--
-- Modello: un GRAFO. I nodi (nodes) sono persone e luoghi; gli archi (edges)
-- sono le relazioni tra nodi; gli eventi (events) sono cose accadute che
-- coinvolgono uno o più nodi (event_participants). La chat con l'LLM è il core:
-- l'agente legge e scrive queste tabelle tramite i tool.
--
-- Applicato automaticamente dal container db al PRIMO avvio (montato in
-- /docker-entrypoint-initdb.d/, eseguito solo se il volume pgdata è vuoto).
-- ============================================================================

-- Estensione per gli embedding semantici (ricerca per similarità).
CREATE EXTENSION IF NOT EXISTS vector;
-- pg_trgm: ricerca fuzzy/parziale sui nomi (es. "muratori" → "Erik Muratori").
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ----------------------------------------------------------------------------
-- NODES — persone e luoghi.
-- ----------------------------------------------------------------------------
-- type: 'person' | 'place'  (estendibile in futuro)
-- name: nome canonico (es. "Erik Muratori", "Bar Basso")
-- aliases: soprannomi / nomi alternativi (es. {"Mura"}). Ricerca via GIN.
-- data: campi liberi e tipizzati a runtime (lavoro, interessi, fili aperti,
--       indirizzo per i luoghi, ...). JSONB così lo schema resta flessibile.
-- embedding: vettore della descrizione testuale del nodo (ricerca semantica).
CREATE TABLE IF NOT EXISTS nodes (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type        TEXT        NOT NULL CHECK (type IN ('person', 'place')),
    name        TEXT        NOT NULL,
    aliases     TEXT[]      NOT NULL DEFAULT '{}',
    data        JSONB       NOT NULL DEFAULT '{}',
    embedding   vector(768),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ricerca case-insensitive su name e sugli aliases.
CREATE INDEX IF NOT EXISTS idx_nodes_name_trgm   ON nodes USING gin (lower(name) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_nodes_aliases_gin ON nodes USING gin (aliases);
CREATE INDEX IF NOT EXISTS idx_nodes_type        ON nodes (type);

-- ----------------------------------------------------------------------------
-- EDGES — relazioni dirette tra due nodi.
-- ----------------------------------------------------------------------------
-- Una relazione "tra Mura e Lucia" è un edge person↔person; "Mura frequenta il
-- Bar Basso" è un edge person→place. Il grafo è gestito come DIRETTO (from→to);
-- per relazioni simmetriche l'agente può creare o interrogare entrambi i versi.
-- type: etichetta della relazione (es. 'amico', 'collega', 'frequenta', 'ex').
-- weight: forza/intensità del legame (default 1.0), aggiornabile nel tempo.
-- last_seen: ultima volta che i due nodi sono comparsi INSIEME in un evento.
-- data: note, fonte, e qualsiasi metadato (es. {"note":"non si parlano più"}).
CREATE TABLE IF NOT EXISTS edges (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    from_id     BIGINT      NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    to_id       BIGINT      NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    type        TEXT        NOT NULL,
    weight      REAL        NOT NULL DEFAULT 1.0,
    last_seen   TIMESTAMPTZ,
    data        JSONB       NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Una sola relazione di un certo tipo tra due nodi nello stesso verso:
    -- gli update sono upsert su (from_id, to_id, type).
    UNIQUE (from_id, to_id, type),
    CHECK (from_id <> to_id)
);

CREATE INDEX IF NOT EXISTS idx_edges_from ON edges (from_id);
CREATE INDEX IF NOT EXISTS idx_edges_to   ON edges (to_id);

-- ----------------------------------------------------------------------------
-- EVENTS — cose accadute (aperitivi, incontri, cambi di lavoro, litigi...).
-- ----------------------------------------------------------------------------
-- raw_text: la frase originale dell'utente da cui l'evento è stato estratto,
--           così resta sempre la fonte grezza (utile per audit e re-parsing).
-- summary: sintesi normalizzata dell'evento.
-- occurred_at: quando è successo (può differire da created_at = quando inserito).
-- place_id: luogo dell'evento, se presente.
-- data: metadati liberi (es. {"topic":"cambio lavoro"}).
-- embedding: per la ricerca semantica sugli eventi.
CREATE TABLE IF NOT EXISTS events (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    raw_text    TEXT        NOT NULL,
    summary     TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    place_id    BIGINT      REFERENCES nodes(id) ON DELETE SET NULL,
    data        JSONB       NOT NULL DEFAULT '{}',
    embedding   vector(768),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_occurred_at ON events (occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_place       ON events (place_id);

-- ----------------------------------------------------------------------------
-- EVENT_PARTICIPANTS — quali nodi (persone) hanno preso parte a un evento.
-- ----------------------------------------------------------------------------
-- role: ruolo opzionale nel contesto dell'evento (es. 'organizzatore').
CREATE TABLE IF NOT EXISTS event_participants (
    event_id    BIGINT      NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    node_id     BIGINT      NOT NULL REFERENCES nodes(id)  ON DELETE CASCADE,
    role        TEXT,
    PRIMARY KEY (event_id, node_id)
);

CREATE INDEX IF NOT EXISTS idx_event_participants_node ON event_participants (node_id);

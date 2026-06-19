-- ============================================================================
-- Osimantis — query di riferimento per il layer dei tool.
--
-- Raccolta delle query SQL (parametrizzate) usate dai tool in internal/tools.
-- Sono scritte con placeholder pgx ($1, $2, ...). Servono sia come riferimento
-- per l'implementazione Go, sia (in futuro) come base per sqlc.
--
-- Coerenti con schema.sql: nodes, edges, events, event_participants.
-- ============================================================================


-- ---------------------------------------------------------------------------
-- 1. find_node — risolve un riferimento (nome o alias) a un nodo.
--    Cerca case-insensitive su name, sugli aliases e in fuzzy (trigram).
--    Restituisce i candidati ordinati per rilevanza: match esatto > alias >
--    fuzzy. Permette all'agente di disambiguare se ci sono più candidati.
-- $1 = testo cercato (es. 'Mura'), $2 = type opzionale ('person'|'place'|NULL)
-- ---------------------------------------------------------------------------
SELECT id, type, name, aliases, data,
       CASE
           WHEN lower(name) = lower($1)                          THEN 3
           WHEN EXISTS (SELECT 1 FROM unnest(aliases) a
                        WHERE lower(a) = lower($1))              THEN 2
           ELSE 1
       END AS match_rank,
       similarity(lower(name), lower($1)) AS name_similarity
FROM nodes
WHERE ($2::text IS NULL OR type = $2)
  AND (
        lower(name) = lower($1)
        OR EXISTS (SELECT 1 FROM unnest(aliases) a WHERE lower(a) = lower($1))
        OR lower(name) % lower($1)            -- fuzzy trigram
        OR lower(name) LIKE '%' || lower($1) || '%'
      )
ORDER BY match_rank DESC, name_similarity DESC
LIMIT 10;


-- ---------------------------------------------------------------------------
-- 2. upsert_person / upsert_place — crea o aggiorna un nodo, fondendo gli
--    alias senza duplicati e mergiando i campi data.
-- $1 = type, $2 = name, $3 = aliases (text[]), $4 = data (jsonb)
-- L'upsert avviene per name canonico (case-insensitive) tramite ON CONFLICT
-- su un indice unico logico gestito a livello applicativo: qui mostriamo la
-- forma INSERT ... e l'UPDATE di merge corrispondente.
-- ---------------------------------------------------------------------------
-- INSERT (nuovo nodo):
INSERT INTO nodes (type, name, aliases, data)
VALUES ($1, $2, $3, $4)
RETURNING id, type, name, aliases, data;

-- UPDATE (nodo esistente, $1 = id): merge alias (dedup) + merge data.
UPDATE nodes
SET aliases    = ARRAY(SELECT DISTINCT unnest(aliases || $2::text[])),
    data       = data || $3::jsonb,
    updated_at = now()
WHERE id = $1
RETURNING id, type, name, aliases, data;


-- ---------------------------------------------------------------------------
-- 3. link_nodes — crea/aggiorna una relazione tra due nodi (upsert su
--    from_id,to_id,type). Mergia eventuali note/source nel campo data.
-- $1 = from_id, $2 = to_id, $3 = type, $4 = weight, $5 = data (jsonb, es. note)
-- ---------------------------------------------------------------------------
INSERT INTO edges (from_id, to_id, type, weight, data)
VALUES ($1, $2, $3, COALESCE($4, 1.0), $5)
ON CONFLICT (from_id, to_id, type) DO UPDATE
SET weight     = COALESCE($4, edges.weight),
    data       = edges.data || $5::jsonb,
    updated_at = now()
RETURNING id, from_id, to_id, type, weight, last_seen, data;


-- ---------------------------------------------------------------------------
-- 4. get_neighbors — vicinato di un nodo: relazioni uscenti ED entranti,
--    con il nodo collegato e i metadati dell'arco.
-- $1 = node_id
-- ---------------------------------------------------------------------------
SELECT e.id            AS edge_id,
       e.type          AS relation,
       e.weight,
       e.last_seen,
       e.data          AS edge_data,
       CASE WHEN e.from_id = $1 THEN 'out' ELSE 'in' END AS direction,
       n.id            AS neighbor_id,
       n.type          AS neighbor_type,
       n.name          AS neighbor_name,
       n.aliases       AS neighbor_aliases
FROM edges e
JOIN nodes n ON n.id = CASE WHEN e.from_id = $1 THEN e.to_id ELSE e.from_id END
WHERE e.from_id = $1 OR e.to_id = $1
ORDER BY e.weight DESC, e.last_seen DESC NULLS LAST;


-- ---------------------------------------------------------------------------
-- 5. add_event — inserimento di un evento (da fare in TRANSAZIONE nel codice).
--    5a inserisce l'evento, 5b i partecipanti, 5c aggiorna last_seen SOLO sugli
--    archi che collegano i partecipanti di QUESTO evento (e verso il luogo).
-- ---------------------------------------------------------------------------
-- 5a. inserisci evento  ($1=raw_text, $2=summary, $3=occurred_at, $4=place_id, $5=data)
INSERT INTO events (raw_text, summary, occurred_at, place_id, data)
VALUES ($1, $2, COALESCE($3, now()), $4, $5)
RETURNING id, occurred_at;

-- 5b. inserisci un partecipante  ($1=event_id, $2=node_id, $3=role)
INSERT INTO event_participants (event_id, node_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (event_id, node_id) DO NOTHING;

-- 5c. aggiorna last_seen sugli archi tra i partecipanti dell'evento.
--     $1 = array degli id dei nodi coinvolti (participanti + eventuale luogo)
--     $2 = occurred_at dell'evento
UPDATE edges
SET last_seen  = GREATEST(COALESCE(last_seen, $2::timestamptz), $2::timestamptz),
    updated_at = now()
WHERE from_id = ANY($1::bigint[])
  AND to_id   = ANY($1::bigint[]);


-- ---------------------------------------------------------------------------
-- 6. recent_events — ultimi eventi (opzionalmente filtrati per nodo coinvolto).
-- $1 = node_id opzionale (NULL = tutti), $2 = limit
-- ---------------------------------------------------------------------------
SELECT ev.id, ev.raw_text, ev.summary, ev.occurred_at, ev.place_id, ev.data,
       COALESCE(
           array_agg(DISTINCT p.node_id) FILTER (WHERE p.node_id IS NOT NULL),
           '{}'
       ) AS participant_ids
FROM events ev
LEFT JOIN event_participants p ON p.event_id = ev.id
WHERE $1::bigint IS NULL
   OR ev.id IN (SELECT event_id FROM event_participants WHERE node_id = $1)
GROUP BY ev.id
ORDER BY ev.occurred_at DESC
LIMIT $2;


-- ---------------------------------------------------------------------------
-- 7. prediction_features — segnali aggregati su una persona, per dare
--    all'LLM materiale per ragionare/predire (NON è una predizione: è il
--    contesto fattuale che l'LLM userà).
-- $1 = node_id
-- ---------------------------------------------------------------------------
SELECT
    n.id, n.name, n.aliases, n.data,
    -- numero di legami e forza media
    (SELECT count(*) FROM edges e WHERE e.from_id = n.id OR e.to_id = n.id)        AS degree,
    (SELECT avg(weight) FROM edges e WHERE e.from_id = n.id OR e.to_id = n.id)     AS avg_weight,
    -- ultimo contatto registrato (max last_seen sugli archi della persona)
    (SELECT max(last_seen) FROM edges e WHERE e.from_id = n.id OR e.to_id = n.id)  AS last_contact,
    -- numero di eventi negli ultimi 90 giorni
    (SELECT count(*) FROM event_participants ep
        JOIN events ev ON ev.id = ep.event_id
        WHERE ep.node_id = n.id AND ev.occurred_at > now() - interval '90 days')   AS events_last_90d,
    -- legami "a rischio": archi non visti da oltre 180 giorni
    (SELECT count(*) FROM edges e
        WHERE (e.from_id = n.id OR e.to_id = n.id)
          AND e.last_seen IS NOT NULL
          AND e.last_seen < now() - interval '180 days')                           AS stale_edges
FROM nodes n
WHERE n.id = $1;


-- ---------------------------------------------------------------------------
-- 8. search_semantic — ricerca per similarità coseno con pgvector (Step 9).
--    Trova i nodi più vicini a un embedding di query.
-- $1 = embedding della query (vector(768)), $2 = limit
-- ---------------------------------------------------------------------------
SELECT id, type, name, aliases, data,
       1 - (embedding <=> $1) AS similarity
FROM nodes
WHERE embedding IS NOT NULL
ORDER BY embedding <=> $1
LIMIT $2;


-- ---------------------------------------------------------------------------
-- 9. graph_dump — tutto il grafo per l'endpoint GET /graph (app Flutter).
-- ---------------------------------------------------------------------------
-- nodi:
SELECT id, type, name, data FROM nodes ORDER BY id;
-- archi:
SELECT id, from_id, to_id, type, weight FROM edges ORDER BY id;

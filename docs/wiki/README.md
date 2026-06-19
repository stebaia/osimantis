# Wiki Osimantis

Memoria dell'infrastruttura del progetto. Documenta **decisioni** e **stato corrente**, non tutorial.
Pensata per essere letta da un LLM: fatti espliciti, formato tabellare/elenco, una decisione per riga.

## Cos'è Osimantis

Backend di un **knowledge graph personale delle relazioni**: un sistema che memorizza
persone, entità e i legami tra loro, interrogabile in linguaggio naturale tramite un LLM
con accesso a tool (function calling) e ricerca semantica via embedding.

## Indice

| File | Contenuto |
|------|-----------|
| [00-overview.md](00-overview.md) | Visione, obiettivi, glossario |
| [01-architecture.md](01-architecture.md) | Struttura del progetto, moduli, flusso di una richiesta |
| [02-backend.md](02-backend.md) | Stack BE, scelte tecniche, vincoli obbligatori |
| [03-config.md](03-config.md) | Variabili d'ambiente e configurazione |
| [04-database.md](04-database.md) | Postgres, pgvector, schema (TBD) |
| [05-decisions.md](05-decisions.md) | Decision log (ADR sintetici, append-only) |
| [06-roadmap.md](06-roadmap.md) | Stato degli step e prossimi passi |

## Convenzioni di questa wiki

- **Append-only** per `05-decisions.md`: le decisioni non si cancellano, si supersano.
- Ogni decisione ha: data (assoluta), scelta, motivazione, eventuale alternativa scartata.
- `TBD` = da decidere. `TODO` = da implementare (deciso ma non fatto).
- Le date sono assolute (YYYY-MM-DD), mai relative.
- Quando una scelta cambia: aggiorna il file tematico **e** aggiungi una riga nel decision log.

_Ultimo aggiornamento: 2026-06-17_

package tools

// Definitions sono le function declarations dei tool, nel formato atteso da
// Gemini (parameters = JSON Schema). Rispecchiano llm_contract.md ed è ciò che
// l'agente passa all'LLM come toolDefs. L'ordine non è significativo.
//
// Mantenere allineato a llm_contract.md: se cambia uno, cambiare l'altro.
var Definitions = []map[string]any{
	{
		"name":        "find_node",
		"description": "Cerca un nodo (persona o luogo) per nome o alias, case-insensitive e fuzzy. Restituisce i candidati con id, nome, alias e dati. Usare sempre prima di creare o collegare nodi.",
		"parameters": obj(props{
			"query": str("Nome o alias da cercare, es. 'Mura'."),
			"type":  enumStr("Filtro opzionale sul tipo.", "person", "place"),
		}, "query"),
	},
	{
		"name":        "get_user",
		"description": "Restituisce il nodo che rappresenta l'UTENTE con cui stai parlando (quello con is_user). Usalo SEMPRE quando l'utente parla di sé ('io', 'me', 'chi sono', 'i miei amici') invece di cercarlo per nome. Restituisce user=null se non è ancora stato creato.",
		"parameters":  obj(props{}),
	},
	{
		"name":        "upsert_person",
		"description": "Crea una persona o aggiorna quella esistente (passando id). Gli alias vengono uniti senza duplicati; i campi data vengono mergiati.",
		"parameters": obj(props{
			"id":      integer("Id del nodo se si aggiorna una persona esistente. Omettere per crearne una nuova."),
			"name":    str("Nome canonico, es. 'Erik Muratori'."),
			"aliases": arrStr("Soprannomi/nomi alternativi, es. ['Mura']."),
			"data":    object("Campi liberi: lavoro, interessi, fili_aperti, ecc."),
		}, "name"),
	},
	{
		"name":        "upsert_place",
		"description": "Crea un luogo o aggiorna quello esistente (passando id). Alias uniti senza duplicati; data mergiato.",
		"parameters": obj(props{
			"id":      integer("Id del luogo se si aggiorna. Omettere per crearlo."),
			"name":    str("Nome del luogo, es. 'Bar Basso'."),
			"aliases": arrStr("Soprannomi/nomi alternativi."),
			"data":    object("Es. indirizzo, tipo (bar, palestra), note."),
		}, "name"),
	},
	{
		"name":        "link_nodes",
		"description": "Crea o aggiorna una relazione diretta tra due nodi (upsert su from_id,to_id,type). Per relazioni tra terzi (es. Mura↔Lucia) usare i loro id, non quello dell'utente.",
		"parameters": obj(props{
			"from_id": integer("Id nodo di partenza."),
			"to_id":   integer("Id nodo di arrivo."),
			"type":    str("Tipo di relazione, es. amico, ex, collega, conflitto, frequenta."),
			"weight":  number("Forza del legame, default 1.0."),
			"note":    str("Nota/sfumatura sulla relazione, salvata in data.note."),
		}, "from_id", "to_id", "type"),
	},
	{
		"name":        "add_event",
		"description": "Registra un evento: testo grezzo, partecipanti (id persone), eventuale luogo (id). Aggiorna last_seen sugli archi tra i partecipanti di questo evento.",
		"parameters": obj(props{
			"raw_text":        str("Frase originale dell'utente."),
			"summary":         str("Sintesi normalizzata dell'evento."),
			"occurred_at":     str("Data/ora ISO 8601. Default: ora."),
			"participant_ids": arrInt("Id delle persone coinvolte."),
			"place_id":        integer("Id del luogo, se presente."),
			"data":            object("Metadati, es. {'topic':'cambio lavoro'}."),
		}, "raw_text", "participant_ids"),
	},
	{
		"name":        "get_neighbors",
		"description": "Restituisce il vicinato di un nodo: tutte le relazioni con i nodi collegati, direzione, peso, last_seen e note.",
		"parameters": obj(props{
			"node_id": integer("Id del nodo."),
		}, "node_id"),
	},
	{
		"name":        "recent_events",
		"description": "Restituisce gli eventi più recenti. Se node_id è dato, solo quelli che coinvolgono quel nodo.",
		"parameters": obj(props{
			"node_id": integer("Filtro opzionale per persona/luogo."),
			"limit":   integer("Numero massimo di eventi, default 10."),
		}),
	},
	{
		"name":        "prediction_features",
		"description": "Restituisce segnali aggregati su una persona (grado, peso medio dei legami, ultimo contatto, eventi recenti, legami a rischio) come base fattuale per ragionare o fare ipotesi.",
		"parameters": obj(props{
			"node_id": integer("Id della persona."),
		}, "node_id"),
	},
	{
		"name":        "search_semantic",
		"description": "Ricerca semantica per similarità: dato un testo, trova i nodi più affini per significato. Disponibile solo quando gli embedding sono attivi.",
		"parameters": obj(props{
			"query": str("Testo della ricerca."),
			"limit": integer("Numero di risultati, default 5."),
		}, "query"),
	},
}

// --- piccoli costruttori per tenere le declarations leggibili ---------------

type props = map[string]any

func obj(p props, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": p}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func integer(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
func number(desc string) map[string]any { return map[string]any{"type": "number", "description": desc} }
func object(desc string) map[string]any { return map[string]any{"type": "object", "description": desc} }

func arrStr(desc string) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "string"}}
}
func arrInt(desc string) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "integer"}}
}
func enumStr(desc string, vals ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": vals}
}

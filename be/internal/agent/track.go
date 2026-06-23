package agent

import (
	"context"
	"encoding/json"
)

// Tracciamento dei nodi PERSONA toccati durante un turno.
//
// Obiettivo: dopo aver eseguito i tool per un messaggio, sapere DI QUALI persone
// si è parlato, così il frontend può aprire la scheda giusta (l'ultima toccata).
// I tool del grafo restituiscono già gli id: upsert_person/upsert_place ritornano
// il nodo, find_node ritorna {"candidates":[...]}. Qui non interroghiamo il DB:
// leggiamo gli id/name direttamente dai risultati dei tool.
//
// È per-richiesta come il pseudonymizer: ogni chiamata a /chat ha il suo executor
// con il suo accumulatore, così i nodi toccati non sfuggono tra richieste diverse.

// TouchedNode è un nodo persona menzionato in un turno, nella forma che serve al
// frontend per aprire la scheda.
type TouchedNode struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// trackingExecutor avvolge un ToolExecutor e raccoglie i nodi persona che
// compaiono nei risultati dei tool, in ordine di comparsa (deduplicati per id).
type trackingExecutor struct {
	inner  ToolExecutor
	order  []int64               // id nell'ordine di prima comparsa
	byID   map[int64]TouchedNode // id → nodo (l'ultimo name visto vince)
	isUser map[int64]bool        // id → è il nodo utente (data.is_user)
}

// NewTrackingExecutor avvolge exec così che ogni nodo persona presente nei
// risultati dei tool venga registrato. Usa Touched() per leggere l'esito.
func NewTrackingExecutor(exec ToolExecutor) *trackingExecutor {
	return &trackingExecutor{
		inner:  exec,
		byID:   map[int64]TouchedNode{},
		isUser: map[int64]bool{},
	}
}

func (e *trackingExecutor) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	out, err := e.inner.Call(ctx, name, args)
	if err == nil {
		e.scan(out)
	}
	return out, err
}

// Touched restituisce i nodi persona toccati nell'ordine di prima comparsa.
// L'ULTIMO della lista è il più recente: il frontend apre la sua scheda.
//
// Il nodo UTENTE (data.is_user) viene messo in coda solo se non c'è nessun altro:
// così, parlando di un amico, la scheda aperta è quella dell'amico e non la tua
// (l'utente compare quasi sempre tra i toccati perché i legami partono da lui).
// Se l'utente è l'unico toccato (es. "chi sono?"), resta e viene mostrato.
func (e *trackingExecutor) Touched() []TouchedNode {
	others := make([]TouchedNode, 0, len(e.order))
	user := make([]TouchedNode, 0, 1)
	for _, id := range e.order {
		if e.isUser[id] {
			user = append(user, e.byID[id])
		} else {
			others = append(others, e.byID[id])
		}
	}
	if len(others) == 0 {
		return user
	}
	return others
}

// scan ispeziona un risultato di tool e registra ogni oggetto che sembra un nodo
// persona (ha id, type=="person" e name). Marshalliamo in JSON e camminiamo la
// struttura: così gestiamo sia i risultati struct (upsert → nodeResult) sia quelli
// annidati (find_node → candidates[]), senza dipendere dai tipi interni di tools.
func (e *trackingExecutor) scan(out any) {
	raw, err := json.Marshal(out)
	if err != nil {
		return
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return
	}
	e.walk(v)
}

func (e *trackingExecutor) walk(v any) {
	switch t := v.(type) {
	case map[string]any:
		e.consider(t)
		for _, child := range t {
			e.walk(child)
		}
	case []any:
		for _, child := range t {
			e.walk(child)
		}
	}
}

// consider registra l'oggetto come nodo toccato se è una persona con id e name.
func (e *trackingExecutor) consider(m map[string]any) {
	if typ, _ := m["type"].(string); typ != "person" {
		return
	}
	name, _ := m["name"].(string)
	if name == "" {
		return
	}
	// json.Unmarshal decodifica i numeri come float64.
	idf, ok := m["id"].(float64)
	if !ok {
		return
	}
	id := int64(idf)
	if id <= 0 {
		return
	}
	if _, seen := e.byID[id]; !seen {
		e.order = append(e.order, id)
	}
	e.byID[id] = TouchedNode{ID: id, Name: name}
	// Il flag is_user può comparire in data.is_user (upsert/get_user) oppure come
	// is_user a livello dell'oggetto. Una volta true, resta true.
	if isUserFlag(m["data"]) || isTrue(m["is_user"]) {
		e.isUser[id] = true
	}
}

// isUserFlag legge data.is_user da una mappa data (se presente).
func isUserFlag(data any) bool {
	m, ok := data.(map[string]any)
	if !ok {
		return false
	}
	return isTrue(m["is_user"])
}

// isTrue interpreta true sia come bool sia come stringa "true" (JSONB).
func isTrue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	}
	return false
}

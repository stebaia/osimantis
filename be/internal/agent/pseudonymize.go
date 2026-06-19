package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
)

// Pseudonimizzazione verso l'LLM.
//
// Obiettivo: ridurre i nomi reali di persone/luoghi che spediamo all'LLM. I tool
// del grafo identificano i nodi con id numerici (from_id, node_id, participant_ids,
// ...), quindi le SCRITTURE non hanno bisogno dei nomi: il modello lavora con id e
// pseudonimi. I nomi reali compaiono invece nei RISULTATI dei tool di lettura
// (find_node, get_neighbors, recent_events, prediction_features) nei campi name,
// aliases, neighbor_name, e nel testo grezzo degli eventi.
//
// Strategia:
//   - in USCITA (risultato tool → LLM): impariamo le coppie (id → nome reale) dai
//     risultati stessi, generiamo uno pseudonimo STABILE derivato dall'id
//     (es. "Persona_1a2b3c4d") e sostituiamo ogni occorrenza del nome reale con lo
//     pseudonimo prima di re-iniettare il risultato nello storico.
//   - in INGRESSO (args dal modello → tool): se il modello rimette uno pseudonimo
//     in un campo testuale (query, name, note, raw_text, ...), lo riconvertiamo nel
//     nome reale prima di eseguire il tool contro il DB.
//
// LIMITE NOTO (documentato apposta): la frase che l'utente digita può contenere
// nomi reali, e quella frase la vede comunque l'LLM (entra nello storico come
// messaggio utente, non passa da qui). Questa pseudonimizzazione copre solo i dati
// che ESTRAIAMO dal grafo, non l'input libero dell'utente.
type pseudonymizer struct {
	mu           sync.Mutex
	realToPseudo map[string]string // nome reale (lower) → pseudonimo
	pseudoToReal map[string]string // pseudonimo → nome reale (forma originale)
}

func newPseudonymizer() *pseudonymizer {
	return &pseudonymizer{
		realToPseudo: map[string]string{},
		pseudoToReal: map[string]string{},
	}
}

// pseudoExecutor avvolge un ToolExecutor applicando la pseudonimizzazione su
// args (ingresso) e risultati (uscita). È per-richiesta: ogni conversazione ha la
// sua mappa, così pseudonimi e nomi non sfuggono tra richieste diverse.
type pseudoExecutor struct {
	inner ToolExecutor
	p     *pseudonymizer
}

// NewPseudonymizingExecutor avvolge exec così che i nomi reali estratti dal grafo
// vengano sostituiti con pseudonimi stabili prima di raggiungere l'LLM, e gli
// pseudonimi rimessi dal modello vengano risolti ai nomi reali prima del DB.
func NewPseudonymizingExecutor(exec ToolExecutor) ToolExecutor {
	return &pseudoExecutor{inner: exec, p: newPseudonymizer()}
}

func (e *pseudoExecutor) Call(ctx context.Context, name string, args map[string]any) (any, error) {
	// Ingresso: il modello può aver usato pseudonimi appresi in giri precedenti.
	deAnon := e.p.resolve(args)

	out, err := e.inner.Call(ctx, name, deAnon)
	if err != nil {
		// Anche i messaggi d'errore possono contenere nomi reali (es. echo di un
		// valore): meglio pseudonimizzarli prima che tornino all'LLM.
		return out, e.p.maskError(err)
	}

	// Impara le coppie id→nome dal risultato, poi maschera i nomi reali.
	e.p.learn(out)
	return e.p.mask(out), nil
}

// pseudoFor genera (e memorizza) uno pseudonimo stabile per un nodo dato id, tipo
// e nome reale. Lo pseudonimo dipende solo dall'id, quindi è stabile tra richieste.
func (p *pseudonymizer) pseudoFor(id int64, nodeType, realName string) string {
	prefix := "Nodo"
	switch nodeType {
	case "person":
		prefix = "Persona"
	case "place":
		prefix = "Luogo"
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", nodeType, id)))
	pseudo := prefix + "_" + hex.EncodeToString(sum[:4])

	if realName != "" {
		p.realToPseudo[strings.ToLower(realName)] = pseudo
		p.pseudoToReal[pseudo] = realName
	}
	return pseudo
}

// learn scorre un risultato di tool e registra ogni coppia (id, name[, aliases])
// che trova, così mask saprà cosa sostituire. Riconosce le forme prodotte dai
// nostri tool: oggetti con id+name(+aliases) e il pattern neighbor_id/neighbor_name.
func (p *pseudonymizer) learn(v any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.learnValue(v)
}

func (p *pseudonymizer) learnValue(v any) {
	switch t := v.(type) {
	case map[string]any:
		p.learnNode(t, "id", "type", "name", "aliases")
		p.learnNode(t, "neighbor_id", "neighbor_type", "neighbor_name", "neighbor_aliases")
		for _, val := range t {
			p.learnValue(val)
		}
	case []any:
		for _, item := range t {
			p.learnValue(item)
		}
	}
}

// learnNode registra un nodo da una mappa usando i nomi di campo dati, se presenti.
func (p *pseudonymizer) learnNode(m map[string]any, idKey, typeKey, nameKey, aliasesKey string) {
	id, ok := toInt64(m[idKey])
	if !ok {
		return
	}
	nodeType, _ := m[typeKey].(string)
	name, _ := m[nameKey].(string)
	if name != "" {
		p.pseudoFor(id, nodeType, name)
	}
	for _, a := range toStringSlice(m[aliasesKey]) {
		if a == "" {
			continue
		}
		pseudo := p.pseudoFor(id, nodeType, name)
		p.realToPseudo[strings.ToLower(a)] = pseudo
	}
}

// mask restituisce una copia del valore in cui ogni nome reale noto è sostituito
// dal suo pseudonimo. Non muta l'originale (che resta com'è nei log/DB).
func (p *pseudonymizer) mask(v any) any {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.transform(v, p.realToPseudo, true)
}

// resolve restituisce una copia degli args in cui ogni pseudonimo noto è
// risolto al nome reale, così il tool lavora sui dati veri.
func (p *pseudonymizer) resolve(args map[string]any) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	out, _ := p.transform(args, p.pseudoToReal, false).(map[string]any)
	return out
}

// maskError pseudonimizza il testo di un errore.
func (p *pseudonymizer) maskError(err error) error {
	if err == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	masked := p.replaceInString(err.Error(), p.realToPseudo, true)
	if masked == err.Error() {
		return err
	}
	return fmt.Errorf("%s", masked)
}

// transform copia ricorsivamente v sostituendo nelle stringhe le chiavi della
// mappa con i relativi valori. caseInsensitive controlla il matching (true per i
// nomi reali, che possono comparire con casing diverso).
func (p *pseudonymizer) transform(v any, repl map[string]string, caseInsensitive bool) any {
	switch t := v.(type) {
	case string:
		return p.replaceInString(t, repl, caseInsensitive)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = p.transform(val, repl, caseInsensitive)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = p.transform(item, repl, caseInsensitive)
		}
		return out
	default:
		return v
	}
}

// replaceInString sostituisce in s ogni occorrenza delle chiavi di repl. Le
// chiavi sono ordinate dalla più lunga alla più corta per evitare che una
// sottostringa venga sostituita prima del nome completo che la contiene.
func (p *pseudonymizer) replaceInString(s string, repl map[string]string, caseInsensitive bool) string {
	if s == "" || len(repl) == 0 {
		return s
	}
	keys := sortedKeysByLen(repl)
	for _, k := range keys {
		if k == "" {
			continue
		}
		if caseInsensitive {
			s = replaceAllCaseInsensitive(s, k, repl[k])
		} else {
			s = strings.ReplaceAll(s, k, repl[k])
		}
	}
	return s
}

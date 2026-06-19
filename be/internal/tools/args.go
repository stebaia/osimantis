package tools

import (
	"encoding/json"
	"fmt"
)

// Gli argomenti dei tool arrivano da JSON (decoded in map[string]any), quindi:
// numeri = float64, array = []any, oggetti = map[string]any. Questi helper
// estraggono e validano i tipi con errori espliciti, così un argomento sbagliato
// dell'LLM diventa un errore leggibile (re-iniettabile nel loop) invece di un panic.

// argString legge una stringa obbligatoria.
func argString(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", fmt.Errorf("argomento %q mancante", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("argomento %q deve essere una stringa", key)
	}
	return s, nil
}

// argStringOpt legge una stringa opzionale ("" se assente).
func argStringOpt(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("argomento %q deve essere una stringa", key)
	}
	return s, nil
}

// argInt64 legge un intero obbligatorio (accetta float64 da JSON o int).
func argInt64(args map[string]any, key string) (int64, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("argomento %q mancante", key)
	}
	return toInt64(key, v)
}

// argInt64Opt legge un intero opzionale; ok=false se assente.
func argInt64Opt(args map[string]any, key string) (val int64, ok bool, err error) {
	v, present := args[key]
	if !present || v == nil {
		return 0, false, nil
	}
	n, err := toInt64(key, v)
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}

// argFloat64Opt legge un float opzionale; ok=false se assente.
func argFloat64Opt(args map[string]any, key string) (val float64, ok bool, err error) {
	v, present := args[key]
	if !present || v == nil {
		return 0, false, nil
	}
	switch n := v.(type) {
	case float64:
		return n, true, nil
	case int:
		return float64(n), true, nil
	case int64:
		return float64(n), true, nil
	default:
		return 0, false, fmt.Errorf("argomento %q deve essere un numero", key)
	}
}

func toInt64(key string, v any) (int64, error) {
	switch n := v.(type) {
	case float64:
		return int64(n), nil
	case int:
		return int64(n), nil
	case int64:
		return n, nil
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, fmt.Errorf("argomento %q non è un intero valido: %w", key, err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("argomento %q deve essere un intero", key)
	}
}

// argStringSlice legge un array di stringhe opzionale (nil se assente).
func argStringSlice(args map[string]any, key string) ([]string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("argomento %q deve essere un array", key)
	}
	out := make([]string, 0, len(raw))
	for i, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("argomento %q[%d] deve essere una stringa", key, i)
		}
		out = append(out, s)
	}
	return out, nil
}

// argInt64Slice legge un array di interi (nil se assente).
func argInt64Slice(args map[string]any, key string) ([]int64, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("argomento %q deve essere un array", key)
	}
	out := make([]int64, 0, len(raw))
	for i, item := range raw {
		n, err := toInt64(fmt.Sprintf("%s[%d]", key, i), item)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// argJSONOpt legge un oggetto JSON opzionale e lo restituisce serializzato come
// []byte pronto per essere passato a una colonna JSONB. Default: "{}".
func argJSONOpt(args map[string]any, key string) ([]byte, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return []byte("{}"), nil
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("argomento %q deve essere un oggetto", key)
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("serializzazione argomento %q: %w", key, err)
	}
	return b, nil
}

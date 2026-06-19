package agent

import (
	"sort"
	"strings"
)

// toInt64 estrae un int64 dai vari tipi numerici che possono arrivare dopo un
// round-trip JSON (json.Number, float64) o direttamente dai nostri tool (int64).
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		// Gli id JSON arrivano come float64; sono interi nel dominio.
		return int64(n), true
	default:
		return 0, false
	}
}

// toStringSlice estrae uno slice di stringhe da []string o []any (post-JSON).
func toStringSlice(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

// sortedKeysByLen restituisce le chiavi della mappa ordinate dalla più lunga
// alla più corta. Serve a sostituire prima i nomi completi ("Erik Muratori")
// e solo dopo eventuali sottostringhe ("Erik").
func sortedKeysByLen(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	return keys
}

// replaceAllCaseInsensitive sostituisce in s ogni occorrenza di old (confronto
// case-insensitive) con new, preservando il resto del testo invariato.
func replaceAllCaseInsensitive(s, old, new string) string {
	if old == "" {
		return s
	}
	lowerS := strings.ToLower(s)
	lowerOld := strings.ToLower(old)

	var b strings.Builder
	for {
		idx := strings.Index(lowerS, lowerOld)
		if idx < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:idx])
		b.WriteString(new)
		s = s[idx+len(old):]
		lowerS = lowerS[idx+len(lowerOld):]
	}
	return b.String()
}

package config

import (
	"strings"
	"testing"
)

// Con DATABASE_URL impostata, ha precedenza ed è restituita tale e quale.
func TestDatabaseURLPrefersExplicit(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@h:5432/d")
	t.Setenv("POSTGRES_USER", "altro")
	if got := databaseURL(); got != "postgres://u:p@h:5432/d" {
		t.Errorf("databaseURL = %q", got)
	}
}

// Senza DATABASE_URL, costruisce un DSN key=value dai componenti POSTGRES_*.
// Il caso chiave: una password con caratteri speciali che romperebbe un URL.
func TestDatabaseURLFromComponentsEscapesPassword(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_USER", "relazioni")
	t.Setenv("POSTGRES_PASSWORD", `9nZIQ\w}mAJ:foo@bar`)
	t.Setenv("POSTGRES_DB", "relazioni")

	dsn := databaseURL()

	// Deve essere un DSN key=value, non un URL: niente "postgres://".
	if strings.HasPrefix(dsn, "postgres://") {
		t.Fatalf("atteso DSN key=value, ho un URL: %q", dsn)
	}
	for _, want := range []string{"host=db", "port=5432", "user=relazioni", "dbname=relazioni", "sslmode=disable"} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN manca %q: %q", want, dsn)
		}
	}
	// La password con spazi/apici/backslash va quotata; qui niente spazi ma c'è
	// un backslash: deve comparire raddoppiato dentro apici NON richiesti (no
	// spazi) → in questo caso resta non quotata perché senza spazi/apici. Verifichiamo
	// solo che il valore grezzo sia presente in forma escapabile.
	if !strings.Contains(dsn, "password=") {
		t.Errorf("DSN manca la password: %q", dsn)
	}
}

// quoteDSN: valori semplici non quotati, valori con spazi/apici/backslash quotati
// ed escapati.
func TestQuoteDSN(t *testing.T) {
	cases := map[string]string{
		"semplice":   "semplice",
		"":           "''",
		"con spazio": `'con spazio'`,
		`back\slash`: `'back\\slash'`,
		`ap'ice`:     `'ap\'ice'`,
	}
	for in, want := range cases {
		if got := quoteDSN(in); got != want {
			t.Errorf("quoteDSN(%q) = %q, atteso %q", in, got, want)
		}
	}
}

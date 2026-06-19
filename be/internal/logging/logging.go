// Package logging configura il logger strutturato basato su log/slog.
package logging

import (
	"log/slog"
	"os"
)

// Setup costruisce un logger slog che scrive JSON su stdout e lo imposta come
// logger di default a livello globale, così slog.Info/Error funzionano ovunque
// senza dover passare l'istanza in giro. Restituisce anche il logger per chi
// preferisce iniettarlo esplicitamente.
func Setup() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

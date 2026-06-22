// Package logger menyediakan structured logger (slog) untuk seluruh aplikasi.
package logger

import (
	"log/slog"
	"os"
)

// New membuat slog.Logger. Di production pakai JSON handler, selain itu text.
func New(env string) *slog.Logger {
	level := slog.LevelInfo
	if env != "production" {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	if env == "production" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

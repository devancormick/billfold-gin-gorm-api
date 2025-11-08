package config

import (
	"log"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

// ConnectSentry initializes error tracking. No-op if SENTRY_DSN is unset,
// so local dev doesn't require a Sentry project.
func ConnectSentry() {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      os.Getenv("GIN_MODE"),
		TracesSampleRate: 0.2,
	})
	if err != nil {
		log.Println("Sentry init failed:", err)
		return
	}

	log.Println("Sentry error tracking enabled")
}

// FlushSentry blocks briefly to deliver buffered events; call on shutdown.
func FlushSentry() {
	sentry.Flush(2 * time.Second)
}

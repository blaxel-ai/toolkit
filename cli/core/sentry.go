package core

import (
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

// SentryDSN is the default Sentry DSN for the CLI
var SentryDSN = ""

// SentryConfig holds the configuration for Sentry initialization
type SentryConfig struct {
	DSN     string
	Release string
}

// InitSentry initializes the Sentry SDK with the given configuration
func InitSentry(cfg SentryConfig) error {
	SentryDSN = cfg.DSN
	if SentryDSN == "" {
		return nil
	}
	environment := os.Getenv("BL_ENV")
	if environment == "" {
		environment = "prod"
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              SentryDSN,
		Environment:      environment,
		Release:          cfg.Release,
		AttachStacktrace: true,
	})
	if err != nil {
		return err
	}
	return nil
}

// FlushSentry flushes buffered events before the program exits
func FlushSentry(timeout time.Duration) {
	if SentryDSN == "" {
		return
	}
	sentry.Flush(timeout)
}

// CaptureException captures an error and sends it to Sentry
func CaptureException(err error) {
	if err == nil {
		return
	}
	sentry.CaptureException(err)
}

// SetSentryTag sets a tag on the current scope
func SetSentryTag(key, value string) {
	if SentryDSN == "" {
		return
	}
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag(key, value)
	})
}

// RecoverWithSentry recovers from a panic and sends it to Sentry
// Usage: defer core.RecoverWithSentry()
func RecoverWithSentry() {
	if SentryDSN == "" {
		return
	}
	if r := recover(); r != nil {
		sentry.CurrentHub().Recover(r)
		sentry.Flush(2 * time.Second)
		panic(r) // Re-panic after capturing
	}
}

// ExitWithError captures the error to Sentry and exits with code 1
func ExitWithError(err error) {
	if err != nil && SentryDSN != "" {
		sentry.CaptureException(err)
		sentry.Flush(2 * time.Second)
	}
	os.Exit(1)
}

// ExitWithMessage captures a message to Sentry and exits with code 1
func ExitWithMessage(msg string) {
	if msg != "" && SentryDSN != "" {
		sentry.CaptureMessage(msg)
		sentry.Flush(2 * time.Second)
	}
	os.Exit(1)
}

// Exit captures to Sentry and exits with the given code
func Exit(code int) {
	if code != 0 && SentryDSN != "" {
		sentry.Flush(2 * time.Second)
	}
	os.Exit(code)
}

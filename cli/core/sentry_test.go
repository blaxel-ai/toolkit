package core

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSentryConfigStruct(t *testing.T) {
	cfg := SentryConfig{
		DSN:     "https://test@sentry.io/123",
		Release: "v1.0.0",
	}

	assert.Equal(t, "https://test@sentry.io/123", cfg.DSN)
	assert.Equal(t, "v1.0.0", cfg.Release)
}

func TestInitSentryWithEmptyDSN(t *testing.T) {
	cfg := SentryConfig{
		DSN:     "",
		Release: "v1.0.0",
	}

	err := InitSentry(cfg)
	assert.NoError(t, err)
	assert.Empty(t, SentryDSN)
}

func TestFlushSentryWithEmptyDSN(t *testing.T) {
	// Reset DSN
	SentryDSN = ""

	// Should not panic
	FlushSentry(time.Second)
}

func TestCaptureExceptionWithNil(t *testing.T) {
	// Should not panic with nil error
	CaptureException(nil)
}

func TestCaptureExceptionWithError(t *testing.T) {
	// Reset DSN to ensure it doesn't actually send to Sentry
	SentryDSN = ""

	err := errors.New("test error")
	// Should not panic
	CaptureException(err)
}

func TestSetSentryTagWithEmptyDSN(t *testing.T) {
	SentryDSN = ""

	// Should not panic
	SetSentryTag("key", "value")
}

func TestRecoverWithSentryEmptyDSN(t *testing.T) {
	SentryDSN = ""

	// Should not panic when DSN is empty
	RecoverWithSentry()
}

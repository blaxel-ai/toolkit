package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetPosthogTestState(t *testing.T, host string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DO_NOT_TRACK", "0")

	oldKey, oldHost := PosthogAPIKey, PosthogHost
	PosthogAPIKey, PosthogHost = "test-key", host
	telemetryMu.Lock()
	telemetryOnce = sync.Once{}
	telemetryCache = nil
	telemetryRaw = nil
	pendingCLIEvents = make(map[string]struct{})
	telemetryMu.Unlock()
	t.Cleanup(func() {
		FlushPosthog()
		PosthogAPIKey, PosthogHost = oldKey, oldHost
		telemetryMu.Lock()
		telemetryOnce = sync.Once{}
		telemetryCache = nil
		telemetryRaw = nil
		pendingCLIEvents = make(map[string]struct{})
		telemetryMu.Unlock()
	})
}

func TestTrackCLIInstalledSuccessfulPayloadAndDedupe(t *testing.T) {
	var requests atomic.Int32
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		require.Equal(t, "/capture/", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)

	TrackCLIInstalled("1.2.3")
	FlushPosthog()
	TrackCLIInstalled("1.2.3")
	FlushPosthog()

	assert.Equal(t, int32(1), requests.Load())
	assert.Equal(t, "test-key", payload["api_key"])
	assert.Equal(t, "Installed CLI", payload["event"])
	assert.NotEmpty(t, payload["distinct_id"])
	properties, ok := payload["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "1.2.3", properties["version"])
	assert.Equal(t, "1.2.3", loadTelemetryState().CLI)
}

func TestTrackCLIInstalledDeduplicatesPendingDelivery(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		close(requestStarted)
		<-releaseRequest
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)

	TrackCLIInstalled("1.2.4")
	<-requestStarted
	TrackCLIInstalled("1.2.4")
	close(releaseRequest)
	FlushPosthog()

	assert.Equal(t, int32(1), requests.Load())
}

func TestTrackCLIInstalledFailedDeliveryRetries(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)

	TrackCLIInstalled("2.0.0")
	FlushPosthog()
	assert.Empty(t, loadTelemetryState().CLI, "failed captures must not be deduplicated")

	TrackCLIInstalled("2.0.0")
	FlushPosthog()
	assert.Equal(t, int32(2), requests.Load())
	assert.Equal(t, "2.0.0", loadTelemetryState().CLI)

	data, err := os.ReadFile(getTelemetryPath())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"cli": "2.0.0"`)
}

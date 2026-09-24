package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetPosthogTestState(t *testing.T, host string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
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

// A CLI is a short-lived, interactive program. Telemetry that cannot be
// delivered must never hold the process open long enough for a user to notice,
// no matter how the network fails. A firewall that accepts the TCP connection
// and then silently drops the traffic is the worst case: nothing errors, the
// request simply never completes.
func TestFlushPosthogStaysWithinLatencyBudgetWhenEndpointHangs(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release // accept the request and never answer it
	}))
	defer func() {
		close(release)
		server.Close()
	}()
	resetPosthogTestState(t, server.URL)

	TrackCLIInstalled("3.0.0")

	start := time.Now()
	FlushPosthog()
	elapsed := time.Since(start)

	assert.Less(t, elapsed, posthogFlushBudget+time.Second,
		"a hanging PostHog endpoint must not delay CLI exit beyond the flush budget")
}

// The version marker is only persisted after a successful delivery, so a
// permanently unreachable endpoint makes every single command pay the flush
// budget. Keep that budget small enough to stay imperceptible.
func TestPosthogFlushBudgetIsImperceptible(t *testing.T) {
	// Measured round-trip to https://us.i.posthog.com/capture/ is ~250-350ms
	// including DNS and the TLS handshake. Anything at or under a second keeps
	// a stalled send from registering as a hang while leaving ample headroom.
	assert.LessOrEqual(t, posthogFlushBudget, time.Second)
}

// ~/.blaxel/telemetry.json is shared by the CLI and by both SDKs, each of which
// loads it once and keeps it in memory. A process that saves its own snapshot
// must not roll back fields another process wrote in the meantime, or the
// clobbered process re-sends its "Installed" event on every later run.
func TestSaveTelemetryStateMergesWritesFromOtherProcesses(t *testing.T) {
	resetPosthogTestState(t, "http://127.0.0.1:1")

	// This process loads the file early and holds it (a long-lived SDK run).
	state := loadTelemetryState()
	state.DistinctID = "shared-id"
	state.SDKs["python"] = "1.0.0"

	// Another process writes fields this one has never seen.
	path := getTelemetryPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(
		`{"distinct_id":"shared-id","cli":"9.9.9","sdks":{"typescript":"2.0.0"},"future_field":true}`,
	), 0o600))

	saveTelemetryState(state)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, "9.9.9", got["cli"], "another process's CLI version must survive")
	assert.Equal(t, true, got["future_field"], "unknown fields must survive")
	sdks, ok := got["sdks"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "2.0.0", sdks["typescript"], "another SDK's version must survive")
	assert.Equal(t, "1.0.0", sdks["python"], "this process's own version must be written")
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

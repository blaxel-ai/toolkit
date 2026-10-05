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
}

// Each writer owns exactly one field: the CLI owns "cli", and each SDK owns its
// own language entry. Re-asserting anything else on save would roll back a
// newer value written by whoever actually owns it, and that owner would then
// treat its version as unreported and send "Installed" all over again.
func TestSaveTelemetryStateNeverRollsBackEntriesItDoesNotOwn(t *testing.T) {
	resetPosthogTestState(t, "http://127.0.0.1:1")

	// This process starts up and caches whatever the SDKs had recorded so far.
	path := getTelemetryPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(
		`{"distinct_id":"shared-id","sdks":{"python":"1.0.0"}}`,
	), 0o600))
	state := loadTelemetryState()
	require.Equal(t, "1.0.0", state.SDKs["python"], "precondition: the stale value is cached")

	// The Python SDK upgrades and records a newer version on disk.
	require.NoError(t, os.WriteFile(path, []byte(
		`{"distinct_id":"shared-id","sdks":{"python":"2.0.0"}}`,
	), 0o600))

	// The CLI now persists its own install. It must not resurrect python 1.0.0.
	state.CLI = "3.0.0"
	saveTelemetryState(state)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, "3.0.0", got["cli"], "the CLI must record the field it owns")
	sdks, ok := got["sdks"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "2.0.0", sdks["python"],
		"the CLI owns no language entries and must not roll back the SDK's newer version")
}

// distinct_id is shared by the CLI and both SDKs. If the CLI starts before any
// id exists and an SDK persists one first, the CLI must adopt it instead of
// replacing it, or the same user becomes two PostHog identities.
func TestGetDistinctIDAdoptsIDPersistedByAnotherProcess(t *testing.T) {
	resetPosthogTestState(t, "http://127.0.0.1:1")

	path := getTelemetryPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`{"sdks":{}}`), 0o600))
	loadTelemetryState()

	require.NoError(t, os.WriteFile(path, []byte(`{"distinct_id":"sdk-generated"}`), 0o600))

	used := getDistinctID()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, "sdk-generated", got["distinct_id"])
	assert.Equal(t, "sdk-generated", used)
}

// Shell completion runs on every TAB press, and the version marker is only
// persisted after a successful delivery, so an unreported version or an
// unreachable endpoint would otherwise make each keypress pay the flush budget.
// These are the same commands already exempted from the tracking prompt for
// being latency-sensitive and side-effect free.
func TestTrackCLIInstalledSkipsCommandsThatMustStayFast(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)

	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	for _, args := range [][]string{
		{"bl", "__complete"},
		{"bl", "__completeNoDesc", "get", ""},
		{"bl", "-w", "dev", "__complete", ""},
		{"bl", "completion"},
		{"bl", "version"},
		{"bl", "--version"},
	} {
		os.Args = args
		TrackCLIInstalled("4.0.0")
		FlushPosthog()
		assert.Equal(t, int32(0), requests.Load(), "%q must not send telemetry", args)
	}

	// An ordinary command still reports the install.
	os.Args = []string{"bl", "get", "sandboxes"}
	TrackCLIInstalled("4.0.0")
	FlushPosthog()
	assert.Equal(t, int32(1), requests.Load(), "a normal command must still report")
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

// A user who installs a newer binary directly (curl, brew upgrade outside
// `bl upgrade`) already has an older version recorded. The new version must
// replace it, or every later command re-sends "Installed CLI".
func TestTrackCLIInstalledRecordsNewerVersionOverPreviouslyRecordedOne(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)

	path := getTelemetryPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`{"distinct_id":"shared-id","cli":"1.0.0"}`), 0o600))

	TrackCLIInstalled("2.0.0")
	FlushPosthog()
	TrackCLIInstalled("2.0.0")
	FlushPosthog()

	assert.Equal(t, int32(1), requests.Load(), "the newer version must be reported exactly once")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"cli": "2.0.0"`)
}

// An upgrade can be delivered before the install event of the binary that ran
// it. The older install event must not roll back the upgraded version.
func TestTrackCLIInstalledDoesNotOverwriteUpgradeDeliveredFirst(t *testing.T) {
	requestStarted := make(chan struct{}, 2)
	releaseInstall := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["event"] == "Installed CLI" {
			requestStarted <- struct{}{}
			<-releaseInstall
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)

	TrackCLIInstalled("1.0.0")
	<-requestStarted
	TrackCLIUpgraded("1.0.0", "2.0.0")
	for loadTelemetryStateCLI() != "2.0.0" {
		time.Sleep(5 * time.Millisecond)
	}
	close(releaseInstall)
	FlushPosthog()

	data, err := os.ReadFile(getTelemetryPath())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"cli": "2.0.0"`)
}

// A long-running older CLI can finish delivering its install event after a
// newer CLI in another terminal recorded its own version. The older event must
// not roll that back, or the newer CLI re-sends "Installed CLI" on its next run.
func TestTrackCLIInstalledDoesNotOverwriteVersionAnotherProcessRecorded(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseInstall := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseInstall
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)

	TrackCLIInstalled("1.0.0")
	<-requestStarted
	path := getTelemetryPath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`{"distinct_id":"shared-id","cli":"2.0.0"}`), 0o600))
	close(releaseInstall)
	FlushPosthog()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"cli":"2.0.0"`)
}

func loadTelemetryStateCLI() string {
	telemetryMu.Lock()
	defer telemetryMu.Unlock()
	return loadTelemetryState().CLI
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

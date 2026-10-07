package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageCaptureConsentOverridesSavedTrue(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)
	cases := []struct{ key, value string }{
		{"DO_NOT_TRACK", "1"}, {"DO_NOT_TRACK", "0"}, {"DO_NOT_TRACK", "false"}, {"DO_NOT_TRACK", " "},
		{"BL_INSTALL_TRACKING", "false"}, {"BL_INSTALL_TRACKING", " FALSE "}, {"BL_SKIP_TELEMETRY", "1"},
	}
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE", "TEAMCITY_VERSION"} {
		cases = append(cases, struct{ key, value string }{key, "1"})
	}
	for _, test := range cases {
		t.Run(test.key+"="+test.value, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			assert.False(t, capturePosthogEvent("Setup CLI", nil, func(bool) {}))
			TrackCLIInstalled("1.2.3")
			TrackCLIUpgraded("1.2.3", "1.2.4")
			TrackCLIFirstResource("Sandbox")
			TrackCLILogin("started", nil)
			FlushPosthog()
		})
	}
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(getTelemetryPath()), "config.yaml"), []byte("tracking: false\n"), 0600))
	assert.False(t, capturePosthogEvent("Login CLI", nil, func(bool) {}))
	require.NoError(t, os.Remove(filepath.Join(filepath.Dir(getTelemetryPath()), "config.yaml")))
	assert.False(t, capturePosthogEvent("Login CLI", nil, func(bool) {}), "unset consent retains the SDK default")
	assert.Zero(t, requests.Load())
	_, err := os.Stat(getTelemetryPath())
	assert.True(t, os.IsNotExist(err), "opt-outs must not create a usage ID")
}

func TestUsagePayloadAllowlistAndRandomLocalID(t *testing.T) {
	payloads := make(chan map[string]any, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		payloads <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)
	oldVersion := version
	version = "1.2.3"
	t.Cleanup(func() { version = oldVersion })
	input := map[string]any{
		"status": "failure", "failure_category": "authentication", "workspace": "private", "args": "private",
		"detected_agents": []string{"codex", "codex", "private"}, "components": []string{"mcp", "skills", "private"},
		"version": "1.2.3", "old_version": "1.2.2", "new_version": "1.2.3", "resource_category": "Sandbox",
		"$process_person_profile": true, "$geoip_disable": false,
	}
	for _, event := range []string{"Installed CLI", "Upgraded CLI", "Setup CLI", "Login CLI", "First Resource CLI"} {
		require.True(t, capturePosthogEvent(event, input, func(bool) {}))
		FlushPosthog()
		payload := <-payloads
		assert.Len(t, payload, 5)
		id, ok := payload["distinct_id"].(string)
		require.True(t, ok)
		assert.Regexp(t, usageID, id)
		assert.Equal(t, getDistinctID(), id)
		props := payload["properties"].(map[string]any)
		keys := []string{"$process_person_profile", "$geoip_disable", "os", "architecture", "install_method", "cli_version"}
		switch event {
		case "Installed CLI":
			keys = append(keys, "version")
		case "Upgraded CLI":
			keys = append(keys, "old_version", "new_version")
		case "Setup CLI":
			keys = append(keys, "status", "failure_category", "detected_agents", "components")
		case "Login CLI":
			keys = append(keys, "status", "failure_category")
		case "First Resource CLI":
			keys = append(keys, "resource_category")
		}
		actualKeys := []string{}
		for key := range props {
			actualKeys = append(actualKeys, key)
		}
		slices.Sort(keys)
		slices.Sort(actualKeys)
		assert.Equal(t, keys, actualKeys)
		assert.Equal(t, "1.2.3", props["cli_version"])
		assert.Equal(t, false, props["$process_person_profile"])
		assert.Equal(t, true, props["$geoip_disable"])
		assert.NotContains(t, props, "workspace")
		assert.NotContains(t, props, "args")
		if event == "Setup CLI" {
			assert.Equal(t, []any{"codex"}, props["detected_agents"])
			assert.Equal(t, []any{"mcp", "skills"}, props["components"])
		}
	}
	assert.False(t, capturePosthogEvent("private event", input, func(bool) {}))
	bad := usageProperties("Installed CLI", map[string]any{"version": "private@host"})
	assert.NotContains(t, bad, "version")
	bad = usageProperties("Login CLI", map[string]any{"failure_category": "private@host"})
	assert.Equal(t, "unknown", bad["failure_category"])
}

func TestUsageIDNeverForwardsArbitraryStoredText(t *testing.T) {
	resetPosthogTestState(t, "http://127.0.0.1:1")
	require.NoError(t, os.WriteFile(getTelemetryPath(), []byte(`{"distinct_id":"private@host","sdks":{"python":"1.0.0"},"future":true}`), 0600))
	id := getDistinctID()
	assert.Regexp(t, usageID, id)
	data, err := os.ReadFile(getTelemetryPath())
	require.NoError(t, err)
	assert.NotContains(t, string(data), "private@host")
	assert.Contains(t, string(data), `"python": "1.0.0"`)
	assert.Contains(t, string(data), `"future": true`)
}

func TestUsageFailureCategoryAndFirstResourceDeduplication(t *testing.T) {
	payloads := make(chan map[string]any, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		payloads <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)
	privateErr := MarkExpectedError(errors.New("private@host /private/path secret"), CLIErrorAuthentication)
	TrackCLILogin("failure", privateErr)
	TrackCLISetup([]string{"codex"}, []string{"login"}, "failure", privateErr)
	FlushPosthog()
	for range 2 {
		payload := <-payloads
		assert.Equal(t, "authentication", payload["properties"].(map[string]any)["failure_category"])
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "private")
	}
	TrackCLIFirstResource("Sandbox")
	TrackCLIFirstResource("Agent")
	FlushPosthog()
	first := <-payloads
	assert.Equal(t, "First Resource CLI", first["event"])
	assert.Equal(t, "Sandbox", first["properties"].(map[string]any)["resource_category"])
	TrackCLIFirstResource("Agent")
	FlushPosthog()
	assert.Empty(t, payloads)
	data, err := os.ReadFile(getTelemetryPath())
	require.NoError(t, err)
	assert.Contains(t, string(data), `"cli_first_resource": true`)
}

func TestUsageFirstResourceKeepsNewerCLIVersionFromAnotherProcess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	resetPosthogTestState(t, server.URL)
	path := getTelemetryPath()
	id := "6f1c0b52-3c4d-4e5f-8a9b-0c1d2e3f4a5b"
	require.NoError(t, os.WriteFile(path, []byte(`{"distinct_id":"`+id+`","cli":"1.0.0"}`), 0600))
	require.Equal(t, "1.0.0", loadTelemetryState().CLI, "precondition: the old version is cached")

	// Another CLI process upgrades and records its version.
	require.NoError(t, os.WriteFile(path, []byte(`{"distinct_id":"`+id+`","cli":"2.0.0"}`), 0600))

	TrackCLIFirstResource("Sandbox")
	FlushPosthog()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, true, got["cli_first_resource"])
	assert.Equal(t, "2.0.0", got["cli"], "the cached version must not roll back the newer one")
}

func TestUsageEventsEnabledNeedsKeyAndConsent(t *testing.T) {
	resetPosthogTestState(t, "http://127.0.0.1:1")
	assert.True(t, UsageEventsEnabled())
	t.Setenv("DO_NOT_TRACK", "1")
	assert.False(t, UsageEventsEnabled())
	t.Setenv("DO_NOT_TRACK", "")
	PosthogAPIKey = ""
	assert.False(t, UsageEventsEnabled())
}

func TestUsageTransportBoundsConcurrencyAndDropsOfflineAttempts(t *testing.T) {
	release := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		<-release
	}))
	defer func() { close(release); server.Close() }()
	resetPosthogTestState(t, server.URL)
	var successes atomic.Int32
	for range cap(posthogSlots) {
		require.True(t, capturePosthogEvent("Login CLI", nil, func(success bool) {
			if success {
				successes.Add(1)
			}
		}))
	}
	assert.False(t, capturePosthogEvent("Login CLI", nil, func(bool) {}), "excess events are dropped without a queue")
	started := time.Now()
	FlushPosthog()
	posthogWg.Wait()
	assert.Less(t, time.Since(started), 2*time.Second)
	assert.Zero(t, successes.Load())
	assert.LessOrEqual(t, requests.Load(), int32(cap(posthogSlots)))
	assert.Empty(t, posthogSlots)
}

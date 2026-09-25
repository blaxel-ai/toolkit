package core

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
)

// PostHog API key injected at build time via ldflags
var PosthogAPIKey = ""

// PostHog API endpoint
var PosthogHost = "https://us.i.posthog.com"

// posthogFlushBudget caps how long telemetry may delay process exit.
//
// A successful capture against us.i.posthog.com takes ~250-350ms end to end
// (DNS + TLS handshake + POST), so a one second budget comfortably covers the
// happy path. It matters because a version is only marked as reported after a
// successful delivery: when the endpoint is unreachable, every subsequent
// command re-sends and pays this budget again. Networks that silently drop
// traffic rather than refusing it — corporate firewalls, captive portals — hit
// that path on every invocation, so the bound has to stay imperceptible.
const posthogFlushBudget = 1 * time.Second

// telemetryState stores the last reported versions to deduplicate events
type telemetryState struct {
	DistinctID string            `json:"distinct_id"`
	CLI        string            `json:"cli,omitempty"`
	SDKs       map[string]string `json:"sdks,omitempty"`
}

var (
	telemetryOnce    sync.Once
	telemetryMu      sync.Mutex
	telemetryCache   *telemetryState
	telemetryRaw     map[string]interface{} // preserves unknown fields from disk
	pendingCLIEvents = make(map[string]struct{})
	posthogWg        sync.WaitGroup
)

// getTelemetryPath returns the path to the telemetry state file
func getTelemetryPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".blaxel", "telemetry.json")
}

// loadTelemetryState reads the telemetry state from disk
func loadTelemetryState() *telemetryState {
	telemetryOnce.Do(func() {
		telemetryCache = &telemetryState{
			SDKs: make(map[string]string),
		}
		telemetryRaw = make(map[string]interface{})
		path := getTelemetryPath()
		if path == "" {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		// Unmarshal into raw map to preserve unknown fields
		_ = json.Unmarshal(data, &telemetryRaw)
		_ = json.Unmarshal(data, telemetryCache)
		if telemetryCache.SDKs == nil {
			telemetryCache.SDKs = make(map[string]string)
		}
	})
	return telemetryCache
}

// saveTelemetryState writes the telemetry state to disk, preserving unknown
// fields and any values written by another process since this one loaded.
//
// The CLI and both SDKs share this file and each caches it in memory for the
// lifetime of its process, so the snapshot held here can be arbitrarily stale.
// Writing it back wholesale would roll back the other process's record and make
// it re-send its "Installed" event on every later run, so re-read first and
// merge only the fields this process actually owns.
func saveTelemetryState(state *telemetryState) {
	path := getTelemetryPath()
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0755)

	// Start from the fields seen at load time, then layer on whatever is on
	// disk right now, which is strictly fresher.
	merged := make(map[string]interface{})
	for k, v := range telemetryRaw {
		merged[k] = v
	}
	if onDiskData, err := os.ReadFile(path); err == nil {
		onDisk := make(map[string]interface{})
		if json.Unmarshal(onDiskData, &onDisk) == nil {
			for k, v := range onDisk {
				merged[k] = v
			}
		}
	}

	if state.DistinctID != "" {
		merged["distinct_id"] = state.DistinctID
	}
	if state.CLI != "" {
		merged["cli"] = state.CLI
	}
	// Per-language entries belong to the SDKs; the CLI only ever owns "cli".
	// Writing back this process's load-time copy of them would roll back a
	// newer version an SDK recorded after this process started, and that SDK
	// would then re-send its "Installed" event.
	if _, ok := merged["sdks"].(map[string]interface{}); !ok {
		merged["sdks"] = map[string]interface{}{}
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0600)
}

// getDistinctID returns a persistent anonymous UUID for PostHog events.
// The UUID is generated on first use and stored in ~/.blaxel/telemetry.json.
func getDistinctID() string {
	telemetryMu.Lock()
	defer telemetryMu.Unlock()

	state := loadTelemetryState()
	if state.DistinctID != "" {
		return state.DistinctID
	}
	state.DistinctID = generateUUID()
	saveTelemetryState(state)
	return state.DistinctID
}

// capturePosthogEvent sends an event to PostHog via HTTP POST. onComplete is
// called with true only after PostHog accepts the event with a 2xx response.
// Delivery errors remain silent so telemetry can never cause a user-facing failure.
func capturePosthogEvent(event string, properties map[string]string, onComplete func(success bool)) bool {
	if PosthogAPIKey == "" || !blaxel.IsTrackingEnabled() {
		return false
	}

	distinctID := getDistinctID()

	payload := map[string]interface{}{
		"api_key":     PosthogAPIKey,
		"event":       event,
		"distinct_id": distinctID,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"properties":  properties,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return false
	}

	posthogWg.Add(1)
	go func() {
		defer posthogWg.Done()
		success := false
		defer func() { onComplete(success) }()

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(PosthogHost+"/capture/", "application/json", bytes.NewReader(data))
		if err != nil {
			return
		}
		defer func() { _ = resp.Body.Close() }()
		success = resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	}()
	return true
}

// TrackCLIInstalled checks if this CLI version has been reported and sends
// an "Installed CLI" event if it hasn't.
func TrackCLIInstalled(cliVersion string) {
	if PosthogAPIKey == "" || !blaxel.IsTrackingEnabled() || cliVersion == "" || cliVersion == "dev" {
		return
	}
	// Skip telemetry in subprocess spawned by detectInstalledVersion()
	if os.Getenv("BL_SKIP_TELEMETRY") == "1" {
		return
	}
	// Shell completion runs on every TAB press. Since the version is only
	// marked reported after a successful delivery, tracking here would make
	// each keypress wait out the flush budget until PostHog accepts the event.
	// These are the same latency-sensitive, side-effect-free commands already
	// exempted from the tracking consent prompt.
	if isTrackingPromptCommandExempt(os.Args) {
		return
	}

	eventKey := "install:" + cliVersion
	telemetryMu.Lock()
	state := loadTelemetryState()
	if state.CLI == cliVersion {
		telemetryMu.Unlock()
		return
	}
	if _, pending := pendingCLIEvents[eventKey]; pending {
		telemetryMu.Unlock()
		return
	}
	pendingCLIEvents[eventKey] = struct{}{}
	telemetryMu.Unlock()

	started := capturePosthogEvent("Installed CLI", map[string]string{
		"version": cliVersion,
	}, func(success bool) {
		telemetryMu.Lock()
		defer telemetryMu.Unlock()
		delete(pendingCLIEvents, eventKey)
		if !success {
			return
		}
		state := loadTelemetryState()
		// An upgrade may complete before this older install event. Do not
		// overwrite the newer version recorded by that successful upgrade.
		if state.CLI == "" || state.CLI == cliVersion {
			state.CLI = cliVersion
			saveTelemetryState(state)
		}
	})
	if !started {
		telemetryMu.Lock()
		delete(pendingCLIEvents, eventKey)
		telemetryMu.Unlock()
	}
}

// TrackCLIUpgraded sends an "Upgraded CLI" event with old and new versions.
func TrackCLIUpgraded(oldVersion string, newVersion string) {
	if PosthogAPIKey == "" || !blaxel.IsTrackingEnabled() {
		return
	}
	if oldVersion == "" || newVersion == "" || oldVersion == newVersion {
		return
	}

	eventKey := "upgrade:" + oldVersion + ":" + newVersion
	telemetryMu.Lock()
	if _, pending := pendingCLIEvents[eventKey]; pending {
		telemetryMu.Unlock()
		return
	}
	pendingCLIEvents[eventKey] = struct{}{}
	telemetryMu.Unlock()

	started := capturePosthogEvent("Upgraded CLI", map[string]string{
		"old_version": oldVersion,
		"new_version": newVersion,
	}, func(success bool) {
		// Prevent the next process from reporting the successfully delivered
		// upgraded version as a fresh installation.
		telemetryMu.Lock()
		defer telemetryMu.Unlock()
		delete(pendingCLIEvents, eventKey)
		if !success {
			return
		}
		state := loadTelemetryState()
		state.CLI = newVersion
		saveTelemetryState(state)
	})
	if !started {
		telemetryMu.Lock()
		delete(pendingCLIEvents, eventKey)
		telemetryMu.Unlock()
	}
}

// generateUUID creates a random UUID v4 string without external dependencies.
func generateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "unknown"
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// FlushPosthog waits for all in-flight PostHog requests to complete, giving up
// after posthogFlushBudget so telemetry can never make the CLI feel hung.
// Abandoned requests are simply not marked as delivered, so they are retried by
// a later invocation.
func FlushPosthog() {
	if PosthogAPIKey == "" {
		return
	}
	done := make(chan struct{})
	go func() {
		posthogWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(posthogFlushBudget):
	}
}

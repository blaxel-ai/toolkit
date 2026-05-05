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
)

// PostHog API key injected at build time via ldflags
var PosthogAPIKey = ""

// PostHog API endpoint
var PosthogHost = "https://us.i.posthog.com"

// telemetryState stores the last reported versions to deduplicate events
type telemetryState struct {
	DistinctID string            `json:"distinct_id"`
	CLI        string            `json:"cli,omitempty"`
	SDKs       map[string]string `json:"sdks,omitempty"`
}

var (
	telemetryOnce  sync.Once
	telemetryCache *telemetryState
	telemetryRaw   map[string]interface{} // preserves unknown fields from disk
	posthogWg      sync.WaitGroup
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

// saveTelemetryState writes the telemetry state to disk, preserving unknown fields
func saveTelemetryState(state *telemetryState) {
	path := getTelemetryPath()
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0755)

	// Merge known fields into raw map to preserve unknown fields from disk
	merged := make(map[string]interface{})
	for k, v := range telemetryRaw {
		merged[k] = v
	}
	merged["distinct_id"] = state.DistinctID
	if state.CLI != "" {
		merged["cli"] = state.CLI
	}
	merged["sdks"] = state.SDKs

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0600)
}

// getDistinctID returns a persistent anonymous UUID for PostHog events.
// The UUID is generated on first use and stored in ~/.blaxel/telemetry.json.
func getDistinctID() string {
	state := loadTelemetryState()
	if state.DistinctID != "" {
		return state.DistinctID
	}
	state.DistinctID = generateUUID()
	saveTelemetryState(state)
	return state.DistinctID
}

// capturePosthogEvent sends an event to PostHog via HTTP POST.
// This is fire-and-forget: errors are silently ignored.
func capturePosthogEvent(event string, properties map[string]string) {
	if PosthogAPIKey == "" {
		return
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
		return
	}

	posthogWg.Add(1)
	go func() {
		defer posthogWg.Done()
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(PosthogHost+"/capture/", "application/json", bytes.NewReader(data))
		if err != nil {
			return
		}
		defer resp.Body.Close()
	}()
}

// TrackCLIInstalled checks if this CLI version has been reported and sends
// an "Installed CLI" event if it hasn't.
func TrackCLIInstalled(cliVersion string) {
	if PosthogAPIKey == "" || cliVersion == "" || cliVersion == "dev" {
		return
	}
	// Skip telemetry in subprocess spawned by detectInstalledVersion()
	if os.Getenv("BL_SKIP_TELEMETRY") == "1" {
		return
	}

	state := loadTelemetryState()
	if state.CLI == cliVersion {
		return
	}

	capturePosthogEvent("Installed CLI", map[string]string{
		"version": cliVersion,
	})

	state.CLI = cliVersion
	saveTelemetryState(state)
}

// TrackCLIUpgraded sends an "Upgraded CLI" event with old and new versions.
func TrackCLIUpgraded(oldVersion string, newVersion string) {
	if PosthogAPIKey == "" {
		return
	}
	if oldVersion == "" || newVersion == "" || oldVersion == newVersion {
		return
	}

	capturePosthogEvent("Upgraded CLI", map[string]string{
		"old_version": oldVersion,
		"new_version": newVersion,
	})

	// Update the stored CLI version so "Installed CLI" won't re-fire
	state := loadTelemetryState()
	state.CLI = newVersion
	saveTelemetryState(state)
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

// FlushPosthog waits for all in-flight PostHog requests to complete,
// with a maximum timeout of 5 seconds to avoid blocking indefinitely.
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
	case <-time.After(5 * time.Second):
	}
}

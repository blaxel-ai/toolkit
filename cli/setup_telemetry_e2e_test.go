package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePosthog records the event names captured by a bl binary.
type fakePosthog struct {
	mu     sync.Mutex
	events []string
}

func (f *fakePosthog) installedCLI() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, event := range f.events {
		if event == "Installed CLI" {
			count++
		}
	}
	return count
}

func startFakePosthog(t *testing.T) (*fakePosthog, string) {
	t.Helper()
	fake := &fakePosthog{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Event string `json:"event"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		fake.mu.Lock()
		fake.events = append(fake.events, body.Event)
		fake.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return fake, server.URL
}

// buildTelemetryBinary builds bl with a PostHog key and a release version,
// pointed at the given capture host, as a release build would be.
func buildTelemetryBinary(t *testing.T, posthogHost string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "bl")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	ldflags := "-X main.version=9.9.9 -X main.posthogKey=phc_test" +
		" -X github.com/blaxel-ai/toolkit/cli/core.PosthogHost=" + posthogHost
	build := exec.Command("go", "build", "-ldflags", ldflags, "-o", binary, ".")
	build.Dir = ".."
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	return binary
}

// runSetupOnFreshMachine runs bl setup without a terminal in an empty home,
// with only the given extra environment.
func runSetupOnFreshMachine(t *testing.T, binary string, extraEnv ...string) string {
	t.Helper()
	home := t.TempDir()
	cmd := exec.Command(binary, "setup", "--skip-skills", "--skip-mcp", "--skip-login")
	cmd.Env = append([]string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"USERPROFILE=" + home,
		"SYSTEMROOT=" + os.Getenv("SYSTEMROOT"),
	}, extraEnv...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	return home
}

// On a fresh machine bl setup is what turns tracking on, so it must report the
// install itself rather than leave it to whatever command the user runs next.
func TestSetupOnFreshMachineReportsInstall(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the bl binary")
	}
	posthog, host := startFakePosthog(t)
	binary := buildTelemetryBinary(t, host)

	home := runSetupOnFreshMachine(t, binary)

	assert.Equal(t, 1, posthog.installedCLI(), "bl setup must report the install in the same run")
	state, err := os.ReadFile(filepath.Join(home, ".blaxel", "telemetry.json"))
	require.NoError(t, err)
	assert.Contains(t, string(state), `"cli": "9.9.9"`)
}

func TestSetupWithTrackingTurnedOffDoesNotReportInstall(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the bl binary")
	}
	posthog, host := startFakePosthog(t)
	binary := buildTelemetryBinary(t, host)

	runSetupOnFreshMachine(t, binary, "BL_INSTALL_TRACKING=false")

	assert.Equal(t, 0, posthog.installedCLI())
}

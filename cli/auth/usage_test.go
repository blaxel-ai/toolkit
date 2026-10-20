package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageBrowserLoginFailureHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("BL_INSTALL_TRACKING", "")
	t.Setenv("BL_SKIP_TELEMETRY", "")
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE", "TEAMCITY_VERSION"} {
		t.Setenv(key, "")
	}
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".blaxel"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".blaxel", "config.yaml"), []byte("tracking: true\n"), 0600))
	payloads := make(chan map[string]any, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login/device" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"private@host"}`))
			return
		}
		require.Equal(t, "/capture/", r.URL.Path)
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		payloads <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	oldBase := blaxel.GetBaseURL()
	blaxel.SetBaseURL(server.URL)
	oldKey, oldHost := core.PosthogAPIKey, core.PosthogHost
	core.PosthogAPIKey, core.PosthogHost = "test-key", server.URL
	t.Cleanup(func() {
		core.FlushPosthog()
		core.PosthogAPIKey, core.PosthogHost = oldKey, oldHost
		blaxel.SetBaseURL(oldBase)
	})
	require.Error(t, LoginWithDevice("private-workspace"))
	core.FlushPosthog()
	assert.Len(t, payloads, 2)
	statuses := map[string]bool{}
	for range 2 {
		payload := <-payloads
		assert.Equal(t, "Login CLI", payload["event"])
		props := payload["properties"].(map[string]any)
		statuses[props["status"].(string)] = true
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "private")
	}
	assert.Equal(t, map[string]bool{"started": true, "failure": true}, statuses)
}

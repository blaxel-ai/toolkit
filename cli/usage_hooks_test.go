package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageSetupAndRegisteredCreateHooks(t *testing.T) {
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
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".codex"), 0755))
	payloads := make(chan map[string]any, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		payloads <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	oldKey, oldHost := core.PosthogAPIKey, core.PosthogHost
	core.PosthogAPIKey, core.PosthogHost = "test-key", server.URL
	t.Cleanup(func() { core.FlushPosthog(); core.PosthogAPIKey, core.PosthogHost = oldKey, oldHost })
	options := testSetupOptions(t, home, map[string]string{}, &setupRecorder{})
	options.skipSkills, options.skipMCP = true, true
	options.interactive, options.yes = true, true
	privateErr := core.MarkExpectedError(errors.New("private@host /private/path secret"), core.CLIErrorAuthentication)
	options.login = func(context.Context, *ui.Control, string) (string, error) { return "", privateErr }
	require.Error(t, runSetup(context.Background(), options))
	core.FlushPosthog()
	assert.Len(t, payloads, 3)
	seen := map[string]map[string]any{}
	for range 3 {
		payload := <-payloads
		props := payload["properties"].(map[string]any)
		seen[payload["event"].(string)+":"+props["status"].(string)] = props
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "private")
	}
	assert.Contains(t, seen, "Login CLI:started")
	assert.Equal(t, "authentication", seen["Login CLI:failure"]["failure_category"])
	assert.Equal(t, []any{"codex"}, seen["Setup CLI:failure"]["detected_agents"])
	assert.Equal(t, []any{"login", "tracking"}, seen["Setup CLI:failure"]["components"])
	options.login = func(context.Context, *ui.Control, string) (string, error) { return "private-workspace", nil }
	require.NoError(t, runSetup(context.Background(), options))
	core.FlushPosthog()
	assert.Len(t, payloads, 3)
	for range 3 {
		<-payloads
	}
	resource := &core.Resource{Kind: "Sandbox"}
	type params struct{}
	resource.Post = func(context.Context, params, ...option.RequestOption) (any, error) { return nil, privateErr }
	_, err := handleResourceOperation(resource, "private-name", nil, "post", "", nil)
	require.Error(t, err)
	resource.Put = func(context.Context, string, params, ...option.RequestOption) (any, error) {
		return map[string]any{}, nil
	}
	_, err = handleResourceOperation(resource, "private-name", nil, "put", "", nil)
	require.NoError(t, err)
	core.FlushPosthog()
	assert.Empty(t, payloads, "failed creates and successful updates cannot count as first resource")
	resource.Post = func(context.Context, params, ...option.RequestOption) (any, error) { return map[string]any{}, nil }
	_, err = handleResourceOperation(resource, "private-name", nil, "post", "", nil)
	require.NoError(t, err)
	core.FlushPosthog()
	payload := <-payloads
	assert.Equal(t, "First Resource CLI", payload["event"])
	assert.Equal(t, "Sandbox", payload["properties"].(map[string]any)["resource_category"])
}
